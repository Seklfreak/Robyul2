package instagram

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"time"

	"net/url"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/ahmdrz/goinsta"
	goinstaResponse "github.com/ahmdrz/goinsta/response"
	goinstaStore "github.com/ahmdrz/goinsta/store"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/globalsign/mgo/bson"
)

type Handler struct{}

var (
	instagramClient      *goinsta.Instagram
	instagramPicUrlRegex *regexp.Regexp
)

const (
	hexColor                 = "#fcaf45"
	instagramFriendlyUser    = "https://www.instagram.com/%s/"
	instagramFriendlyPost    = "https://www.instagram.com/p/%s/"
	instagramPicUrlRegexText = `(http(s)?\:\/\/[^\/]+\/[^\/]+\/)([a-z0-9\.]+\/)?([a-z0-9\.]+\/)?([a-z0-9]+x[a-z0-9]+\/)?([a-z0-9\.]+\/)?(([a-z0-9]+\/)?.+\.jpg)`
	instagramSessionKey      = "robyul2-discord:instagram:session"
)

func (m *Handler) Commands() []string {
	return []string{
		"instagram",
	}
}

func (m *Handler) Init(session *discordgo.Session) {
	go func() {
		defer helpers.Recover()

		var err error

		go m.checkInstagramGraphQlFeedLoop()
		cache.GetLogger().WithField("module", "instagram").Info("Started Instagram GraphQl Feed loop")

		//cache.GetRedisClient().Del(instagramSessionKey).Result()
		storedInstagram, err := cache.GetRedisClient().Get(instagramSessionKey).Bytes()
		if err == nil {
			instagramClient, err = goinstaStore.Import(storedInstagram, make([]byte, 32))
			helpers.Relax(err)
			cache.GetLogger().WithField("module", "instagram").Infof(
				"restoring instagram session from redis",
			)
		} else {
			instagramClient = goinsta.New(
				helpers.GetConfig().Path("instagram.username").Data().(string),
				helpers.GetConfig().Path("instagram.password").Data().(string),
			)
			cache.GetLogger().WithField("module", "instagram").Infof(
				"starting new instagram session",
			)
		}
		// set proxy
		instagramClient.Proxy = helpers.GetConfig().Path("instagram.proxy").Data().(string)
		err = instagramClient.Login()
		helpers.Relax(err)
		cache.GetLogger().WithField("module", "instagram").Infof(
			"logged in to instagram as @%s",
			instagramClient.Informations.Username,
		)
		storedInstagram, err = goinstaStore.Export(instagramClient, make([]byte, 32))
		helpers.Relax(err)
		err = cache.GetRedisClient().Set(instagramSessionKey, storedInstagram, 0).Err()
		helpers.Relax(err)
		cache.GetLogger().WithField("module", "instagram").Infof(
			"stored instagram session in redis",
		)

		instagramPicUrlRegex, err = regexp.Compile(instagramPicUrlRegexText)
		helpers.Relax(err)

		go m.checkInstagramFeedsAndStoryLoop()
		cache.GetLogger().WithField("module", "instagram").Info("Started Instagram Feeds and Story loop")
	}()
}

