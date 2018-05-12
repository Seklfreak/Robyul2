package plugins

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"errors"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bradfitz/slice"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/getsentry/raven-go"
	"github.com/globalsign/mgo/bson"
	redisCache "github.com/go-redis/cache"
	"github.com/vmihailenco/msgpack"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

type RandomPictures struct{}

var (
	driveService *drive.Service
)

const (
	driveSearchText       = "\"%s\" in parents and (mimeType = \"image/gif\" or mimeType = \"image/jpeg\" or mimeType = \"image/png\" or mimeType = \"application/vnd.google-apps.folder\")"
	driveFieldsText       = "nextPageToken, files(id, size, mimeType)"
	driveFieldsSingleText = "id, name, size, modifiedTime, imageMediaMetadata, webContentLink"
)

func (rp *RandomPictures) Commands() []string {
	return []string{
		"randompictures",
		"rapi",
		"rp",
		"pic",
	}
}

func (rp *RandomPictures) Init(session *discordgo.Session) {

	// Get drive service
	driveService = cache.GetGoogleDriveService()

	// initial random generator
	rand.Seed(time.Now().Unix())

	go func() {
		log := cache.GetLogger()

		defer helpers.Recover()

		for {
			var marshalled []byte
			redisClient := cache.GetRedisClient()

			var rpSources []models.RandompictureSourceEntry
			err := helpers.MDbIter(helpers.MdbCollection(models.RandompictureSourcesTable).Find(nil)).All(&rpSources)
			if len(rpSources) <= 0 {
				time.Sleep(30 * time.Second)
				continue
			}
			helpers.Relax(err)

			log.WithField("module", "randompictures").Info("gathering google drive picture cache")
			for _, sourceEntry := range rpSources {
				var key1 string
				var key2 string
				var fileHash string
				var i int
				var entry *drive.File
				for i, entry = range rp.getFileCache(sourceEntry) {
					fileHash = rp.GetFileHash(sourceEntry.ID, sourceEntry.PreviousID, entry.Id)
					key1 = fmt.Sprintf("robyul2-discord:randompictures:filescache:by-n:%s:entry:%d", helpers.MdbIdToHuman(sourceEntry.ID), i+1)
					key2 = fmt.Sprintf("robyul2-discord:randompictures:filescache:by-hash:%s", fileHash)
					marshalled, err = msgpack.Marshal(entry)
					if err != nil {
						raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
						continue
					}
					err = redisClient.Set(key1, fileHash, 7*24*time.Hour).Err()
					if err != nil {
						raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
						continue
					}
					err = redisClient.Set(key2, marshalled, 7*24*time.Hour).Err()
					if err != nil {
						raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
						continue
					}
				}
				key := fmt.Sprintf("robyul2-discord:randompictures:filescache:%s:entry:%s", helpers.MdbIdToHuman(sourceEntry.ID), "count")
				err = redisClient.Set(key, i+1, 7*24*time.Hour).Err()
				if err != nil {
					raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
					continue
				}
				rp.updateImagesCachedMetric()
			}

			time.Sleep(12 * time.Hour)
		}
	}()
	cache.GetLogger().WithField("module", "randompictures").Info("Started files cache loop (12h)")

	go func() {
		defer helpers.Recover()

		for {
			time.Sleep(time.Duration(rand.Intn(30)+60) * time.Minute)

			redisClient := cache.GetRedisClient()

			var rpSources []models.RandompictureSourceEntry
			err := helpers.MDbIter(helpers.MdbCollection(models.RandompictureSourcesTable).Find(nil)).All(&rpSources)
			helpers.Relax(err)
			if len(rpSources) <= 0 {
				time.Sleep(30 * time.Second)
				continue
			}
			helpers.Relax(err)

			var fileHash string
			var key string

			for _, sourceEntry := range rpSources {
				if len(sourceEntry.PostToChannelIDs) > 0 {
					key = fmt.Sprintf("robyul2-discord:randompictures:filescache:%s:entry:%s", helpers.MdbIdToHuman(sourceEntry.ID), "count")
					pictureCount, err := redisClient.Get(key).Int64()
					if err == nil {
						for _, postToChannelID := range sourceEntry.PostToChannelIDs {
						RetryNewPicture:
							chosenPicN := rand.Intn(int(pictureCount)) + 1
							key = fmt.Sprintf("robyul2-discord:randompictures:filescache:by-n:%s:entry:%d", helpers.MdbIdToHuman(sourceEntry.ID), chosenPicN)
							fileHash = redisClient.Get(key).Val()
							if fileHash == "" {
								continue
							}
							key = fmt.Sprintf("robyul2-discord:randompictures:filescache:by-hash:%s", fileHash)
							resultBytes, err := redisClient.Get(key).Bytes()
							if err == nil {
								var gPicture *drive.File
								msgpack.Unmarshal(resultBytes, &gPicture)
								defer helpers.Recover()
								err = rp.postItem(sourceEntry.GuildID, postToChannelID, "", gPicture, sourceEntry.ID, sourceEntry.PreviousID, strconv.Itoa(chosenPicN))
								if err != nil {
									if errG, ok := err.(*googleapi.Error); ok {
										if strings.Contains("The download quota for this file has been exceeded", errG.Error()) {
											goto RetryNewPicture
										} else {
											raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
											continue
										}
									} else {
										if errD, ok := err.(*discordgo.RESTError); !ok || (errD.Message.Code != discordgo.ErrCodeMissingPermissions && errD.Message.Code != discordgo.ErrCodeUnknownChannel) {
											raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{"GuildID": sourceEntry.GuildID})
										}
										continue
									}
								}
							}
						}
					}
				}
			}
		}
	}()
	cache.GetLogger().WithField("module", "randompictures").Info("Started post loop (1h)")

	go rp.setServerFeaturesLoop()
}

