package notifications

import (
	"fmt"
	"strings"

	"time"

	"regexp"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bradfitz/slice"
	"github.com/bwmarrin/discordgo"
	"github.com/globalsign/mgo/bson"
)

type Handler struct{}

func (m *Handler) Commands() []string {
	return []string{
		"notifications",
		"notification",
		"noti",
		"notis",
	}
}

func (m *Handler) Init(session *discordgo.Session) {
	session.AddHandler(m.OnMessage)
	go func() {
		defer helpers.Recover()

		err := refreshNotificationSettingsCache()
		helpers.RelaxLog(err)
	}()
}

func (m *Handler) Uninit(session *discordgo.Session) {

}

func (m *Handler) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermNotifications) {
		return
	}

	args := strings.Fields(content)
	if len(args) > 0 {
		switch args[0] {
		case "add": // [p]notifications add <keyword(s)>
			if len(args) < 2 {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
				return
			}
			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)
			guild, err := helpers.GetGuild(channel.GuildID)
			helpers.Relax(err)

			keywords := strings.TrimSpace(strings.Replace(content, args[0], "", 1))
			keywordGuild := guild.ID
			if strings.HasPrefix(keywords, "global ") {
				keywords = strings.TrimSpace(strings.TrimPrefix(keywords, "global "))
				keywordGuild = "global"
			}

			var entryBucket models.NotificationsEntry
			err = helpers.MdbOne(
				helpers.MdbCollection(models.NotificationsTable).Find(
					bson.M{"userid": msg.Author.ID,
						"guildid": bson.M{"$in": []string{guild.ID, "global"}},
						"keyword": bson.M{"$regex": bson.RegEx{Pattern: "^" + regexp.QuoteMeta(keywords) + "$", Options: "i"}},
					}),
				&entryBucket,
			)

			if err == nil {
				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.notifications.keyword-add-error-duplicate", msg.Author.ID))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				session.ChannelMessageDelete(msg.ChannelID, msg.ID)
				return
			}
			if err != nil && !helpers.IsMdbNotFound(err) {
				helpers.Relax(err)
			}

			err = helpers.MDbUpsert(
				models.NotificationsTable,
				bson.M{"userid": msg.Author.ID, "guildid": keywordGuild, "keyword": keywords},
				models.NotificationsEntry{
					Keyword: keywords,
					GuildID: keywordGuild,
					UserID:  msg.Author.ID,
				},
			)
			helpers.Relax(err)

			_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.notifications.keyword-added-success", msg.Author.ID))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			cache.GetLogger().WithField("module", "notifications").Info(fmt.Sprintf("Added Notification Keyword \"%s\" to Guild %s (#%s) for User %s (#%s)", keywords, guild.Name, guild.ID, msg.Author.Username, msg.Author.ID))
			session.ChannelMessageDelete(msg.ChannelID, msg.ID) // Do not get error as it might fail because deletion permissions are not given to the user
			go func() {
				defer helpers.Recover()

				err := refreshNotificationSettingsCache()
				helpers.RelaxLog(err)
			}()
		case "delete", "del", "remove": // [p]notifications delete <keyword(s)>
			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)
			guild, err := helpers.GetGuild(channel.GuildID)
			helpers.Relax(err)
			if len(args) < 2 {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
				return
			}
			session.ChannelTyping(msg.ChannelID)

			keywords := strings.TrimSpace(strings.Replace(content, args[0], "", 1))
			if strings.HasPrefix(keywords, "global ") {
				keywords = strings.TrimSpace(strings.TrimPrefix(keywords, "global "))
			}

			var entryBucket models.NotificationsEntry
			err = helpers.MdbOne(
				helpers.MdbCollection(models.NotificationsTable).Find(
					bson.M{"userid": msg.Author.ID,
						"guildid": bson.M{"$in": []string{guild.ID, "global"}},
						"keyword": bson.M{"$regex": bson.RegEx{Pattern: "^" + regexp.QuoteMeta(keywords) + "$", Options: "i"}},
					}),
				&entryBucket,
			)
			if helpers.IsMdbNotFound(err) {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.notifications.keyword-delete-not-found-error", msg.Author.ID))
				return
			}
			helpers.Relax(err)

			err = helpers.MDbDelete(models.NotificationsTable, entryBucket.ID)
			helpers.Relax(err)

			_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.notifications.keyword-delete-success", msg.Author.ID))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			cache.GetLogger().WithField("module", "notifications").Info(fmt.Sprintf("Deleted Notification Keyword \"%s\" from Guild %s (#%s) for User %s (#%s)", entryBucket.Keyword, guild.Name, guild.ID, msg.Author.Username, msg.Author.ID))
			session.ChannelMessageDelete(msg.ChannelID, msg.ID) // Do not get error as it might fail because deletion permissions are not given to the user
			go func() {
				defer helpers.Recover()

				err := refreshNotificationSettingsCache()
				helpers.RelaxLog(err)
			}()
		case "list": // [p]notifications list
			handleList(session, msg)
			return
		case "ignore":
			handleIgnore(session, content, msg, args)
			return
		case "ignore-channel":
			if len(args) < 2 {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
				return
			}
			commandIssueChannel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)
			switch args[1] {
			case "list": // [p]notifications ignore-channel list
				var entryBucket []models.NotificationsIgnoredChannelsEntry
				err := helpers.MDbIter(helpers.MdbCollection(models.NotificationsIgnoredChannelsTable).Find(bson.M{"guildid": commandIssueChannel.GuildID})).All(&entryBucket)
				helpers.Relax(err)

				if entryBucket == nil || len(entryBucket) <= 0 {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.notifications.ignoredchannels-list-no-keywords-error"))
					return
				}
				helpers.Relax(err)

				resultMessage := fmt.Sprintf("Ignored channels on this server:\n")
				for _, entry := range entryBucket {
					resultMessage += fmt.Sprintf("<#%s>\n", entry.ChannelID)
				}
				resultMessage += fmt.Sprintf("Found **%d** Ignored Channels in total.", len(entryBucket))

				_, err = helpers.SendMessage(msg.ChannelID, resultMessage)
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			default: // [p]notifications ignore-channel <channel>
				helpers.RequireAdmin(msg, func() {
					targetChannel, err := helpers.GetChannelOrCategoryFromMention(msg, args[1])
					if err != nil {
						if strings.Contains(err.Error(), "Channel not found.") {
							helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
							return
						}
					}
					helpers.Relax(err)

					if targetChannel.GuildID != commandIssueChannel.GuildID {
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.notifications.ignore-channel-addorremove-error-server"))
						return
					}

					var entryBucket models.NotificationsIgnoredChannelsEntry
					err = helpers.MdbOne(
						helpers.MdbCollection(models.NotificationsIgnoredChannelsTable).Find(bson.M{"channelid": targetChannel.ID}),
						&entryBucket,
					)

					if helpers.IsMdbNotFound(err) {
						// Add to list
						err = helpers.MDbUpsert(
							models.NotificationsIgnoredChannelsTable,
							bson.M{"channelid": targetChannel.ID, "guildid": targetChannel.GuildID},
							models.NotificationsIgnoredChannelsEntry{
								GuildID:   targetChannel.GuildID,
								ChannelID: targetChannel.ID,
							},
						)
						helpers.Relax(err)

						_, err = helpers.EventlogLog(time.Now(), targetChannel.GuildID, targetChannel.ID,
							models.EventlogTargetTypeChannel, msg.Author.ID,
							models.EventlogTypeRobyulNotificationsChannelIgnore, "",
							nil,
							[]models.ElasticEventlogOption{
								{
									Key:   "notifications_ignoredchannelids_added",
									Value: targetChannel.ID,
									Type:  models.EventlogTargetTypeChannel,
								},
							}, false)
						helpers.RelaxLog(err)

						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.notifications.ignore-channel-add-success", targetChannel.ID))

						go func() {
							defer helpers.Recover()

							err := refreshNotificationSettingsCache()
							helpers.RelaxLog(err)
						}()
						return
					}
					helpers.Relax(err)

					// Remove from list
					err = helpers.MDbDelete(models.NotificationsIgnoredChannelsTable, entryBucket.ID)
					helpers.Relax(err)

					_, err = helpers.EventlogLog(time.Now(), targetChannel.GuildID, targetChannel.ID,
						models.EventlogTargetTypeChannel, msg.Author.ID,
						models.EventlogTypeRobyulNotificationsChannelIgnore, "",
						nil,
						[]models.ElasticEventlogOption{
							{
								Key:   "notifications_ignoredchannelids_removed",
								Value: targetChannel.ID,
								Type:  models.EventlogTargetTypeChannel,
							},
						}, false)
					helpers.RelaxLog(err)

					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.notifications.ignore-channel-remove-success", targetChannel.ID))

					go func() {
						defer helpers.Recover()

						err := refreshNotificationSettingsCache()
						helpers.RelaxLog(err)
					}()
				})
			}
		case "toggle-mode", "toggle-modes", "toggle-layout", "toggle-layouts":
			session.ChannelTyping(msg.ChannelID)

			var message string

			newValue := helpers.GetUserConfigInt(msg.Author.ID, UserConfigNotificationsLayoutModeKey, 1) + 1
			if newValue > 4 {
				newValue = 1
			}

			switch newValue {
			case 2:
				message = helpers.GetTextF("plugins.notifications.mode-2")
				break
			case 3:
				message = helpers.GetTextF("plugins.notifications.mode-3")
				break
			case 4:
				message = helpers.GetTextF("plugins.notifications.mode-4")
				break
			default:
				message = helpers.GetTextF("plugins.notifications.mode-1")
				break
			}

			err := helpers.SetUserConfigInt(msg.Author.ID, UserConfigNotificationsLayoutModeKey, newValue)
			helpers.Relax(err)

			_, err = helpers.SendMessage(msg.ChannelID, message)
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			break
		}
	}

}

