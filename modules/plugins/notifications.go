package plugins

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

type Notifications struct{}

var (
	notificationSettingsCache      []models.NotificationsEntry
	ignoredChannelsCache           []models.NotificationsIgnoredChannelsEntry
	ValidTextDelimiters            = []string{" ", ".", ",", "?", "!", ";", "(", ")", "=", "\"", "'", "`", "´", "_", "~", "+", "-", "/", ":", "*", "\n", "…", "’", "“"}
	NotificationsWhitelistedBotIDs = []string{
		"178215222614556673", // Fiscord-IRC (Kakkela)
		"232927528325611521", // TrelleIRC (Kakkela)
	}
)

const (
	UserConfigNotificationsLayoutModeKey = "notifications:layout-mode"
)

func (m *Notifications) Commands() []string {
	return []string{
		"notifications",
		"notification",
		"noti",
		"notis",
	}
}

func (m *Notifications) Init(session *discordgo.Session) {
	go func() {
		defer helpers.Recover()

		err := m.refreshNotificationSettingsCache()
		helpers.RelaxLog(err)
	}()
}

func (m *Notifications) Uninit(session *discordgo.Session) {

}

func (m *Notifications) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
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

				err := m.refreshNotificationSettingsCache()
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

				err := m.refreshNotificationSettingsCache()
				helpers.RelaxLog(err)
			}()
		case "list": // [p]notifications list
			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)
			guild, err := helpers.GetGuild(channel.GuildID)
			helpers.Relax(err)
			var entryBucket []models.NotificationsEntry
			err = helpers.MDbIter(helpers.MdbCollection(models.NotificationsTable).Find(bson.M{
				"userid":  msg.Author.ID,
				"guildid": bson.M{"$in": []string{guild.ID, "global"}},
			}).Sort("-triggered")).All(&entryBucket)
			helpers.Relax(err)

			if entryBucket == nil || len(entryBucket) <= 0 {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.notifications.keyword-list-no-keywords-error", msg.Author.ID))
				return
			} else if err != nil {
				helpers.Relax(err)
			}

			resultMessage := fmt.Sprintf("Enabled keywords for the server: `%s`:\n", guild.Name)
			for _, entry := range entryBucket {
				resultMessage += fmt.Sprintf("`%s` (triggered `%d` times)", entry.Keyword, entry.Triggered)
				if entry.GuildID == "global" {
					resultMessage += " `[Global Keyword]` :globe_with_meridians:"
				}
				resultMessage += "\n"
			}
			resultMessage += fmt.Sprintf("Found **%d** Keywords in total.", len(entryBucket))

			dmChannel, err := session.UserChannelCreate(msg.Author.ID)
			helpers.Relax(err)

			for _, resultPage := range helpers.Pagify(resultMessage, "\n") {
				_, err := helpers.SendMessage(dmChannel.ID, resultPage)
				helpers.RelaxMessage(err, "", "")
			}

			_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.check-your-dms", msg.Author.ID))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
		case "ignore":
			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)

			if len(args) < 2 {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
				return
			}
			session.ChannelTyping(msg.ChannelID)

			keywords := strings.TrimSpace(strings.Replace(content, args[0], "", 1))

			var entryBucket models.NotificationsEntry
			err = helpers.MdbOne(
				helpers.MdbCollection(models.NotificationsTable).Find(
					bson.M{"userid": msg.Author.ID,
						"guildid": "global",
						"keyword": bson.M{"$regex": bson.RegEx{Pattern: "^" + regexp.QuoteMeta(keywords) + "$", Options: "i"}}}),
				&entryBucket,
			)
			if helpers.IsMdbNotFound(err) {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.notifications.keyword-ignore-not-found-error"))
				return
			}
			helpers.Relax(err)

			ignoredGuildIDsWithout := make([]string, 0)
			for _, alreadyIgnoredGuildID := range entryBucket.IgnoredGuildIDs {
				if alreadyIgnoredGuildID != channel.GuildID {
					ignoredGuildIDsWithout = append(ignoredGuildIDsWithout, alreadyIgnoredGuildID)
				}
			}

			var message string
			if len(ignoredGuildIDsWithout) != len(entryBucket.IgnoredGuildIDs) {
				// remove ignoring
				entryBucket.IgnoredGuildIDs = ignoredGuildIDsWithout
				message = helpers.GetText("plugins.notifications.keyword-ignore-guild-removed")
			} else {
				// add ignoring
				entryBucket.IgnoredGuildIDs = append(entryBucket.IgnoredGuildIDs, channel.GuildID)
				message = helpers.GetText("plugins.notifications.keyword-ignore-guild-added")
			}

			err = helpers.MDbUpdate(models.NotificationsTable, entryBucket.ID, entryBucket)
			helpers.Relax(err)

			go func() {
				defer helpers.Recover()

				err := m.refreshNotificationSettingsCache()
				helpers.RelaxLog(err)
			}()

			_, err = helpers.SendMessage(msg.ChannelID, message)
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
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
								},
							}, false)
						helpers.RelaxLog(err)

						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.notifications.ignore-channel-add-success", targetChannel.ID))

						go func() {
							defer helpers.Recover()

							err := m.refreshNotificationSettingsCache()
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
							},
						}, false)
					helpers.RelaxLog(err)

					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.notifications.ignore-channel-remove-success", targetChannel.ID))

					go func() {
						defer helpers.Recover()

						err := m.refreshNotificationSettingsCache()
						helpers.RelaxLog(err)
					}()
				})
			}
		case "toggle-mode", "toggle-modes":
			session.ChannelTyping(msg.ChannelID)

			var message string

			newValue := helpers.GetUserConfigInt(msg.Author.ID, UserConfigNotificationsLayoutModeKey, 1) + 1
			if newValue > 3 {
				newValue = 1
			}

			switch newValue {
			case 2:
				message = helpers.GetTextF("plugins.notifications.mode-2")
				break
			case 3:
				message = helpers.GetTextF("plugins.notifications.mode-3")
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

func (m *Notifications) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {
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
		if strings.HasPrefix(content, prefix) {
			return
		}
	}
	// ignore music bot prefixes
	if strings.HasPrefix(content, "__") || strings.HasPrefix(content, "//") ||
		strings.HasPrefix(content, "___") || strings.HasPrefix(content, "///") {
		return
	}
	// ignore bot messages except whitelisted bots
	if msg.Author.Bot {
		var isWhitelisted bool
		for _, whitelistedBotID := range NotificationsWhitelistedBotIDs {
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

NextKeyword:
	for _, notificationSetting := range notificationSettingsCache {
		if notificationSetting.GuildID == guild.ID || notificationSetting.GuildID == "global" {
			// ignore messages by the notification setting author
			if notificationSetting.UserID == msg.Author.ID {
				continue NextKeyword
			}
			// ignore message if in ignored guild for global keywords
			if notificationSetting.IgnoredGuildIDs != nil && len(notificationSetting.IgnoredGuildIDs) > 0 {
				for _, ignoredGuildID := range notificationSetting.IgnoredGuildIDs {
					if ignoredGuildID == guild.ID {
						continue NextKeyword
					}
				}
			}

			matchContent := strings.ToLower(strings.TrimSpace(content))
			doesMatch := false
			for _, combination := range m.getAllDelimiterCombinations(ValidTextDelimiters) {
				if strings.Contains(matchContent, strings.ToLower(combination.Start+notificationSetting.Keyword+combination.End)) {
					doesMatch = true
				}
			}
			for _, delimiter := range ValidTextDelimiters {
				if strings.HasPrefix(matchContent, strings.ToLower(notificationSetting.Keyword+delimiter)) {
					doesMatch = true
				}
			}
			for _, delimiter := range ValidTextDelimiters {
				if strings.HasSuffix(matchContent, strings.ToLower(delimiter+notificationSetting.Keyword)) {
					doesMatch = true
				}
			}
			if matchContent == strings.ToLower(notificationSetting.Keyword) {
				doesMatch = true
			}
			if doesMatch == true {
				memberToNotify, err := helpers.GetGuildMemberWithoutApi(guild.ID, notificationSetting.UserID)
				if err != nil {
					cache.GetLogger().WithField("module", "notifications").WithField("channelID", channel.ID).WithField("userID", notificationSetting.UserID).Warn("error getting member to notify: " + err.Error())
					continue NextKeyword
				}
				if memberToNotify == nil {
					cache.GetLogger().WithField("module", "notifications").WithField("channelID", channel.ID).WithField("userID", notificationSetting.UserID).Warn("member to notify not found")
					continue NextKeyword
				}
				messageAuthor, err := helpers.GetGuildMemberWithoutApi(guild.ID, msg.Author.ID)
				if err != nil {
					cache.GetLogger().WithField("module", "notifications").WithField("channelID", channel.ID).WithField("userID", msg.Author.ID).Warn("error getting message author: " + err.Error())
					continue NextKeyword
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
		dmChannel, err := session.UserChannelCreate(pendingNotification.Member.User.ID)
		if err != nil {
			cache.GetLogger().WithField("module", "notifications").WithField("channelID", channel.ID).WithField("userID", pendingNotification.Member.User.ID).Warn("error creating DM channel: " + err.Error())
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

		switch helpers.GetUserConfigInt(pendingNotification.Member.User.ID, UserConfigNotificationsLayoutModeKey, 1) {
		case 2:
			for _, resultPage := range helpers.Pagify(fmt.Sprintf("```"+helpers.ZERO_WIDTH_SPACE+"%s```:bell: User `%s` mentioned %s in %s on `%s` at `%s UTC`.\n\u200B",
				content,
				pendingNotification.Author.User.Username,
				keywordsTriggeredText,
				fmt.Sprintf("<#%s>", channel.ID),
				guild.Name,
				messageTime.UTC().Format("15:04:05"),
			), "\n") {
				_, err := helpers.SendMessage(dmChannel.ID, resultPage)
				if err != nil {
					cache.GetLogger().WithField("module", "notifications").Warn("error sending DM: " + err.Error())
					continue
				}
			}
			break
		case 3:
			notificationEmbed := &discordgo.MessageEmbed{
				Description: fmt.Sprintf(
					"`@%s` mentioned %s on `%s` in `#%s`",
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
				Value:  content,
				Inline: false,
			})
			if guild.Icon != "" {
				notificationEmbed.Author.IconURL = discordgo.EndpointGuildIcon(guild.ID, guild.Icon)
			}
			_, err = helpers.SendEmbed(dmChannel.ID, notificationEmbed)
			if err != nil {
				cache.GetLogger().WithField("module", "notifications").Warn("error sending DM: " + err.Error())
			}
			break
		default:
			for _, resultPage := range helpers.Pagify(fmt.Sprintf(":bell: User `%s` mentioned %s in %s on `%s` at `%s UTC`:\n```"+helpers.ZERO_WIDTH_SPACE+"%s```",
				pendingNotification.Author.User.Username,
				keywordsTriggeredText,
				fmt.Sprintf("<#%s>", channel.ID),
				guild.Name,
				messageTime.UTC().Format("15:04:05"),
				content,
			), "\n") {
				_, err := helpers.SendMessage(dmChannel.ID, resultPage)
				if err != nil {
					cache.GetLogger().WithField("module", "notifications").Warn("error sending DM: " + err.Error())
					continue
				}
			}
			break
		}
		metrics.KeywordNotificationsSentCount.Add(1)
	}
}

func (m *Notifications) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {

}

func (m *Notifications) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {

}

func (m *Notifications) refreshNotificationSettingsCache() (err error) {
	err = helpers.MDbIter(helpers.MdbCollection(models.NotificationsTable).Find(nil)).All(&notificationSettingsCache)
	if err != nil {
		return err
	}
	err = helpers.MDbIter(helpers.MdbCollection(models.NotificationsIgnoredChannelsTable).Find(nil)).All(&ignoredChannelsCache)
	if err != nil {
		return err
	}

	cache.GetLogger().WithField("module", "notifications").Info(fmt.Sprintf("Refreshed Notification Settings Cache: Got %d keywords and %d ignored channels",
		len(notificationSettingsCache), len(ignoredChannelsCache)))
	return nil
}

type delimiterCombination struct {
	Start string
	End   string
}

func (m *Notifications) getAllDelimiterCombinations(delimiters []string) []delimiterCombination {
	var result []delimiterCombination
	for _, delimiterStart := range delimiters {
		for _, delimiterEnd := range delimiters {
			result = append(result, delimiterCombination{Start: delimiterStart, End: delimiterEnd})
		}
	}
	return result
}

func (m *Notifications) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {

}
func (m *Notifications) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {

}
func (m *Notifications) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {

}
func (m *Notifications) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}
func (m *Notifications) OnMessageDelete(msg *discordgo.MessageDelete, session *discordgo.Session) {

}
