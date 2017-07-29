package plugins

import (
    "github.com/bwmarrin/discordgo"
    "strings"
    rethink "github.com/gorethink/gorethink"
    "github.com/Seklfreak/Robyul2/helpers"
    "fmt"
    "github.com/Seklfreak/Robyul2/logger"
    "github.com/Seklfreak/Robyul2/metrics"
)

type Notifications struct{}

var (
    notificationSettingsCache []DB_NotificationSetting
    ignoredChannelsCache      []DB_IgnoredChannel
    ValidTextDelimiters       = []string{" ", ".", ",", "?", "!", ";", "(", ")", "=", "\"", "'", "`", "´", "_", "~", "+", "-", "/", ":", "*", "\n"}
)

type DB_IgnoredChannel struct {
    ID        string `gorethink:"id,omitempty"`
    GuildID   string `gorethink:"guildid"`
    ChannelID string `gorethink:"channelid"`
}

type DB_NotificationSetting struct {
    ID        string `gorethink:"id,omitempty"`
    Keyword   string `gorethink:"keyword"`
    GuildID   string `gorethink:"guildid"` // can be "global" to affect every guild
    UserID    string `gorethink:"userid"`
    Triggered int `gorethink:"triggered"`
}

func (m *Notifications) Commands() []string {
    return []string{
        "notifications",
        "notification",
        "noti",
        "notis",
    }
}

func (m *Notifications) Init(session *discordgo.Session) {
    go m.refreshNotificationSettingsCache()
}

// @TODO: add command to make a keyword global (owner only)

