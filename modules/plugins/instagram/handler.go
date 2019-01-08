package instagram

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	humanize "github.com/dustin/go-humanize"
	"github.com/globalsign/mgo/bson"
)

type Handler struct{}

var (
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

		go func() {
			defer helpers.Recover()
			m.checkInstagramPublicFeedLoop()
		}()
		cache.GetLogger().WithField("module", "instagram").Info("Started Instagram GraphQl Feed loop")
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
			RetryUserInfo:
				instagramUser, _, err := m.getInformationAndPosts(instagramUsername, proxy)
				if err != nil || instagramUser.IsPrivate {
					if err != nil && m.retryOnError(err) {
						proxy, err = helpers.GetRandomProxy()
						helpers.Relax(err)
						goto RetryUserInfo
					}
					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-not-found"))
					return
				}
				// Create DB Entries
				dbPosts := make([]models.InstagramPostEntry, 0)
				/*
					// gather story if logged in
					if instagramClient != nil && instagramClient.IsLoggedIn {
						accoundIdInt, err := strconv.Atoi(instagramUser.ID)
						helpers.Relax(err)
						story, err := instagramClient.GetUserStories(int64(accoundIdInt))
						if err != nil {
							if err != nil && strings.Contains(err.Error(), "Please wait a few minutes before you try again.") {
								helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.instagram.ratelimited"))
								return
							}
						}
						helpers.Relax(err)
						for _, reelMedia := range story.Reel.Items {
							postEntry := models.InstagramPostEntry{
								ID:            reelMedia.ID,
								Type:          models.InstagramPostTypeReel,
								CreatedAtTime: time.Unix(int64(reelMedia.DeviceTimestamp), 0),
							}
							dbPosts = append(dbPosts, postEntry)
						}
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
						LastPostCheck:         time.Now(),
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
							Type:  models.EventlogTargetTypeChannel,
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
						if helpers.IsMdbNotFound(err) {
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
								Type:  models.EventlogTargetTypeChannel,
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
					if helpers.IsMdbNotFound(err) {
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
							Type:  models.EventlogTargetTypeChannel,
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
		default:
			session.ChannelTyping(msg.ChannelID)
			instagramUsername := strings.Replace(args[0], "@", "", 1)

			proxy, err := helpers.GetRandomProxy()
			helpers.Relax(err)

			instagramUser, _, err := m.getInformationAndPosts(instagramUsername, proxy)
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