func (m *Handler) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermInstagram) {
		return
	}

	args := strings.Fields(content)
	if len(args) >= 1 {
		switch args[0] {
		case "add": // [p]instagram add <instagram account name (with or without @)> <discord channel>
			helpers.RequireMod(msg, func() {
				session.ChannelTyping(msg.ChannelID)
				// get target channel
				var err error
				var targetChannel *discordgo.Channel
				var targetGuild *discordgo.Guild
				if len(args) >= 3 {
					targetChannel, err = helpers.GetChannelFromMention(msg, args[2])
					if err != nil {
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
						return
					}
				} else {
					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
					return
				}
				targetGuild, err = helpers.GetGuild(targetChannel.GuildID)
				helpers.Relax(err)
				// proxy
				proxy, err := helpers.GetRandomProxy()
				helpers.Relax(err)
				// get instagram account
				instagramUsername := strings.Replace(args[1], "@", "", 1)
				instagramUser, err := m.getUserInformation(instagramUsername, proxy)
				if err != nil || instagramUser.IsPrivate {
					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-not-found"))
					return
				}
				instagramPosts, err := m.getPosts(instagramUser.ID, proxy)
				if err != nil {
					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-not-found"))
					return
				}
				// Create DB Entries
				dbPosts := make([]models.InstagramPostEntry, 0)
				for _, post := range instagramPosts {
					postEntry := models.InstagramPostEntry{
						ID:            post.ID,
						Type:          models.InstagramPostTypePost,
						CreatedAtTime: post.CreatedAt,
					}
					dbPosts = append(dbPosts, postEntry)
				}
				/*
					for _, reelMedia := range story.Reel.Items {
						postEntry := models.InstagramPostEntry{
							ID:            reelMedia.ID,
							Type:          models.InstagramPostTypeReel,
							CreatedAtTime: time.Unix(int64(reelMedia.DeviceTimestamp), 0),
						}
						dbPosts = append(dbPosts, postEntry)

					}
				*/
				// create new entry in db
				var specialText string
				postMode := models.InstagramSendPostTypeRobyulEmbed
				if strings.HasSuffix(content, " direct link mode") ||
					strings.HasSuffix(content, " link mode") ||
					strings.HasSuffix(content, " links") {
					postMode = models.InstagramSendPostTypeDirectLinks
					specialText += " using direct links"
				}

				newID, err := helpers.MDbInsert(
					models.InstagramTable,
					models.InstagramEntry{
						GuildID:               targetChannel.GuildID,
						ChannelID:             targetChannel.ID,
						Username:              instagramUser.Username,
						InstagramUserIDString: instagramUser.ID,
						PostedPosts:           dbPosts,
						IsLive:                false,
						SendPostType:          postMode,
					},
				)
				helpers.Relax(err)

				_, err = helpers.EventlogLog(time.Now(), targetChannel.GuildID, helpers.MdbIdToHuman(newID),
					models.EventlogTargetTypeRobyulInstagramFeed, msg.Author.ID,
					models.EventlogTypeRobyulInstagramFeedAdd, "",
					nil,
					[]models.ElasticEventlogOption{
						{
							Key:   "instagram_channelid",
							Value: targetChannel.ID,
						},
						{
							Key:   "instagram_sendposttype",
							Value: strconv.Itoa(int(postMode)),
						},
						{
							Key:   "instagram_instagramuserid",
							Value: instagramUser.ID,
						},
						{
							Key:   "instagram_instagramusername",
							Value: instagramUser.Username,
						},
					}, false)
				helpers.RelaxLog(err)

				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-added-success", instagramUser.Username, targetChannel.ID, specialText))
				cache.GetLogger().WithField("module", "instagram").Info(fmt.Sprintf("Added Instagram Account @%s to Channel %s (#%s) on Guild %s (#%s)", instagramUser.Username, targetChannel.Name, targetChannel.ID, targetGuild.Name, targetGuild.ID))
			})
		case "delete", "del", "remove": // [p]instagram delete <id>
			helpers.RequireMod(msg, func() {
				if len(args) >= 2 {
					session.ChannelTyping(msg.ChannelID)

					channel, err := helpers.GetChannel(msg.ChannelID)
					helpers.Relax(err)

					entryId := args[1]
					var entryBucket models.InstagramEntry
					err = helpers.MdbOne(
						helpers.MdbCollection(models.InstagramTable).Find(bson.M{"guildid": channel.GuildID, "_id": helpers.HumanToMdbId(entryId)}),
						&entryBucket,
					)

					if err != nil {
						if strings.Contains(err.Error(), "not found") {
							helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.instagram.account-delete-not-found-error"))
							return
						}
						helpers.Relax(err)
					}

					err = helpers.MDbDelete(models.InstagramTable, entryBucket.ID)

					_, err = helpers.EventlogLog(time.Now(), channel.GuildID, helpers.MdbIdToHuman(entryBucket.ID),
						models.EventlogTargetTypeRobyulInstagramFeed, msg.Author.ID,
						models.EventlogTypeRobyulInstagramFeedRemove, "",
						nil,
						[]models.ElasticEventlogOption{
							{
								Key:   "instagram_channelid",
								Value: entryBucket.ChannelID,
							},
							{
								Key:   "instagram_sendposttype",
								Value: strconv.Itoa(int(entryBucket.SendPostType)),
							},
							{
								Key:   "instagram_instagramuserid",
								Value: entryBucket.InstagramUserIDString,
							},
							{
								Key:   "instagram_instagramusername",
								Value: entryBucket.Username,
							},
						}, false)
					helpers.RelaxLog(err)

					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-delete-success", entryBucket.Username))
					cache.GetLogger().WithField("module", "instagram").Info(fmt.Sprintf("Deleted Instagram Account @%s", entryBucket.Username))

				} else {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					return
				}
			})
		case "list": // [p]instagram list
			currentChannel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)
			var entryBucket []models.InstagramEntry
			err = helpers.MDbIter(helpers.MdbCollection(models.InstagramTable).Find(bson.M{"guildid": currentChannel.GuildID})).All(&entryBucket)
			helpers.Relax(err)

			if entryBucket == nil || len(entryBucket) <= 0 {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-list-no-accounts-error"))
				return
			} else if err != nil {
				helpers.Relax(err)
			}

			resultMessage := ""
			for _, entry := range entryBucket {
				var directLinkModeText string
				if entry.SendPostType == models.InstagramSendPostTypeDirectLinks {
					directLinkModeText = " (direct link mode)"
				}

				resultMessage += fmt.Sprintf("`%s`: Instagram Account `@%s` posting to <#%s>%s\n",
					helpers.MdbIdToHuman(entry.ID), entry.Username, entry.ChannelID, directLinkModeText)
			}
			resultMessage += fmt.Sprintf("Found **%d** Instagram Accounts in total.", len(entryBucket))
			for _, resultPage := range helpers.Pagify(resultMessage, "\n") {
				_, err = helpers.SendMessage(msg.ChannelID, resultPage)
				helpers.Relax(err)
			}
		case "toggle-direct-link", "toggle-direct-links": // [p]instagram toggle-direct-links <id>
			helpers.RequireMod(msg, func() {
				session.ChannelTyping(msg.ChannelID)

				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)

				if len(args) < 2 {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					return
				}

				entryId := args[1]
				var entryBucket models.InstagramEntry
				err = helpers.MdbOne(
					helpers.MdbCollection(models.InstagramTable).Find(bson.M{"guildid": channel.GuildID, "_id": helpers.HumanToMdbId(entryId)}),
					&entryBucket,
				)

				if err != nil {
					if strings.Contains(err.Error(), "not found") {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
						return
					}
					helpers.Relax(err)
				}

				beforeValue := entryBucket.SendPostType

				var messageText string
				if entryBucket.SendPostType == models.InstagramSendPostTypeRobyulEmbed {
					entryBucket.SendPostType = models.InstagramSendPostTypeDirectLinks
					messageText = helpers.GetText("plugins.instagram.post-direct-links-enabled")
				} else {
					entryBucket.SendPostType = models.InstagramSendPostTypeRobyulEmbed
					messageText = helpers.GetText("plugins.instagram.post-direct-links-disabled")
				}

				_, err = helpers.EventlogLog(time.Now(), channel.GuildID, helpers.MdbIdToHuman(entryBucket.ID),
					models.EventlogTargetTypeRobyulInstagramFeed, msg.Author.ID,
					models.EventlogTypeRobyulInstagramFeedUpdate, "",
					[]models.ElasticEventlogChange{
						{
							Key:      "instagram_sendposttype",
							OldValue: strconv.Itoa(int(beforeValue)),
							NewValue: strconv.Itoa(int(entryBucket.SendPostType)),
						},
					},
					[]models.ElasticEventlogOption{
						{
							Key:   "instagram_channelid",
							Value: entryBucket.ChannelID,
						},
						{
							Key:   "instagram_sendposttype",
							Value: strconv.Itoa(int(entryBucket.SendPostType)),
						},
						{
							Key:   "instagram_instagramuserid",
							Value: entryBucket.InstagramUserIDString,
						},
						{
							Key:   "instagram_instagramusername",
							Value: entryBucket.Username,
						},
					}, false)
				helpers.RelaxLog(err)

				err = helpers.MDbUpdate(models.InstagramTable, entryBucket.ID, entryBucket)
				helpers.Relax(err)

				helpers.SendMessage(msg.ChannelID, messageText)
				return
			})
		case "login":
			helpers.RequireRobyulMod(msg, func() {
				err := instagramClient.Login()
				helpers.Relax(err)

				helpers.SendMessage(msg.ChannelID, "plugins.instagram.logged-in")
				return
			})
			return
		default:
			session.ChannelTyping(msg.ChannelID)
			instagramUsername := strings.Replace(args[0], "@", "", 1)

			proxy, err := helpers.GetRandomProxy()
			helpers.Relax(err)

			instagramUser, err := m.getUserInformation(instagramUsername, proxy)
			if err != nil {
				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-not-found"))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			}

			instagramNameModifier := ""
			if instagramUser.IsVerified {
				instagramNameModifier += " ☑"
			}
			if instagramUser.IsPrivate {
				instagramNameModifier += " 🔒"
			}
			/*
				if instagramUser.User.IsBusiness {
					instagramNameModifier += " 🏢"
				}
				if instagramUser.User.IsFavorite {
					instagramNameModifier += " ⭐"
				}
			*/

			accountEmbed := &discordgo.MessageEmbed{
				Title:     helpers.GetTextF("plugins.instagram.account-embed-title", instagramUser.FullName, instagramUser.Username, instagramNameModifier),
				URL:       fmt.Sprintf(instagramFriendlyUser, instagramUser.Username),
				Thumbnail: &discordgo.MessageEmbedThumbnail{URL: instagramUser.ProfilePicUrl},
				Footer: &discordgo.MessageEmbedFooter{
					Text: helpers.GetTextF("plugins.instagram.account-embed-footer", instagramUser.ID) + " | " +
						helpers.GetText("plugins.instagram.embed-footer"),
					IconURL: helpers.GetText("plugins.instagram.embed-footer-imageurl"),
				},
				Description: instagramUser.Biography,
				Fields: []*discordgo.MessageEmbedField{
					{Name: "Followers", Value: humanize.Comma(int64(instagramUser.Followers)), Inline: true},
					{Name: "Following", Value: humanize.Comma(int64(instagramUser.Followings)), Inline: true},
					{Name: "Posts", Value: humanize.Comma(int64(instagramUser.Posts)), Inline: true}},
				Color: helpers.GetDiscordColorFromHex(hexColor),
			}
			if instagramUser.Link != "" {
				accountEmbed.Fields = append(accountEmbed.Fields, &discordgo.MessageEmbedField{
					Name:   "Website",
					Value:  instagramUser.Link,
					Inline: true,
				})
			}
			_, err = helpers.SendComplex(
				msg.ChannelID, &discordgo.MessageSend{
					Content: fmt.Sprintf("<%s>", fmt.Sprintf(instagramFriendlyUser, instagramUser.Username)),
					Embed:   accountEmbed,
				})
			helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
			return
		}
	} else {
		helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
	}
}