type PendingNotification struct {
	Member   *discordgo.Member
	Author   *discordgo.Member
	Keywords []string
}

func (m *Handler) OnMessage(session *discordgo.Session, msg *discordgo.MessageCreate) {
	if msg == nil || msg.Content == "" {
		return
	}

	if helpers.IsBlacklisted(msg.Author.ID) {
		return
	}

	channel, err := helpers.GetChannelWithoutApi(msg.ChannelID)
	if err != nil {
		helpers.RelaxLog(err)
		return
	}
	if channel.GuildID == "" {
		return
	}
	guild, err := helpers.GetGuild(channel.GuildID)
	if err != nil {
		helpers.RelaxLog(err)
		return
	}

	// ignore commands
	prefix := helpers.GetPrefixForServer(guild.ID)
	if prefix != "" {
		if strings.HasPrefix(msg.Content, prefix) {
			return
		}
	}
	// ignore music bot prefixes
	if strings.HasPrefix(msg.Content, "__") || strings.HasPrefix(msg.Content, "//") ||
		strings.HasPrefix(msg.Content, "___") || strings.HasPrefix(msg.Content, "///") {
		return
	}
	// ignore bot messages except whitelisted bots
	if msg.Author.Bot {
		var isWhitelisted bool
		for _, whitelistedBotID := range WhitelistedBotIDs {
			if msg.Author.ID == whitelistedBotID {
				isWhitelisted = true
			}
		}
		if !isWhitelisted {
			return
		}
	}

	for _, ignoredChannel := range ignoredChannelsCache {
		// ignore messages in ignored channels
		if ignoredChannel.ChannelID == msg.ChannelID {
			return
		}
		// ignore messages if parent channel is ignored and child is in sync
		if ignoredChannel.ChannelID == channel.ParentID {
			if helpers.ChannelPermissionsInSync(channel.ID) {
				return
			}
		}
	}

	// create a copy of the channel messages in the state
	session.State.RLock()
	messagesCopy := make([]*discordgo.Message, len(channel.Messages))
	copy(messagesCopy, channel.Messages)
	session.State.RUnlock()
	// sort in reverse order
	slice.Sort(messagesCopy, func(i, j int) bool {
		return messagesCopy[i].Timestamp > messagesCopy[j].Timestamp
	})
	// save context
	contextMessagesLimit := 4
	contextMessages := make([]*discordgo.Message, 0)
	var startCollecting bool
	for _, contextMessage := range messagesCopy {
		if contextMessagesLimit <= 0 {
			break
		}
		if contextMessage.ID == msg.ID {
			startCollecting = true
			continue
		}
		if startCollecting {
			contextMessagesLimit--
			contextMessages = append([]*discordgo.Message{contextMessage}, contextMessages...)
		}
	}

	var pendingNotifications []PendingNotification

	textToMatch := strings.ToLower(strings.TrimSpace(msg.Content))

NextKeyword:
	for _, notificationSetting := range notificationSettingsCache {
		if notificationSetting.GuildID == guild.ID || notificationSetting.GuildID == "global" {
			// check if message should be ignored for specific keyword
			if isIgnored(notificationSetting, msg.Message) {
				continue NextKeyword
			}

			if keywordMatches(textToMatch, notificationSetting.Keyword) {
				memberToNotify, err := helpers.GetGuildMemberWithoutApi(guild.ID, notificationSetting.UserID)
				if err != nil {
					//cache.GetLogger().WithField("module", "notifications").WithField("channelID", channel.ID).WithField("userID", notificationSetting.UserID).Warn("error getting member to notify: " + err.Error())
					continue NextKeyword
				}
				if memberToNotify == nil {
					//cache.GetLogger().WithField("module", "notifications").WithField("channelID", channel.ID).WithField("userID", notificationSetting.UserID).Warn("member to notify not found")
					continue NextKeyword
				}
				messageAuthor, err := helpers.GetGuildMemberWithoutApi(guild.ID, msg.Author.ID)
				if err != nil {
					messageAuthor = new(discordgo.Member)
					messageAuthor.User = msg.Author
				}
				hasReadPermissions := false
				hasHistoryPermissions := false
				// ignore messages if the users roles have no read permission to the server
				memberAllPermissions := helpers.GetAllPermissions(guild, memberToNotify)
				if memberAllPermissions&discordgo.PermissionReadMessageHistory == discordgo.PermissionReadMessageHistory {
					hasHistoryPermissions = true
					//fmt.Println(msg.Content, ": allowed History: A")
				}
				if memberAllPermissions&discordgo.PermissionReadMessages == discordgo.PermissionReadMessages {
					hasReadPermissions = true
					//fmt.Println(msg.Content, ": allowed Read: B")
				}
				// ignore messages if the users roles have no read permission to the channel
			NextPermOverwriteEveryone:
				for _, overwrite := range channel.PermissionOverwrites {
					if overwrite.Type == "role" {
						roleToCheck, err := session.State.Role(channel.GuildID, overwrite.ID)
						if err != nil {
							cache.GetLogger().WithField("module", "notifications").Warn("error getting role: " + err.Error())
							continue NextPermOverwriteEveryone
						}
						//fmt.Printf("%s: %#v\n", roleToCheck.Name, overwrite)

						if roleToCheck.Name == "@everyone" {
							if overwrite.Allow&discordgo.PermissionReadMessageHistory == discordgo.PermissionReadMessageHistory {
								hasHistoryPermissions = true
								//fmt.Println(msg.Content, ": allowed History: C")
							}
							if overwrite.Allow&discordgo.PermissionReadMessages == discordgo.PermissionReadMessages {
								hasReadPermissions = true
								//fmt.Println(msg.Content, ": allowed Read: D")
							}
							if overwrite.Deny&discordgo.PermissionReadMessageHistory == discordgo.PermissionReadMessageHistory {
								hasHistoryPermissions = false
								//fmt.Println(msg.Content, ": rejected History: E")
							}
							if overwrite.Deny&discordgo.PermissionReadMessages == discordgo.PermissionReadMessages {
								hasReadPermissions = false
								//fmt.Println(msg.Content, ": rejected Read: F")
							}
						}
					}
				}
			NextPermOverwriteNotEveryone:
				for _, overwrite := range channel.PermissionOverwrites {
					if overwrite.Type == "role" {
						roleToCheck, err := session.State.Role(channel.GuildID, overwrite.ID)
						if err != nil {
							cache.GetLogger().WithField("module", "notifications").Warn("error getting role: " + err.Error())
							continue NextPermOverwriteNotEveryone
						}
						//fmt.Printf("%s: %#v\n", roleToCheck.Name, overwrite)

						if roleToCheck.Name != "@everyone" {
							for _, memberRoleId := range memberToNotify.Roles {
								if memberRoleId == overwrite.ID {
									if overwrite.Allow&discordgo.PermissionReadMessageHistory == discordgo.PermissionReadMessageHistory {
										hasHistoryPermissions = true
										//fmt.Println(msg.Content, ": allowed History: G")
									}
									if overwrite.Allow&discordgo.PermissionReadMessages == discordgo.PermissionReadMessages {
										hasReadPermissions = true
										//fmt.Println(msg.Content, ": allowed Read: H")
									}
									if overwrite.Deny&discordgo.PermissionReadMessageHistory == discordgo.PermissionReadMessageHistory {
										hasHistoryPermissions = false
										//fmt.Println(msg.Content, ": rejected History: I")
									}
									if overwrite.Deny&discordgo.PermissionReadMessages == discordgo.PermissionReadMessages {
										hasReadPermissions = false
										//fmt.Println(msg.Content, ": rejected Read: J")
									}
								}
							}
						}
					}
				}
				for _, overwrite := range channel.PermissionOverwrites {
					if overwrite.Type == "member" {
						//memberToCheck, err := helpers.GetGuildMember(channel.GuildID, overwrite.ID)
						//if err == nil {
						//	fmt.Printf("%s: %#v\n", memberToCheck.User.Username, overwrite)
						//}

						if memberToNotify.User.ID == overwrite.ID {
							if overwrite.Allow&discordgo.PermissionReadMessageHistory == discordgo.PermissionReadMessageHistory {
								hasHistoryPermissions = true
								//fmt.Println(msg.Content, ": allowed History: K")
							}
							if overwrite.Allow&discordgo.PermissionReadMessages == discordgo.PermissionReadMessages {
								hasReadPermissions = true
								//fmt.Println(msg.Content, ": allowed Read: L")
							}
							if overwrite.Deny&discordgo.PermissionReadMessageHistory == discordgo.PermissionReadMessageHistory {
								hasHistoryPermissions = false
								//fmt.Println(msg.Content, ": rejected History: M")
							}
							if overwrite.Deny&discordgo.PermissionReadMessages == discordgo.PermissionReadMessages {
								hasReadPermissions = false
								//fmt.Println(msg.Content, ": rejected Read: N")
							}
						}
					}
				}
				if hasReadPermissions == true && hasHistoryPermissions == true {
					addedToExistingPendingNotifications := false
					for i, pendingNotification := range pendingNotifications {
						if pendingNotification.Member.User.ID == memberToNotify.User.ID {
							addedToExistingPendingNotifications = true
							alreadyInKeywordList := false
							for _, keyword := range pendingNotifications[i].Keywords {
								if keyword == notificationSetting.Keyword {
									alreadyInKeywordList = true
								}
							}
							if alreadyInKeywordList == false {
								pendingNotifications[i].Keywords = append(pendingNotification.Keywords, notificationSetting.Keyword)
							}
						}
					}
					if addedToExistingPendingNotifications == false {
						pendingNotifications = append(pendingNotifications, PendingNotification{
							Member:   memberToNotify,
							Author:   messageAuthor,
							Keywords: []string{notificationSetting.Keyword},
						})
					}
					idToIncrease := notificationSetting.ID
					go func() {
						defer helpers.Recover()

						err = helpers.MDbUpdateWithoutLogging(models.NotificationsTable, idToIncrease, bson.M{"$inc": bson.M{"triggered": 1}})
						helpers.RelaxLog(err)
					}()
				}
			}
		}
	}

	messageTime, err := msg.Timestamp.Parse()
	if err != nil {
		messageTime = time.Now()
	}

	for _, pendingNotification := range pendingNotifications {
		if !helpers.GetIsInGuild(pendingNotification.Member.GuildID, pendingNotification.Member.User.ID) {
			continue
		}

		dmChannel, err := session.UserChannelCreate(pendingNotification.Member.User.ID)
		if err != nil {
			continue
		}
		keywordsTriggeredText := ""
		for i, keyword := range pendingNotification.Keywords {
			keywordsTriggeredText += fmt.Sprintf("`%s`", keyword)
			if i+2 < len(pendingNotification.Keywords) {
				keywordsTriggeredText += ", "
			} else if (len(pendingNotification.Keywords) - (i + 1)) > 0 {
				keywordsTriggeredText += " and "
			}
		}

		if pendingNotification.Author == nil {
			cache.GetLogger().WithField("module", "notifications").WithField("channelID", channel.ID).Warn("notification source member is nil")
			continue
		}

		escapedContent := strings.Replace(msg.Content, "```", "", -1)
		escapedContent = strings.Replace(escapedContent, "`", "'", -1)
		escapedContent = strings.TrimSpace(strings.Trim(escapedContent, "\n"))

		switch helpers.GetUserConfigInt(pendingNotification.Member.User.ID, UserConfigNotificationsLayoutModeKey, 1) {
		case 2:
			for _, resultPage := range helpers.Pagify(fmt.Sprintf("```"+helpers.ZERO_WIDTH_SPACE+"%s```:bell: User `%s` mentioned %s in %s on `%s` at `%s UTC`.\n\u200B",
				escapedContent,
				pendingNotification.Author.User.Username,
				keywordsTriggeredText,
				fmt.Sprintf("<#%s>", channel.ID),
				guild.Name,
				messageTime.UTC().Format("15:04:05"),
			), "\n") {
				helpers.SendMessage(dmChannel.ID, resultPage)
			}
			break
		case 3:
			notificationEmbed := &discordgo.MessageEmbed{
				Description: fmt.Sprintf(
					"`@%s` mentioned %s on `%s` in `#%s`\n"+helpers.MessageDeeplink(msg.ChannelID, msg.ID),
					msg.Author.Username,
					keywordsTriggeredText,
					guild.Name,
					channel.Name,
				),
				Color: 0x0FADED,
				Thumbnail: &discordgo.MessageEmbedThumbnail{
					URL: msg.Author.AvatarURL("64"),
				},
				Author: &discordgo.MessageEmbedAuthor{
					Name: "Robyul Keyword Notification on " + guild.Name,
				},
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:  "Channel",
						Value: "<#" + channel.ID + ">",
					},
				},
			}
			for _, contextMessage := range contextMessages {
				contextMessageTime, err := contextMessage.Timestamp.Parse()
				if err != nil {
					messageTime = time.Now()
				}
				notificationEmbed.Fields = append(notificationEmbed.Fields, &discordgo.MessageEmbedField{
					Name:   "@" + contextMessage.Author.Username + "#" + contextMessage.Author.Discriminator + " at " + contextMessageTime.UTC().Format("15:04:05") + " UTC",
					Value:  contextMessage.Content,
					Inline: false,
				})
			}
			notificationEmbed.Fields = append(notificationEmbed.Fields, &discordgo.MessageEmbedField{
				Name:   "🔔 @" + msg.Author.Username + "#" + msg.Author.Discriminator + " at " + messageTime.UTC().Format("15:04:05") + " UTC",
				Value:  msg.Content,
				Inline: false,
			})
			if guild.Icon != "" {
				notificationEmbed.Author.IconURL = discordgo.EndpointGuildIcon(guild.ID, guild.Icon)
			}
			helpers.SendEmbed(dmChannel.ID, notificationEmbed)
			break
		case 4:
			for _, resultPage := range helpers.Pagify(fmt.Sprintf(":bell: User `%s` mentioned %s in %s on `%s` at `%s UTC`\n<%s>:\n```"+helpers.ZERO_WIDTH_SPACE+"%s```",
				pendingNotification.Author.User.Username,
				keywordsTriggeredText,
				fmt.Sprintf("<#%s>", channel.ID),
				guild.Name,
				messageTime.UTC().Format("15:04:05"),
				helpers.MessageDeeplink(msg.ChannelID, msg.ID),
				escapedContent,
			), "\n") {
				helpers.SendMessage(dmChannel.ID, resultPage)
			}
			break
		default:
			for _, resultPage := range helpers.Pagify(fmt.Sprintf(":bell: User `%s` mentioned %s in %s on `%s` at `%s UTC`:\n```"+helpers.ZERO_WIDTH_SPACE+"%s```",
				pendingNotification.Author.User.Username,
				keywordsTriggeredText,
				fmt.Sprintf("<#%s>", channel.ID),
				guild.Name,
				messageTime.UTC().Format("15:04:05"),
				escapedContent,
			), "\n") {
				helpers.SendMessage(dmChannel.ID, resultPage)
			}
			break
		}
		metrics.KeywordNotificationsSentCount.Add(1)
	}
}

type delimiterCombination struct {
	Start string
	End   string
}