func (m *Notifications) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    args := strings.Fields(content)
    if len(args) > 0 {
        switch args[0] {
        case "add": // [p]notifications add <keyword(s)>
            if len(args) < 2 {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
                return
            }
            channel, err := helpers.GetChannel(msg.ChannelID)
            helpers.Relax(err)
            guild, err := helpers.GetGuild(channel.GuildID)
            helpers.Relax(err)

            var entryBucket DB_NotificationSetting
            listCursor, err := rethink.Table("notifications").Filter(
                rethink.Or(
                    rethink.Row.Field("guildid").Eq(guild.ID),
                    rethink.Row.Field("guildid").Eq("global"),
                ),
            ).Filter(
                rethink.Row.Field("userid").Eq(msg.Author.ID),
            ).Filter(
                rethink.Row.Field("keyword").Eq(strings.Join(args[1:], " ")),
            ).Run(helpers.GetDB())
            defer listCursor.Close()
            err = listCursor.One(&entryBucket)

            if err != rethink.ErrEmptyResult || entryBucket.ID != "" {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.notifications.keyword-add-error-duplicate", msg.Author.ID))
                session.ChannelMessageDelete(msg.ChannelID, msg.ID) // Do not get error as it might fail because deletion permissions are not given to the user
                return
            } else if err != nil && err != rethink.ErrEmptyResult {
                helpers.Relax(err)
            }

            entry := m.getNotificationSettingByOrCreateEmpty("id", "")
            entry.Keyword = strings.Join(args[1:], " ")
            entry.GuildID = guild.ID
            entry.UserID = msg.Author.ID
            m.setNotificationSetting(entry)

            _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.notifications.keyword-added-success", msg.Author.ID))
            helpers.Relax(err)
            logger.INFO.L("notifications", fmt.Sprintf("Added Notification Keyword \"%s\" to Guild %s (#%s) for User %s (#%s)", entry.Keyword, guild.Name, guild.ID, msg.Author.Username, msg.Author.ID))
            session.ChannelMessageDelete(msg.ChannelID, msg.ID) // Do not get error as it might fail because deletion permissions are not given to the user
            go m.refreshNotificationSettingsCache()
        case "delete", "del", "remove": // [p]notifications delete <keyword(s)>
            channel, err := helpers.GetChannel(msg.ChannelID)
            helpers.Relax(err)
            guild, err := helpers.GetGuild(channel.GuildID)
            helpers.Relax(err)
            if len(args) < 2 {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                return
            }
            session.ChannelTyping(msg.ChannelID)

            var entryBucket DB_NotificationSetting
            listCursor, err := rethink.Table("notifications").Filter(
                rethink.Or(
                    rethink.Row.Field("guildid").Eq(guild.ID),
                    rethink.Row.Field("guildid").Eq("global"),
                ),
            ).Filter(
                rethink.Row.Field("userid").Eq(msg.Author.ID),
            ).Filter(
                rethink.Row.Field("keyword").Eq(strings.Join(args[1:], " ")),
            ).Run(helpers.GetDB())
            defer listCursor.Close()
            err = listCursor.One(&entryBucket)

            if err == rethink.ErrEmptyResult || entryBucket.ID == "" {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.notifications.keyword-delete-not-found-error", msg.Author.ID))
                return
            } else if err != nil {
                helpers.Relax(err)
            }
            m.deleteNotificationSettingByID(entryBucket.ID)

            session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.notifications.keyword-delete-success", msg.Author.ID))
            logger.INFO.L("notifications", fmt.Sprintf("Deleted Notificaton Keyword \"%s\" from Guild %s (#%s) for User %s (#%s)", entryBucket.Keyword, guild.Name, guild.ID, msg.Author.Username, msg.Author.ID))
            session.ChannelMessageDelete(msg.ChannelID, msg.ID) // Do not get error as it might fail because deletion permissions are not given to the user
            go m.refreshNotificationSettingsCache()
        case "list": // [p]notifications list
            channel, err := helpers.GetChannel(msg.ChannelID)
            helpers.Relax(err)
            guild, err := helpers.GetGuild(channel.GuildID)
            helpers.Relax(err)
            var entryBucket []DB_NotificationSetting
            listCursor, err := rethink.Table("notifications").Filter(
                rethink.Or(
                    rethink.Row.Field("guildid").Eq(guild.ID),
                    rethink.Row.Field("guildid").Eq("global"),
                ),
            ).Filter(
                rethink.Row.Field("userid").Eq(msg.Author.ID),
            ).OrderBy(rethink.Desc("triggered")).Run(helpers.GetDB())
            helpers.Relax(err)
            defer listCursor.Close()
            err = listCursor.All(&entryBucket)

            if err == rethink.ErrEmptyResult || len(entryBucket) <= 0 {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.notifications.keyword-list-no-keywords-error", msg.Author.ID))
                return
            } else if err != nil {
                helpers.Relax(err)
            }

            resultMessage := fmt.Sprintf("Enabled keywords for the server: `%s`:\n", guild.Name)
            for _, entry := range entryBucket {
                if entry.GuildID == "global" {
                    resultMessage += fmt.Sprintf("**GLOBAL KEYWORD:** `%s` (triggered `%d` times)\n", entry.Keyword, entry.Triggered)
                } else {
                    resultMessage += fmt.Sprintf("`%s` (triggered `%d` times)\n", entry.Keyword, entry.Triggered)
                }
            }
            resultMessage += fmt.Sprintf("Found **%d** Keywords in total.", len(entryBucket))

            dmChannel, err := session.UserChannelCreate(msg.Author.ID)
            helpers.Relax(err)

            for _, resultPage := range helpers.Pagify(resultMessage, "\n") {
                _, err := session.ChannelMessageSend(dmChannel.ID, resultPage)
                helpers.Relax(err)
            }

            _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.check-your-dms", msg.Author.ID))
            helpers.Relax(err)
        case "ignore-channel":
            if len(args) < 2 {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
                return
            }
            commandIssueChannel, err := helpers.GetChannel(msg.ChannelID)
            helpers.Relax(err)
            switch args[1] {
            case "list": // [p]notifications ignore-channel list
                var entryBucket []DB_IgnoredChannel
                listCursor, err := rethink.Table("notifications_ignored_channels").Filter(
                    rethink.Or(
                        rethink.Row.Field("guildid").Eq(commandIssueChannel.GuildID),
                    ),
                ).Run(helpers.GetDB())
                helpers.Relax(err)
                defer listCursor.Close()
                err = listCursor.All(&entryBucket)

                if err == rethink.ErrEmptyResult || len(entryBucket) <= 0 {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.notifications.ignoredchannels-list-no-keywords-error"))
                    return
                } else if err != nil {
                    helpers.Relax(err)
                }

                resultMessage := fmt.Sprintf("Ignored channels on this server:\n")
                for _, entry := range entryBucket {
                    resultMessage += fmt.Sprintf("<#%s>\n", entry.ChannelID)
                }
                resultMessage += fmt.Sprintf("Found **%d** Ignored Channels in total.", len(entryBucket))

                _, err = session.ChannelMessageSend(msg.ChannelID, resultMessage)
                helpers.Relax(err)
            default: // [p]notifications ignore-channel <channel>
                helpers.RequireAdmin(msg, func() {
                    targetChannel, err := helpers.GetChannelFromMention(msg, args[1])
                    helpers.Relax(err)

                    if targetChannel.GuildID != commandIssueChannel.GuildID {
                        session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.notifications.ignore-channel-addorremove-error-server"))
                        return
                    }

                    ignoredChannel := m.getIgnoredChannelBy("channelid", targetChannel.ID)
                    if ignoredChannel.ID == "" {
                        // Add to list
                        ignoredChannel := m.getIgnoredChannelByOrCreateEmpty("id", "")
                        ignoredChannel.ChannelID = targetChannel.ID
                        ignoredChannel.GuildID = targetChannel.GuildID
                        m.setIgnoredChannel(ignoredChannel)
                        session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.notifications.ignore-channel-add-success", targetChannel.ID))
                    } else {
                        // Remove from list
                        m.deleteIgnoredChannel(ignoredChannel.ID)
                        session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.notifications.ignore-channel-remove-success", targetChannel.ID))
                    }
                    go m.refreshNotificationSettingsCache()
                })
            }
        }
    }

}