func (m *Handler) postLiveToChannel(channelID string, instagramUser Instagram_User) {
	instagramNameModifier := ""
	if instagramUser.IsVerified {
		instagramNameModifier += " ☑"
	}
	if instagramUser.IsPrivate {
		instagramNameModifier += " 🔒"
	}
	if instagramUser.IsBusiness {
		instagramNameModifier += " 🏢"
	}
	if instagramUser.IsFavorite {
		instagramNameModifier += " ⭐"
	}

	channelEmbed := &discordgo.MessageEmbed{
		Title:     helpers.GetTextF("plugins.instagram.live-embed-title", instagramUser.FullName, instagramUser.Username, instagramNameModifier),
		URL:       fmt.Sprintf(instagramFriendlyUser, instagramUser.Username),
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: instagramUser.ProfilePic.URL},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.instagram.embed-footer"),
			IconURL: helpers.GetText("plugins.instagram.embed-footer-imageurl"),
		},
		Image: &discordgo.MessageEmbedImage{URL: instagramUser.Broadcast.CoverFrameURL},
		Color: helpers.GetDiscordColorFromHex(hexColor),
	}

	mediaUrl := channelEmbed.URL

	_, err := helpers.SendComplex(channelID, &discordgo.MessageSend{
		Content: fmt.Sprintf("<%s>", mediaUrl),
		Embed:   channelEmbed,
	})
	if err != nil {
		cache.GetLogger().WithField("module", "instagram").Warnf("posting broadcast: #%d to channel: #%s failed: %s", instagramUser.Broadcast.ID, channelID, err.Error())
	}
}