func (rp *RandomPictures) setServerFeaturesLoop() {
	defer func() {
		helpers.Recover()

		cache.GetLogger().WithField("module", "randompictures").Error("The setServerFeaturesLoop died. Please investigate! Will be restarted in 60 seconds")
		time.Sleep(60 * time.Second)
		rp.setServerFeaturesLoop()
	}()

	var sourcesBucket []models.RandompictureSourceEntry
	var sourcesOnServer []models.RandompictureSourceEntry
	var err error
	var feature models.Rest_Feature_RandomPictures
	var key string
	cacheCodec := cache.GetRedisCacheCodec()
	for {
		err = helpers.MDbIter(helpers.MdbCollection(models.RandompictureSourcesTable).Find(nil)).All(&sourcesBucket)
		if err != nil {
			raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
			time.Sleep(60 * time.Second)
			continue
		}

		for _, guild := range cache.GetSession().State.Guilds {
			sourcesOnServer = make([]models.RandompictureSourceEntry, 0)
			for _, source := range sourcesBucket {
				if source.GuildID == guild.ID {
					sourcesOnServer = append(sourcesOnServer, source)
				}
			}

			key = fmt.Sprintf(models.Redis_Key_Feature_RandomPictures, guild.ID)
			feature = models.Rest_Feature_RandomPictures{
				Count: len(sourcesOnServer),
			}

			err = cacheCodec.Set(&redisCache.Item{
				Key:        key,
				Object:     feature,
				Expiration: time.Minute * 60,
			})
			if err != nil {
				raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
			}

		}

		time.Sleep(30 * time.Minute)
	}
}