type PendingNotification struct {
    Member   *discordgo.Member
    Author   *discordgo.Member
    Keywords []string
}

func (m *Notifications) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {
    channel, err := helpers.GetChannel(msg.ChannelID)
    helpers.Relax(err)
    guild, err := helpers.GetGuild(channel.GuildID)
    helpers.Relax(err)

    // ignore commands
    prefix := helpers.GetPrefixForServer(guild.ID)
    if prefix != "" {
        if strings.HasPrefix(content, prefix) {
            return
        }
    }
    // ignore bot messages
    if msg.Author.Bot == true {
        return
    }

    // ignore messages in ignored channels
    for _, ignoredChannel := range ignoredChannelsCache {
        if ignoredChannel.ChannelID == msg.ChannelID {
            return
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
                memberToNotify, err := helpers.GetGuildMember(guild.ID, notificationSetting.UserID)
                if err != nil {
                    logger.ERROR.L("notifications", "error getting member to notify: "+err.Error())
                    continue NextKeyword
                }
                if memberToNotify == nil {
                    logger.ERROR.L("notifications", "member to notify not found")
                    continue NextKeyword
                }
                messageAuthor, err := helpers.GetGuildMember(guild.ID, msg.Author.ID)
                if err != nil {
                    logger.ERROR.L("notifications", "error getting message author: "+err.Error())
                    continue NextKeyword
                }
                hasReadPermissions := false
                hasHistoryPermissions := false
                // ignore messages if the users roles have no read permission to the server
                memberAllPermissions := helpers.GetAllPermissions(guild, memberToNotify)
                if memberAllPermissions&discordgo.PermissionReadMessageHistory == discordgo.PermissionReadMessageHistory {
                    hasHistoryPermissions = true
                    //fmt.Println("allowed History: A")
                }
                if memberAllPermissions&discordgo.PermissionReadMessages == discordgo.PermissionReadMessages {
                    hasReadPermissions = true
                    //fmt.Println("allowed Read: B")
                }
                // ignore messages if the users roles have no read permission to the channel
            NextPermOverwrite:
                for _, overwrite := range channel.PermissionOverwrites {
                    if overwrite.Type == "role" {
                        roleToCheck, err := session.State.Role(channel.GuildID, overwrite.ID)
                        if err != nil {
                            logger.ERROR.L("notifications", "error getting role: "+err.Error())
                            continue NextPermOverwrite
                        }
                        //fmt.Printf("%s: %#v\n", roleToCheck.Name, overwrite)

                        if roleToCheck.Name == "@everyone" {
                            if overwrite.Allow&discordgo.PermissionReadMessageHistory == discordgo.PermissionReadMessageHistory {
                                hasHistoryPermissions = true
                                //fmt.Println("allowed History: C")
                            }
                            if overwrite.Allow&discordgo.PermissionReadMessages == discordgo.PermissionReadMessages {
                                hasReadPermissions = true
                                //fmt.Println("allowed Read: D")
                            }
                            if overwrite.Deny&discordgo.PermissionReadMessageHistory == discordgo.PermissionReadMessageHistory {
                                hasHistoryPermissions = false
                                //fmt.Println("rejected History: E")
                            }
                            if overwrite.Deny&discordgo.PermissionReadMessages == discordgo.PermissionReadMessages {
                                hasReadPermissions = false
                                //fmt.Println("rejected Read: F")
                            }
                        } else {
                            for _, memberRoleId := range memberToNotify.Roles {
                                if memberRoleId == overwrite.ID {
                                    if overwrite.Allow&discordgo.PermissionReadMessageHistory == discordgo.PermissionReadMessageHistory {
                                        hasHistoryPermissions = true
                                        //fmt.Println("allowed History: G")
                                    }
                                    if overwrite.Allow&discordgo.PermissionReadMessages == discordgo.PermissionReadMessages {
                                        hasReadPermissions = true
                                        //fmt.Println("allowed Read: H")
                                    }
                                    if overwrite.Deny&discordgo.PermissionReadMessageHistory == discordgo.PermissionReadMessageHistory {
                                        hasHistoryPermissions = false
                                        //fmt.Println("rejected History: I")
                                    }
                                    if overwrite.Deny&discordgo.PermissionReadMessages == discordgo.PermissionReadMessages {
                                        hasReadPermissions = false
                                        //fmt.Println("rejected Read: J")
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
                        //    fmt.Printf("%s: %#v\n", memberToCheck.User.Username, overwrite)
                        //}

                        if memberToNotify.User.ID == overwrite.ID {
                            if overwrite.Allow&discordgo.PermissionReadMessageHistory == discordgo.PermissionReadMessageHistory {
                                hasHistoryPermissions = true
                                //fmt.Println("allowed History: K")
                            }
                            if overwrite.Allow&discordgo.PermissionReadMessages == discordgo.PermissionReadMessages {
                                hasReadPermissions = true
                                //fmt.Println("allowed Read: L")
                            }
                            if overwrite.Deny&discordgo.PermissionReadMessageHistory == discordgo.PermissionReadMessageHistory {
                                hasHistoryPermissions = false
                                //fmt.Println("rejected History: M")
                            }
                            if overwrite.Deny&discordgo.PermissionReadMessages == discordgo.PermissionReadMessages {
                                hasReadPermissions = false
                                //fmt.Println("rejected Read: N")
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
                    go m.increaseNotificationEntryById(notificationSetting.ID)
                }
            }
        }
    }

    for _, pendingNotification := range pendingNotifications {
        dmChannel, err := session.UserChannelCreate(pendingNotification.Member.User.ID)
        if err != nil {
            logger.ERROR.L("notifications", "error creating DM channel: "+err.Error())
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

        for _, resultPage := range helpers.Pagify(fmt.Sprintf(":bell: User `%s` mentioned %s in %s on the server `%s`:\n```%s```",
            pendingNotification.Author.User.Username,
            keywordsTriggeredText,
            fmt.Sprintf("<#%s>", channel.ID),
            guild.Name,
            content,
        ), "\n") {
            _, err := session.ChannelMessageSend(dmChannel.ID, resultPage)
            if err != nil {
                logger.ERROR.L("notifications", "error sending DM: "+err.Error())
                continue
            }
        }
        metrics.KeywordNotificationsSentCount.Add(1)
    }
}

func (m *Notifications) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {

}

func (m *Notifications) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {

}

func (m *Notifications) getIgnoredChannelBy(key string, id string) DB_IgnoredChannel {
    var entryBucket DB_IgnoredChannel
    listCursor, err := rethink.Table("notifications_ignored_channels").Filter(
        rethink.Row.Field(key).Eq(id),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.One(&entryBucket)

    if err == rethink.ErrEmptyResult {
        return entryBucket
    } else if err != nil {
        panic(err)
    }

    return entryBucket
}

func (m *Notifications) getIgnoredChannelByOrCreateEmpty(key string, id string) DB_IgnoredChannel {
    var entryBucket DB_IgnoredChannel
    listCursor, err := rethink.Table("notifications_ignored_channels").Filter(
        rethink.Row.Field(key).Eq(id),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.One(&entryBucket)

    // If user has no DB entries create an empty document
    if err == rethink.ErrEmptyResult {
        insert := rethink.Table("notifications_ignored_channels").Insert(DB_IgnoredChannel{})
        res, e := insert.RunWrite(helpers.GetDB())
        // If the creation was successful read the document
        if e != nil {
            panic(e)
        } else {
            return m.getIgnoredChannelByOrCreateEmpty("id", res.GeneratedKeys[0])
        }
    } else if err != nil {
        panic(err)
    }

    return entryBucket
}

func (m *Notifications) setIgnoredChannel(entry DB_IgnoredChannel) {
    _, err := rethink.Table("notifications_ignored_channels").Update(entry).Run(helpers.GetDB())
    helpers.Relax(err)
}

func (m *Notifications) deleteIgnoredChannel(id string) {
    _, err := rethink.Table("notifications_ignored_channels").Filter(
        rethink.Row.Field("id").Eq(id),
    ).Delete().RunWrite(helpers.GetDB())
    helpers.Relax(err)
}

func (m *Notifications) getNotificationSettingBy(key string, id string) DB_NotificationSetting {
    var entryBucket DB_NotificationSetting
    listCursor, err := rethink.Table("notifications").Filter(
        rethink.Row.Field(key).Eq(id),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.One(&entryBucket)

    if err == rethink.ErrEmptyResult {
        return entryBucket
    } else if err != nil {
        panic(err)
    }

    return entryBucket
}

func (m *Notifications) getNotificationSettingByOrCreateEmpty(key string, id string) DB_NotificationSetting {
    var entryBucket DB_NotificationSetting
    listCursor, err := rethink.Table("notifications").Filter(
        rethink.Row.Field(key).Eq(id),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.One(&entryBucket)

    // If user has no DB entries create an empty document
    if err == rethink.ErrEmptyResult {
        insert := rethink.Table("notifications").Insert(DB_NotificationSetting{})
        res, e := insert.RunWrite(helpers.GetDB())
        // If the creation was successful read the document
        if e != nil {
            panic(e)
        } else {
            return m.getNotificationSettingByOrCreateEmpty("id", res.GeneratedKeys[0])
        }
    } else if err != nil {
        panic(err)
    }

    return entryBucket
}

func (m *Notifications) setNotificationSetting(entry DB_NotificationSetting) {
    _, err := rethink.Table("notifications").Update(entry).Run(helpers.GetDB())
    helpers.Relax(err)
}

func (m *Notifications) deleteNotificationSettingByID(id string) {
    _, err := rethink.Table("notifications").Filter(
        rethink.Row.Field("id").Eq(id),
    ).Delete().RunWrite(helpers.GetDB())
    helpers.Relax(err)
}

func (m *Notifications) refreshNotificationSettingsCache() {
    cursor, err := rethink.Table("notifications").Run(helpers.GetDB())
    helpers.Relax(err)
    err = cursor.All(&notificationSettingsCache)
    helpers.Relax(err)
    cursor, err = rethink.Table("notifications_ignored_channels").Run(helpers.GetDB())
    helpers.Relax(err)
    err = cursor.All(&ignoredChannelsCache)
    helpers.Relax(err)

    logger.INFO.L("notifications", fmt.Sprintf("Refreshed Notification Settings Cache: Got %d keywords and %d ignored channels",
        len(notificationSettingsCache), len(ignoredChannelsCache)))
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

func (m *Notifications) increaseNotificationEntryById(id string) {
    var entryBucket DB_NotificationSetting
    listCursor, err := rethink.Table("notifications").Filter(
        rethink.Row.Field("id").Eq(id),
    ).Run(helpers.GetDB())
    helpers.Relax(err)
    defer listCursor.Close()
    err = listCursor.One(&entryBucket)
    helpers.Relax(err)

    entryBucket.Triggered += 1
    m.setNotificationSetting(entryBucket)
}

func (m *Notifications) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {

}
func (m *Notifications) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {

}
func (m *Notifications) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {

}
func (m *Notifications) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}