func (m *Handler) postReelMediaToChannel(channelID string, story goinstaResponse.StoryResponse, number int, postMode models.InstagramSendPostType) {
	instagramNameModifier := ""
	if story.Reel.User.IsVerified {
		instagramNameModifier += " ☑"
	}
	if story.Reel.User.IsPrivate {
		instagramNameModifier += " 🔒"
	}
	/*
		if story.Reel.User.IsBusiness {
			instagramNameModifier += " 🏢"
		}
		if story.Reel.User.IsFavorite {
			instagramNameModifier += " ⭐"
		}
	*/

	reelMedia := story.Reel.Items[number]

	mediaModifier := "Picture"
	if reelMedia.MediaType == 2 {
		mediaModifier = "Video"
	}

	caption := ""
	if captionData, ok := reelMedia.Caption.(map[string]interface{}); ok {
		caption, _ = captionData["text"].(string)
	}

	var content string
	channelEmbed := &discordgo.MessageEmbed{
		Title:     helpers.GetTextF("plugins.instagram.reelmedia-embed-title", story.Reel.User.FullName, story.Reel.User.Username, instagramNameModifier, mediaModifier),
		URL:       fmt.Sprintf(instagramFriendlyUser, story.Reel.User.Username),
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: story.Reel.User.ProfilePicURL},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.instagram.embed-footer"),
			IconURL: helpers.GetText("plugins.instagram.embed-footer-imageurl"),
		},
		Description: caption,
		Color:       helpers.GetDiscordColorFromHex(hexColor),
	}
	if postMode == models.InstagramSendPostTypeDirectLinks {
		content += "**" + helpers.GetTextF("plugins.instagram.reelmedia-embed-title", story.Reel.User.FullName, story.Reel.User.Username, instagramNameModifier, mediaModifier) + "** _" + helpers.GetText("plugins.instagram.embed-footer") + "_\n"
		if caption != "" {
			content += caption + "\n"
		}
	}

	mediaUrl := ""
	thumbnailUrl := ""

	if len(reelMedia.ImageVersions2.Candidates) > 0 {
		channelEmbed.Image = &discordgo.MessageEmbedImage{URL: getBestCandidateURL(reelMedia.ImageVersions2.Candidates)}
		mediaUrl = getBestCandidateURL(reelMedia.ImageVersions2.Candidates)
	}
	if len(reelMedia.VideoVersions) > 0 {
		channelEmbed.Video = &discordgo.MessageEmbedVideo{
			URL: getBestStoryVideoVersionURL(story, number),
		}
		if mediaUrl != "" {
			thumbnailUrl = mediaUrl
		}
		mediaUrl = getBestStoryVideoVersionURL(story, number)
	}

	if mediaUrl != "" {
		channelEmbed.URL = mediaUrl
	} else {
		mediaUrl = channelEmbed.URL
	}

	content += stripInstagramDirectLink(mediaUrl) + "\n"
	if thumbnailUrl != "" {
		content += stripInstagramDirectLink(thumbnailUrl) + "\n"
	}

	messageSend := &discordgo.MessageSend{
		Content: content,
	}
	if postMode != models.InstagramSendPostTypeDirectLinks {
		messageSend.Content = fmt.Sprintf("<%s>", stripInstagramDirectLink(mediaUrl))
		messageSend.Embed = channelEmbed
	}

	_, err := helpers.SendComplex(channelID, messageSend)
	if err != nil {
		cache.GetLogger().WithField("module", "instagram").Warnf("posting reel media: #%s to channel: #%s failed: %s", reelMedia.ID, channelID, err.Error())
	}
}