func (rp *RandomPictures) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermRandomPictures) {
		return
	}

	switch command {
	case "pic": // [p]pic [<name>]
		channel, err := helpers.GetChannel(msg.ChannelID)
		helpers.Relax(err)

		if !rp.tryPicCommand(channel.GuildID, channel.ID, msg.Author.ID) {
			session.ChannelMessageDelete(msg.ChannelID, msg.ID)
			dmChannel, err := session.UserChannelCreate(msg.Author.ID)
			helpers.Relax(err)
			_, err = helpers.SendMessage(dmChannel.ID, helpers.GetText("plugins.randompictures.pic-delay-dm"))
			helpers.RelaxMessage(err, "", "")
			return
		}

		session.ChannelTyping(msg.ChannelID)
		postedPic := false

		var rpSources []models.RandompictureSourceEntry
		err = helpers.MDbIter(helpers.MdbCollection(models.RandompictureSourcesTable).Find(bson.M{"guildid": channel.GuildID})).All(&rpSources)
		helpers.Relax(err)
		if len(rpSources) <= 0 {
			_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.randompictures.pic-no-picture"))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			return
		}
		helpers.Relax(err)

		initialMessages, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.randompictures.waiting-for-picture"))
		helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
		if len(initialMessages) <= 0 {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.errors.generic-nomessage"))
			return
		}
		initialMessage := initialMessages[0]

		postedPic, err = rp.postRandomItemFromContent(channel, msg, content, initialMessage, rpSources)
		if err != nil || postedPic == false {
			helpers.EditMessage(msg.ChannelID, initialMessage.ID, helpers.GetText("plugins.randompictures.pic-no-picture"))
		} else {
			isPostingNewPic := false
			err = session.MessageReactionAdd(msg.ChannelID, initialMessage.ID, "🎲")
			if err == nil {
				randomHandler := session.AddHandler(func(session *discordgo.Session, reaction *discordgo.MessageReactionAdd) {
					defer helpers.Recover()

					if reaction.MessageID == initialMessage.ID {
						if reaction.UserID == session.State.User.ID {
							return
						}

						if reaction.UserID == msg.Author.ID && reaction.Emoji.Name == "🎲" && isPostingNewPic == false {
							postedPic, err = rp.postRandomItemFromContent(channel, msg, content, initialMessage, rpSources)
							if err != nil || postedPic == false {
								helpers.EditMessage(msg.ChannelID, initialMessage.ID, helpers.GetText("plugins.randompictures.pic-no-picture"))
							}
							session.MessageReactionRemove(reaction.ChannelID, reaction.MessageID, reaction.Emoji.Name, reaction.UserID)
						}
					}
				})
				time.Sleep(5 * time.Minute)
				randomHandler()
				session.MessageReactionRemove(msg.ChannelID, initialMessage.ID, "🎲", session.State.User.ID)
			}
		}
		return
	default:
		args := strings.Fields(content)
		if len(args) > 0 {
			switch args[0] {
			case "new-config": // [p]randompictures new-config
				helpers.RequireRobyulMod(msg, func() {
					session.ChannelTyping(msg.ChannelID)

					if len(args) <= 1 {
						_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
						helpers.Relax(err)
						return
					}

					channel, err := helpers.GetChannel(msg.ChannelID)
					helpers.Relax(err)

					guild, err := helpers.GetGuild(channel.GuildID)
					helpers.Relax(err)

					postToChannelIDs := make([]string, 0)
					driveFolderIDs := make([]string, 0)
					aliases := make([]string, 0)
					blacklistedRoleIDs := make([]string, 0)
					data := helpers.ParseKeyValueString(
						strings.TrimSpace(strings.Replace(content, args[0], "", 1)),
					)
					if channelIDsText, ok := data["channel"]; ok {
						postToChannelIDsParsed := strings.Split(channelIDsText, ",")
						for _, parsedID := range postToChannelIDsParsed {
							channelParsed, err := helpers.GetChannelFromMention(msg, parsedID)
							if err != nil || channelParsed == nil || channelParsed.ID == "" {
								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
								helpers.Relax(err)
								return
							}
							postToChannelIDs = append(postToChannelIDs, channelParsed.ID)
						}
					}
					if folderIDsText, ok := data["folder"]; ok {
						folderIDsParsed := strings.Split(folderIDsText, ",")
						for _, parsedID := range folderIDsParsed {
							result, err := driveService.Files.List().Q(fmt.Sprintf(driveSearchText, parsedID)).Fields(googleapi.Field(driveFieldsText)).PageSize(1).Do()
							if err != nil || len(result.Files) <= 0 {
								_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
								helpers.Relax(err)
								return
							}
							driveFolderIDs = append(driveFolderIDs, parsedID)
						}
					}
					if aliasesText, ok := data["alias"]; ok {
						aliasesParsed := strings.Split(aliasesText, ",")
						for _, parsedAlias := range aliasesParsed {
							parsedAlias = strings.TrimSpace(parsedAlias)
							aliases = append(aliases, parsedAlias)
						}
					}
					if blacklistedRoleIDsText, ok := data["skiproles"]; ok {
						blacklistedRoleIDsParsed := strings.Split(blacklistedRoleIDsText, ",")
						for _, parsedRoleID := range blacklistedRoleIDsParsed {
							parsedRoleID = strings.TrimSpace(parsedRoleID)
							for _, guildRole := range guild.Roles {
								if guildRole.ID == parsedRoleID {
									blacklistedRoleIDs = append(blacklistedRoleIDs, parsedRoleID)
									break
								}
							}
						}
					}

					if len(aliases) <= 0 || len(driveFolderIDs) <= 0 {
						_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
						helpers.Relax(err)
						return
					}

					newID, err := helpers.MDbInsert(
						models.RandompictureSourcesTable,
						models.RandompictureSourceEntry{
							PostToChannelIDs:   postToChannelIDs,
							DriveFolderIDs:     driveFolderIDs,
							Aliases:            aliases,
							GuildID:            channel.GuildID,
							BlacklistedRoleIDs: blacklistedRoleIDs,
						},
					)
					helpers.Relax(err)

					_, err = helpers.EventlogLog(time.Now(), channel.GuildID, helpers.MdbIdToHuman(newID),
						models.EventlogTargetTypeRobyulRandomPictureSource, msg.Author.ID,
						models.EventlogTypeRobyulRandomPictureSourceCreate, "",
						nil,
						[]models.ElasticEventlogOption{
							{
								Key:   "randompicture_source_posttochannelids",
								Value: strings.Join(postToChannelIDs, ","),
								Type:  models.EventlogTargetTypeChannel,
							},
							{
								Key:   "randompicture_source_drivefolderids",
								Value: strings.Join(driveFolderIDs, ","),
							},
							{
								Key:   "randompicture_source_aliases",
								Value: strings.Join(aliases, ","),
							},
							{
								Key:   "randompicture_source_blacklistedroleids",
								Value: strings.Join(blacklistedRoleIDs, ","),
								Type:  models.EventlogTargetTypeRole,
							},
						}, false)
					helpers.RelaxLog(err)

					_, err = helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Created a new entry in the Database: `%s`.", helpers.MdbIdToHuman(newID)))
					helpers.Relax(err)
				})
				return
			case "list": // [p]randompictures list
				helpers.RequireMod(msg, func() {
					session.ChannelTyping(msg.ChannelID)

					channel, err := helpers.GetChannel(msg.ChannelID)
					helpers.Relax(err)

					var rpSources []models.RandompictureSourceEntry
					err = helpers.MDbIter(helpers.MdbCollection(models.RandompictureSourcesTable).Find(bson.M{"guildid": channel.GuildID})).All(&rpSources)
					helpers.Relax(err)

					if len(rpSources) <= 0 {
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.randompictures.list-no-entries-error"))
						return
					} else if err != nil {
						helpers.Relax(err)
					}

					listText := ":cloud: Sources set up:\n"
					totalSources := 0
					totalCachedImages := int64(0)
					for _, rpSource := range rpSources {
						rpSourceGuild, err := helpers.GetGuild(rpSource.GuildID)
						helpers.Relax(err)
						cacheText := "No cache yet"
						key := fmt.Sprintf("robyul2-discord:randompictures:filescache:%s:entry:%s", helpers.MdbIdToHuman(rpSource.ID), "count")
						pictureCount, err := cache.GetRedisClient().Get(key).Int64()
						if err == nil {
							cacheText = fmt.Sprintf("%s images cached", humanize.Comma(pictureCount))
							totalCachedImages += pictureCount
						}

						listText += fmt.Sprintf(":arrow_forward: `%s`: on %s (`#%s`), %d Aliases (`%s`), %d Folders, %d Channels, %d Skipped Roles, %s\n",
							helpers.MdbIdToHuman(rpSource.ID), rpSourceGuild.Name, rpSourceGuild.ID,
							len(rpSource.Aliases), strings.Join(rpSource.Aliases, ","),
							len(rpSource.DriveFolderIDs),
							len(rpSource.PostToChannelIDs), len(rpSource.BlacklistedRoleIDs),
							cacheText)
						totalSources += 1
					}

					if totalSources <= 0 {
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.randompictures.list-no-entries-error"))
						return
					}

					listText += fmt.Sprintf("Found **%d** Sources in total and **%s** Cached Images.", totalSources, humanize.Comma(int64(totalCachedImages)))

					for _, page := range helpers.Pagify(listText, "\n") {
						_, err = helpers.SendMessage(msg.ChannelID, page)
						helpers.Relax(err)
					}
				})
				return
			case "delete-config", "remove-config": // [p]randompictures delete-config <source id>
				helpers.RequireMod(msg, func() {
					if len(args) < 2 {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
						return
					}
					session.ChannelTyping(msg.ChannelID)

					channel, err := helpers.GetChannel(msg.ChannelID)
					helpers.Relax(err)

					var entryBucket models.RandompictureSourceEntry
					err = helpers.MdbOne(
						helpers.MdbCollection(models.RandompictureSourcesTable).Find(bson.M{"guildid": channel.GuildID, "_id": helpers.HumanToMdbId(args[1])}),
						&entryBucket,
					)
					if helpers.IsMdbNotFound(err) {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
						return
					}
					helpers.Relax(err)

					err = helpers.MDbDelete(models.RandompictureSourcesTable, entryBucket.ID)
					helpers.Relax(err)

					_, err = helpers.EventlogLog(time.Now(), entryBucket.GuildID, helpers.MdbIdToHuman(entryBucket.ID),
						models.EventlogTargetTypeRobyulRandomPictureSource, msg.Author.ID,
						models.EventlogTypeRobyulRandomPictureSourceRemove, "",
						nil,
						[]models.ElasticEventlogOption{
							{
								Key:   "randompicture_source_posttochannelids",
								Value: strings.Join(entryBucket.PostToChannelIDs, ","),
								Type:  models.EventlogTargetTypeChannel,
							},
							{
								Key:   "randompicture_source_drivefolderids",
								Value: strings.Join(entryBucket.DriveFolderIDs, ","),
							},
							{
								Key:   "randompicture_source_aliases",
								Value: strings.Join(entryBucket.Aliases, ","),
							},
							{
								Key:   "randompicture_source_blacklistedroleids",
								Value: strings.Join(entryBucket.BlacklistedRoleIDs, ","),
								Type:  models.EventlogTargetTypeRole,
							},
						}, false)
					helpers.RelaxLog(err)

					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.randompictures.remove-success"))
					return
				})
				return
			case "refresh": // [p]randompictures refresh <source id>
				helpers.RequireRobyulMod(msg, func() {
					session.ChannelTyping(msg.ChannelID)
					if len(args) < 2 {
						_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
						helpers.Relax(err)
						return
					}

					channel, err := helpers.GetChannel(msg.ChannelID)
					helpers.Relax(err)

					var rpSources []models.RandompictureSourceEntry
					err = helpers.MDbIter(helpers.MdbCollection(models.RandompictureSourcesTable).Find(bson.M{"guildid": channel.GuildID})).All(&rpSources)
					helpers.Relax(err)

					if len(rpSources) <= 0 {
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.randompictures.list-no-entries-error"))
						return
					} else if err != nil {
						helpers.Relax(err)
					}

					for _, rpSource := range rpSources {
						if helpers.MdbIdToHuman(rpSource.ID) == args[1] {
							var fileHash string
							var key string
							var i int
							var entry *drive.File
							var marshalled []byte
							redisClient := cache.GetRedisClient()
							helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.randompictures.refresh-started"))
							for i, entry = range rp.getFileCache(rpSource) {
								key = fmt.Sprintf("robyul2-discord:randompictures:filescache:by-n:%s:entry:%d", helpers.MdbIdToHuman(rpSource.ID), i+1)
								fileHash = rp.GetFileHash(rpSource.ID, rpSource.PreviousID, entry.Id)
								if fileHash == "" {
									continue
								}
								err = redisClient.Set(key, fileHash, time.Hour*24).Err()
								if err != nil {
									raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
									continue
								}
								key = fmt.Sprintf("robyul2-discord:randompictures:filescache:by-hash:%s", fileHash)
								marshalled, err = msgpack.Marshal(entry)
								if err != nil {
									raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
									continue
								}
								err = redisClient.Set(key, marshalled, time.Hour*24).Err()
								if err != nil {
									raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
									continue
								}
							}
							key = fmt.Sprintf("robyul2-discord:randompictures:filescache:%s:entry:%s", helpers.MdbIdToHuman(rpSource.ID), "count")
							err = redisClient.Set(key, i+1, time.Hour*24).Err()
							if err != nil {
								raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
								continue
							}
							rp.updateImagesCachedMetric()
							_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.randompictures.refresh-success"))
							helpers.Relax(err)
							return
						}
					}

					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.randompictures.refresh-not-found-error"))
					helpers.Relax(err)
					return
				})
			case "pic-delay":
				// [p]rapi pic-delay <n in minutes>
				helpers.RequireMod(msg, func() {
					if len(args) <= 1 {
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
						return
					}

					channel, err := helpers.GetChannel(msg.ChannelID)
					helpers.Relax(err)

					n, err := strconv.Atoi(args[1])
					if err != nil {
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
						return
					}

					targetGuildSettings := helpers.GuildSettingsGetCached(channel.GuildID)
					delayBefore := targetGuildSettings.RandomPicturesPicDelay
					targetGuildSettings.RandomPicturesPicDelay = n
					err = helpers.GuildSettingsSet(channel.GuildID, targetGuildSettings)
					helpers.Relax(err)

					_, err = helpers.EventlogLog(time.Now(), channel.GuildID, channel.GuildID,
						models.EventlogTargetTypeGuild, msg.Author.ID,
						models.EventlogTypeRobyulRandomPictureConfigUpdate, "",
						[]models.ElasticEventlogChange{
							{
								Key:      "randompictures_config_delay",
								OldValue: strconv.Itoa(delayBefore),
								NewValue: strconv.Itoa(targetGuildSettings.RandomPicturesPicDelay),
							},
						},
						[]models.ElasticEventlogOption{
							{
								Key:   "",
								Value: "",
							},
						}, false)
					helpers.RelaxLog(err)

					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.randompictures.pic-delay-set-success", n))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				})
			case "pic-delay-ignore-channel", "pic-delay-ignore-channels", "delay-ignore-channel", "delay-ignore-channels":
				// [p]rapi pic-delay-ignore-channel [<#channel or channel id>]
				helpers.RequireMod(msg, func() {
					channel, err := helpers.GetChannel(msg.ChannelID)
					helpers.Relax(err)

					guildSettings := helpers.GuildSettingsGetCached(channel.GuildID)

					if len(args) <= 1 {
						channelText := strings.Join(guildSettings.RandomPicturesPicDelayIgnoredChannelIDs, ">, <#")
						if channelText != "" {
							channelText = "<#" + channelText + ">"
						} else {
							channelText = "`no channels selected`"
						}

						_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.randompictures.pic-delay-ignore-channels-status", channelText))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					}
					targetChannel, err := helpers.GetChannelFromMention(msg, args[1])
					if err != nil || targetChannel == nil || targetChannel.ID == "" {
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
						return
					}

					removed := false
					newChannelList := make([]string, 0)
					for _, listChannel := range guildSettings.RandomPicturesPicDelayIgnoredChannelIDs {
						if listChannel == targetChannel.ID {
							removed = true
						} else {
							newChannelList = append(newChannelList, listChannel)
						}
					}

					if !removed {
						newChannelList = append(newChannelList, targetChannel.ID)
					}

					beforeChannelIDs := guildSettings.RandomPicturesPicDelayIgnoredChannelIDs
					guildSettings.RandomPicturesPicDelayIgnoredChannelIDs = newChannelList
					err = helpers.GuildSettingsSet(channel.GuildID, guildSettings)
					helpers.Relax(err)

					options := make([]models.ElasticEventlogOption, 0)
					message := ""
					if removed {
						message = helpers.GetText("plugins.randompictures.pic-delay-ignore-channels-removed")
						options = []models.ElasticEventlogOption{
							{
								Key:   "randompictures_config_ignoredchannelids_removed",
								Value: targetChannel.ID,
								Type:  models.EventlogTargetTypeChannel,
							},
						}
					} else {
						message = helpers.GetText("plugins.randompictures.pic-delay-ignore-channels-added")
						options = []models.ElasticEventlogOption{
							{
								Key:   "randompictures_config_ignoredchannelids_added",
								Value: targetChannel.ID,
								Type:  models.EventlogTargetTypeChannel,
							},
						}
					}

					_, err = helpers.EventlogLog(time.Now(), channel.GuildID, channel.GuildID,
						models.EventlogTargetTypeGuild, msg.Author.ID,
						models.EventlogTypeRobyulRandomPictureConfigUpdate, "",
						[]models.ElasticEventlogChange{
							{
								Key:      "randompictures_config_ignoredchannelids",
								OldValue: strings.Join(beforeChannelIDs, ","),
								NewValue: strings.Join(guildSettings.RandomPicturesPicDelayIgnoredChannelIDs, ","),
								Type:     models.EventlogTargetTypeChannel,
							},
						},
						options, false)
					helpers.RelaxLog(err)

					_, err = helpers.SendMessage(msg.ChannelID, message)
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				})
			}
		}
	}

}

func (rp *RandomPictures) postRandomItemFromContent(channel *discordgo.Channel, msg *discordgo.Message, content string, initialMessage *discordgo.Message, rpSources []models.RandompictureSourceEntry) (bool, error) {
	var matchEntry models.RandompictureSourceEntry
	if content != "" { // match <name>
		for _, sourceEntry := range rpSources {
			if sourceEntry.GuildID == channel.GuildID {
				for _, alias := range sourceEntry.Aliases {
					if strings.ToLower(alias) == strings.ToLower(content) {
						matchEntry = sourceEntry
					}
				}
			}
		}
	} else { // match roles
		var err error
		guildRoles := make([]*discordgo.Role, 0)
		if helpers.GetMemberPermissions(channel.GuildID, cache.GetSession().State.User.ID)&discordgo.PermissionManageRoles == discordgo.PermissionManageRoles ||
			helpers.GetMemberPermissions(channel.GuildID, cache.GetSession().State.User.ID)&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator {
			guildRoles, err = cache.GetSession().GuildRoles(channel.GuildID)
			if err != nil {
				if err, ok := err.(*discordgo.RESTError); ok && err.Message.Code == discordgo.ErrCodeMissingPermissions {
					guildRoles = []*discordgo.Role{}
				} else {
					return false, err
				}
			}
		}

		targetMember, err := cache.GetSession().State.Member(channel.GuildID, msg.Author.ID)
		if err != nil {
			return false, err
		}
		slice.Sort(guildRoles, func(i, j int) bool {
			return guildRoles[i].Position > guildRoles[j].Position
		})
		for _, sourceEntry := range rpSources {
			if sourceEntry.GuildID == channel.GuildID {
				for _, alias := range sourceEntry.Aliases {
					if strings.ToLower(alias) == "@everyone" {
						matchEntry = sourceEntry
					}
				}
			}
		}
	CheckRoles:
		for _, guildRole := range guildRoles {
			for _, userRole := range targetMember.Roles {
				if guildRole.ID == userRole {
				NextSource:
					for _, sourceEntry := range rpSources {
						if sourceEntry.GuildID == channel.GuildID {
							for _, skippedRoleID := range sourceEntry.BlacklistedRoleIDs {
								if skippedRoleID == guildRole.ID {
									continue NextSource
								}
							}

							for _, alias := range sourceEntry.Aliases {
								if strings.Contains(strings.ToLower(guildRole.Name), strings.ToLower(alias)) {
									matchEntry = sourceEntry
									break CheckRoles
								}
							}
						}
					}
				}
			}
		}
	}
	if matchEntry.ID != "" {
		redisClient := cache.GetRedisClient()
		key := fmt.Sprintf("robyul2-discord:randompictures:filescache:%s:entry:%s", helpers.MdbIdToHuman(matchEntry.ID), "count")
		pictureCount, err := redisClient.Get(key).Int64()
		if err == nil {
			chosenPicN := rand.Intn(int(pictureCount)) + 1
			key = fmt.Sprintf("robyul2-discord:randompictures:filescache:by-n:%s:entry:%d", helpers.MdbIdToHuman(matchEntry.ID), chosenPicN)
			fileHash := redisClient.Get(key).Val()
			if fileHash == "" {
				return false, errors.New("unable to gather data for pic")
			}
			key = fmt.Sprintf("robyul2-discord:randompictures:filescache:by-hash:%s", fileHash)
			resultBytes, err := redisClient.Get(key).Bytes()
			if err != nil {
				return false, errors.New("invalid picture data cached")
			}
			var gPicture *drive.File
			msgpack.Unmarshal(resultBytes, &gPicture)
			err = rp.postItem(channel.GuildID, msg.ChannelID, initialMessage.ID, gPicture, matchEntry.ID, matchEntry.PreviousID, strconv.Itoa(chosenPicN))
			if err == nil {
				return true, nil
			} else {
				return false, err
			}
		}
	}
	return false, errors.New("unable to match to source")
}

func (rp *RandomPictures) getFileCache(sourceEntry models.RandompictureSourceEntry) []*drive.File {
	var allFiles []*drive.File

Loop:
	for {
		foldersChecked := make([]string, 0)
		foldersToCheck := sourceEntry.DriveFolderIDs
		for {
		CheckFoldersLoop:
			for _, driveFolderID := range foldersToCheck {
				for _, checkedFolder := range foldersChecked {
					if checkedFolder == driveFolderID {
						continue CheckFoldersLoop
					}
				}
				//cache.GetLogger().WithField("module", "randompictures").Debug(fmt.Sprintf("getting google drive picture cache Folder #%s for Entry #%s", driveFolderID, sourceEntry.ID))
				result, err := driveService.Files.List().Q(fmt.Sprintf(driveSearchText, driveFolderID)).Fields(googleapi.Field(driveFieldsText)).PageSize(1000).Do()
				if err != nil {
					cache.GetLogger().WithField("module", "randompictures").Error(fmt.Sprintf("google drive error: %s, retrying in 10 seconds", err.Error()))
					time.Sleep(10 * time.Second)
					continue Loop
				}
				helpers.Relax(err)
				for _, file := range result.Files {
					if rp.isValidDriveFile(file) {
						allFiles = append(allFiles, file)
					}
					if file.MimeType == "application/vnd.google-apps.folder" {
						foldersToCheck = append(foldersToCheck, file.Id)
					}
				}

				for {
					if result.NextPageToken == "" {
						break
					}
				RetryRequest:
					result, err = driveService.Files.List().Q(fmt.Sprintf(driveSearchText, driveFolderID)).Fields(googleapi.Field(driveFieldsText)).PageSize(1000).PageToken(result.NextPageToken).Do()
					if err != nil {
						if strings.Contains(err.Error(), "Error 500: Internal Error") {
							cache.GetLogger().WithField("module", "randompictures").Warnf(
								"internal error requesting files for %s, retrying in 10 seconds",
								driveFolderID,
							)
							time.Sleep(time.Second * 10)
							goto RetryRequest
						}
						helpers.Relax(err)
					}
					for _, file := range result.Files {
						if rp.isValidDriveFile(file) {
							allFiles = append(allFiles, file)
						}
						if file.MimeType == "application/vnd.google-apps.folder" {
							foldersToCheck = append(foldersToCheck, file.Id)
						}
					}
				}
				foldersChecked = append(foldersChecked, driveFolderID)
			}
			if len(foldersChecked) == len(foldersToCheck) {
				break
			}
		}
		break
	}
	return allFiles
}

func (rp *RandomPictures) isValidDriveFile(file *drive.File) bool {
	if file.MimeType == "application/vnd.google-apps.folder" {
		return false
	}
	if file.Size > 8000000 { // bigger than 8 MB? (discords file size limit)
		return false
	}
	return true
}

func (rp *RandomPictures) updateImagesCachedMetric() {
	var totalImages int64

	var rpSources []models.RandompictureSourceEntry
	err := helpers.MDbIterWithoutLogging(helpers.MdbCollection(models.RandompictureSourcesTable).Find(nil)).All(&rpSources)
	helpers.Relax(err)
	if len(rpSources) <= 0 {
		time.Sleep(30 * time.Second)
		return
	}
	helpers.Relax(err)

	var key string
	var items int64
	redisClient := cache.GetRedisClient()
	for _, sourceEntry := range rpSources {
		key = fmt.Sprintf("robyul2-discord:randompictures:filescache:%s:entry:%s", helpers.MdbIdToHuman(sourceEntry.ID), "count")
		items, err = redisClient.Get(key).Int64()
		if err != nil {
			continue
		}
		totalImages += items
	}

	metrics.RandomPictureSourcesImagesCachedCount.Set(totalImages)
}

func (rp *RandomPictures) postItem(guildID string, channelID string, messageID string, file *drive.File, sourceID bson.ObjectId, previousSourceID, pictureID string) error {
	// fmt.Println("channelID:", channelID, "messageID:", messageID, "file:", file.Name, "sourceID:", sourceID, "pictureID:", pictureID)
	file, err := driveService.Files.Get(file.Id).Fields(googleapi.Field(driveFieldsSingleText)).Do()
	if err != nil {
		return err
	}

	camerModelText := ""
	if file.ImageMediaMetadata.CameraModel != "" {
		camerModelText = fmt.Sprintf(" 📷 `%s`", file.ImageMediaMetadata.CameraModel)
	}

	linkToPost := helpers.GetConfig().Path("imageproxy.base_url").Data().(string)

	splitFilename := strings.Split(file.Name, ".")

	linkToPost = fmt.Sprintf(linkToPost, rp.GetFileHash(sourceID, previousSourceID, file.Id), url.QueryEscape(strings.Join(splitFilename[0:len(splitFilename)-1], "-")+"."+strings.ToLower(splitFilename[len(splitFilename)-1])))
	linkToHistory := helpers.GetConfig().Path("website.randompictures_base_url").Data().(string) + guildID

	// open link to prepare cache
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	request, err := http.NewRequest("GET", linkToPost, nil)
	if err == nil {
		request.Header.Set("User-Agent", helpers.DEFAULT_UA)
		resp, err := client.Do(request)
		if err != nil {
			if errU, ok := err.(*url.Error); ok {
				if !strings.Contains(errU.Err.Error(), "Client.Timeout exceeded while awaiting headers") {
					raven.CaptureError(fmt.Errorf("%#v", errU.Err), map[string]string{})
				} else {
					cache.GetLogger().WithField("module", "randompictures").Warn(fmt.Sprintf("warming up cache for %s failed: time out", linkToPost))
				}
			} else {
				raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
			}
		}
		if resp.Body != nil {
			defer resp.Body.Close()
		}
	}
	helpers.RelaxLog(err)

	err = rp.appendLinkToServerHistory(linkToPost, sourceID, pictureID, file.Name, guildID)
	helpers.RelaxLog(err)

	var shortUrl string
	if cache.GetPolr() != nil {
		shortUrl, err = cache.GetPolr().Shorten(linkToPost, "", false)
		helpers.RelaxLog(err)
	}

	if shortUrl == "" {
		shortUrl = linkToPost
	}

	embed := &discordgo.MessageEmbed{
		URL:   shortUrl,
		Title: "🏷 " + file.Name + camerModelText,
		Author: &discordgo.MessageEmbedAuthor{
			URL:  linkToHistory,
			Name: "🖼  Gallery",
		},
		Image: &discordgo.MessageEmbedImage{
			URL: linkToPost,
		},
	}

	text := "🔗 <" + shortUrl + ">"
	if messageID == "" {
		_, err = helpers.SendComplex(channelID, &discordgo.MessageSend{
			Content: text,
			Embed:   embed,
		})
		if err != nil {
			return err
		}
	} else {
		_, err = helpers.EditComplex(&discordgo.MessageEdit{
			Content: &text,
			Embed:   embed,
			ID:      messageID,
			Channel: channelID,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (rp *RandomPictures) GetFileHash(sourceID bson.ObjectId, previousID string, fileID string) string {
	if previousID != "" {
		return helpers.GetMD5Hash(previousID + "-" + fileID)
	}
	return helpers.GetMD5Hash(helpers.MdbIdToHuman(sourceID) + "-" + fileID)
}

func (rp *RandomPictures) appendLinkToServerHistory(link string, sourceID bson.ObjectId, pictureID string, fileName string, guildID string) error {
	redis := cache.GetRedisClient()
	key := fmt.Sprintf("robyul2-discord:randompictures:history:%s", guildID)

	item := new(RandomPictures_HistoryItem)
	item.Link = link
	item.SourceID = helpers.MdbIdToHuman(sourceID)
	item.PictureID = pictureID
	item.Filename = fileName
	item.GuildID = guildID
	item.Time = time.Now()

	itemBytes, err := msgpack.Marshal(&item)
	if err != nil {
		return err
	}

	_, err = redis.LPush(key, itemBytes).Result()
	if err != nil {
		return err
	}

	_, err = redis.LTrim(key, 0, 99).Result()

	return err
}

func (rp *RandomPictures) picDelayKey(guildID string, userID string) (key string) {
	return "robyul2-discord:randompictures:last-pic-usage:" + guildID + ":" + userID
}

func (rp *RandomPictures) tryPicCommand(guildID string, channelID string, userID string) (free bool) {
	targetGuildSettings := helpers.GuildSettingsGetCached(guildID)

	if targetGuildSettings.RandomPicturesPicDelay <= 0 {
		return true
	}

	for _, ignoredChannelID := range targetGuildSettings.RandomPicturesPicDelayIgnoredChannelIDs {
		if ignoredChannelID == channelID {
			return true
		}
	}

	key := rp.picDelayKey(guildID, userID)

	timestamp, err := cache.GetRedisClient().Get(key).Result()
	if err != nil || timestamp == "" {
		err := cache.GetRedisClient().Set(key, time.Now().Format(time.RFC3339), 0).Err()
		helpers.RelaxLog(err)
		return true
	}

	timeLastUsed, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		err := cache.GetRedisClient().Set(key, time.Now().Format(time.RFC3339), 0).Err()
		helpers.RelaxLog(err)
		return true
	}

	if timeLastUsed.Before(time.Now().Add(-1 * time.Duration(targetGuildSettings.RandomPicturesPicDelay) * time.Minute)) {
		err := cache.GetRedisClient().Set(key, time.Now().Format(time.RFC3339), 0).Err()
		helpers.RelaxLog(err)
		return true
	}

	return false
}

type RandomPictures_HistoryItem struct {
	Link      string
	SourceID  string
	PictureID string
	Filename  string
	GuildID   string
	Time      time.Time
}

type ImgurResponse struct {
	Data    ImageData `json:"data"`
	Status  int       `json:"status"`
	Success bool      `json:"success"`
}

type ImageData struct {
	Account_id int    `json:"account_id"`
	Animated   bool   `json:"animated"`
	Bandwidth  int    `json:"bandwidth"`
	DateTime   int    `json:"datetime"`
	Deletehash string `json:"deletehash"`
	Favorite   bool   `json:"favorite"`
	Height     int    `json:"height"`
	Id         string `json:"id"`
	In_gallery bool   `json:"in_gallery"`
	Is_ad      bool   `json:"is_ad"`
	Link       string `json:"link"`
	Name       string `json:"name"`
	Size       int    `json:"size"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	Views      int    `json:"views"`
	Width      int    `json:"width"`
	Error      string `json:"error"`
	Request    string `json:"request"`
	Method     string `json:"method"`
}