func stripInstagramDirectLink(link string) (result string) {
	url, err := url.Parse(link)
	if err != nil {
		return link
	}
	queries := strings.Split(url.RawQuery, "&")
	var newQueryString string
	for _, query := range queries {
		if strings.HasPrefix(query, "ig_cache_key") { // strip ig_cache_key
			continue
		}
		newQueryString += query + "&"
	}
	newQueryString = strings.TrimSuffix(newQueryString, "&")
	url.RawQuery = newQueryString
	return url.String()
}

func getBestCandidateURL(imageCandidates []goinstaResponse.ImageCandidate) string {
	var lastBestCandidate goinstaResponse.ImageCandidate
	for _, candidate := range imageCandidates {
		if lastBestCandidate.URL == "" {
			lastBestCandidate = candidate
		} else {
			if candidate.Height > lastBestCandidate.Height || candidate.Width > lastBestCandidate.Width {
				lastBestCandidate = candidate
			}
		}
	}

	return lastBestCandidate.URL
}

func getBestStoryVideoVersionURL(story goinstaResponse.StoryResponse, number int) string {
	item := story.Reel.Items[number]

	var lastBestCandidateURL string
	var lastBestCandidateWidth, lastBestCandidataHeight int
	for _, version := range item.VideoVersions {
		if lastBestCandidateURL == "" {
			lastBestCandidateURL = version.URL
			lastBestCandidataHeight = version.Height
			lastBestCandidateWidth = version.Width
		} else {
			if version.Height > lastBestCandidataHeight || version.Width > lastBestCandidateWidth {
				lastBestCandidateURL = version.URL
				lastBestCandidataHeight = version.Height
				lastBestCandidateWidth = version.Width
			}
		}
	}

	return lastBestCandidateURL
}
