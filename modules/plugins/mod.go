package plugins

import (
    "fmt"
    "github.com/Seklfreak/Robyul2/helpers"
    "github.com/Seklfreak/Robyul2/logger"
    "github.com/bwmarrin/discordgo"
    "regexp"
    "strconv"
    "strings"
    "github.com/Seklfreak/Robyul2/cache"
    "github.com/getsentry/raven-go"
    "time"
)

type Mod struct{}

func (m *Mod) Commands() []string {
    return []string{
        "cleanup",
        "mute",
        "unmute",
        "ban",
        "kick",
        "serverlist",
        "echo",
        "inspect",
        "auto-inspects-channel",
    }
}

func (m *Mod) Init(session *discordgo.Session) {

}

func (m *Mod) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    regexNumberOnly := regexp.MustCompile(`^\d+$`)

    switch command {
    case "cleanup":
        helpers.RequireMod(msg, func() {
            args := strings.Fields(content)
            if len(args) > 0 {
                switch args[0] {
                case "after": // [p]cleanup after <after message id> [<until message id>]
                    if len(args) < 2 {
                        session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
                        return
                    } else {
                        afterMessageId := args[1]
                        untilMessageId := ""
                        if regexNumberOnly.MatchString(afterMessageId) == false {
                            session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
                            return
                        }
                        if len(args) >= 3 {
                            untilMessageId = args[2]
                            if regexNumberOnly.MatchString(untilMessageId) == false {
                                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
                                return
                            }
                        }
                        messagesToDeleteAfter, _ := session.ChannelMessages(msg.ChannelID, 100, "", afterMessageId)
                        messagesToDeleteBefore := []*discordgo.Message{}
                        if untilMessageId != "" {
                            messagesToDeleteBefore, _ = session.ChannelMessages(msg.ChannelID, 100, "", untilMessageId)
                        }
                        messagesToDeleteIds := []string{msg.ID}
                        for _, messageToDelete := range messagesToDeleteAfter {
                            isExcluded := false
                            for _, messageBefore := range messagesToDeleteBefore {
                                if messageToDelete.ID == messageBefore.ID {
                                    isExcluded = true
                                }
                            }
                            if isExcluded == false {
                                messagesToDeleteIds = append(messagesToDeleteIds, messageToDelete.ID)
                            }
                        }
                        if len(messagesToDeleteIds) <= 10 {
                            err := session.ChannelMessagesBulkDelete(msg.ChannelID, messagesToDeleteIds)
                            logger.PLUGIN.L("mod", fmt.Sprintf("Deleted %d messages (command issued by %s (#%s))", len(messagesToDeleteIds), msg.Author.Username, msg.Author.ID))
                            if err != nil {
                                session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf(helpers.GetTextF("plugins.mod.deleting-messages-failed"), err.Error()))
                            }
                        } else {
                            if helpers.ConfirmEmbed(msg.ChannelID, msg.Author, helpers.GetTextF("plugins.mod.deleting-message-bulkdelete-confirm", len(messagesToDeleteIds)), "✅", "🚫") == true {
                                err := session.ChannelMessagesBulkDelete(msg.ChannelID, messagesToDeleteIds)
                                logger.PLUGIN.L("mod", fmt.Sprintf("Deleted %d messages (command issued by %s (#%s))", len(messagesToDeleteIds), msg.Author.Username, msg.Author.ID))
                                if err != nil {
                                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.mod.deleting-messages-failed", err.Error()))
                                    return
                                }
                            }
                            return
                        }
                    }
                case "messages": // [p]cleanup messages <n>
                    if len(args) < 2 {
                        session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
                        return
                    } else {
                        if regexNumberOnly.MatchString(args[1]) == false {
                            session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
                            return
                        }
                        numOfMessagesToDelete, err := strconv.Atoi(args[1])
                        if err != nil {
                            session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf(helpers.GetTextF("bot.errors.general"), err.Error()))
                            return
                        }
                        if numOfMessagesToDelete < 1 {
                            session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
                            return
                        }

                        messagesToDelete, _ := session.ChannelMessages(msg.ChannelID, numOfMessagesToDelete+1, "", "")
                        messagesToDeleteIds := []string{}
                        for _, messageToDelete := range messagesToDelete {
                            messagesToDeleteIds = append(messagesToDeleteIds, messageToDelete.ID)
                        }
                        if len(messagesToDeleteIds) <= 10 {
                            err := session.ChannelMessagesBulkDelete(msg.ChannelID, messagesToDeleteIds)
                            logger.PLUGIN.L("mod", fmt.Sprintf("Deleted %d messages (command issued by %s (#%s))", len(messagesToDeleteIds), msg.Author.Username, msg.Author.ID))
                            if err != nil {
                                session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf(helpers.GetTextF("plugins.mod.deleting-messages-failed"), err.Error()))
                            }
                        } else {
                            if helpers.ConfirmEmbed(msg.ChannelID, msg.Author, helpers.GetTextF("plugins.mod.deleting-message-bulkdelete-confirm", len(messagesToDeleteIds)-1), "✅", "🚫") == true {
                                err := session.ChannelMessagesBulkDelete(msg.ChannelID, messagesToDeleteIds)
                                logger.PLUGIN.L("mod", fmt.Sprintf("Deleted %d messages (command issued by %s (#%s))", len(messagesToDeleteIds), msg.Author.Username, msg.Author.ID))
                                if err != nil {
                                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.mod.deleting-messages-failed", err.Error()))
                                    return
                                }
                            }
                            return
                        }
                    }
                }
            }
        })
    case "mute": // [p]mute server <User>
        helpers.RequireMod(msg, func() {
            args := strings.Fields(content)
            if len(args) >= 2 {
                targetUser, err := helpers.GetUserFromMention(args[1])
                if err != nil {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
                    return
                }
                switch args[0] {
                case "server":
                    channel, err := session.Channel(msg.ChannelID)
                    helpers.Relax(err)
                    muteRole, err := helpers.GetMuteRole(channel.GuildID)
                    helpers.Relax(err)
                    err = session.GuildMemberRoleAdd(channel.GuildID, targetUser.ID, muteRole.ID)
                    helpers.Relax(err)
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.mod.user-muted-success", targetUser.Username, targetUser.ID))
                }
            } else {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
                return
            }
        })
    case "unmute": // [p]unmute server <User>
        helpers.RequireMod(msg, func() {
            args := strings.Fields(content)
            if len(args) >= 2 {
                targetUser, err := helpers.GetUserFromMention(args[1])
                if err != nil {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
                    return
                }
                switch args[0] {
                case "server":
                    channel, err := session.Channel(msg.ChannelID)
                    helpers.Relax(err)
                    muteRole, err := helpers.GetMuteRole(channel.GuildID)
                    helpers.Relax(err)
                    err = session.GuildMemberRoleRemove(channel.GuildID, targetUser.ID, muteRole.ID)
                    helpers.Relax(err)
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.mod.user-unmuted-success", targetUser.Username, targetUser.ID))
                }
            } else {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
                return
            }
        })
    case "ban": // [p]ban <User> [<Days>], checks for IsMod and Ban Permissions
        helpers.RequireMod(msg, func() {
            args := strings.Fields(content)
            if len(args) >= 1 {
                // Days Argument
                days := 0
                var err error
                if len(args) >= 2 && regexNumberOnly.MatchString(args[1]) {
                    days, err = strconv.Atoi(args[1])
                    if err != nil {
                        session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
                        return
                    }
                }

                targetUser, err := helpers.GetUserFromMention(args[0])
                if err != nil {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
                    return
                }
                // Bot can ban?
                botCanBan := false
                channel, err := session.Channel(msg.ChannelID)
                helpers.Relax(err)
                guild, err := session.Guild(channel.GuildID)
                guildMemberBot, err := session.GuildMember(guild.ID, session.State.User.ID)
                helpers.Relax(err)
                for _, role := range guild.Roles {
                    for _, userRole := range guildMemberBot.Roles {
                        if userRole == role.ID && (role.Permissions&discordgo.PermissionBanMembers == discordgo.PermissionBanMembers || role.Permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator) {
                            botCanBan = true
                        }
                    }
                }
                if botCanBan == false {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.mod.bot-disallowed"))
                    return
                }
                // User can ban?
                userCanBan := false
                guildMemberUser, err := session.GuildMember(guild.ID, msg.Author.ID)
                helpers.Relax(err)
                for _, role := range guild.Roles {
                    for _, userRole := range guildMemberUser.Roles {
                        if userRole == role.ID && (role.Permissions&discordgo.PermissionBanMembers == discordgo.PermissionBanMembers || role.Permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator) {
                            userCanBan = true
                        }
                    }
                }
                if userCanBan == false {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.mod.disallowed"))
                    return
                }
                // Ban user
                err = session.GuildBanCreate(guild.ID, targetUser.ID, days)
                helpers.Relax(err)
                logger.INFO.L("mod", fmt.Sprintf("Banned User %s (#%s) on Guild %s (#%s) by %s (#%s)", targetUser.Username, targetUser.ID, guild.Name, guild.ID, msg.Author.Username, msg.Author.ID))
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.mod.user-banned-success", targetUser.Username, targetUser.ID))
            } else {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
                return
            }
        })
    case "kick": // [p]kick <User>, checks for IsMod and Kick Permissions
        helpers.RequireMod(msg, func() {
            args := strings.Fields(content)
            if len(args) >= 1 {
                targetUser, err := helpers.GetUserFromMention(args[0])
                if err != nil {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
                    return
                }
                // Bot can kick?
                botCanKick := false
                channel, err := session.Channel(msg.ChannelID)
                helpers.Relax(err)
                guild, err := session.Guild(channel.GuildID)
                guildMemberBot, err := session.GuildMember(guild.ID, session.State.User.ID)
                helpers.Relax(err)
                for _, role := range guild.Roles {
                    for _, userRole := range guildMemberBot.Roles {
                        if userRole == role.ID && (role.Permissions&discordgo.PermissionKickMembers == discordgo.PermissionKickMembers || role.Permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator) {
                            botCanKick = true
                        }
                    }
                }
                if botCanKick == false {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.mod.bot-disallowed"))
                    return
                }
                // User can kick?
                userCanKick := false
                guildMemberUser, err := session.GuildMember(guild.ID, msg.Author.ID)
                helpers.Relax(err)
                for _, role := range guild.Roles {
                    for _, userRole := range guildMemberUser.Roles {
                        if userRole == role.ID && (role.Permissions&discordgo.PermissionKickMembers == discordgo.PermissionKickMembers || role.Permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator) {
                            userCanKick = true
                        }
                    }
                }
                if userCanKick == false {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.mod.disallowed"))
                    return
                }
                // Ban user
                err = session.GuildMemberDelete(guild.ID, targetUser.ID)
                helpers.Relax(err)
                logger.INFO.L("mod", fmt.Sprintf("Kicked User %s (#%s) on Guild %s (#%s) by %s (#%s)", targetUser.Username, targetUser.ID, guild.Name, guild.ID, msg.Author.Username, msg.Author.ID))
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.mod.user-kicked-success", targetUser.Username, targetUser.ID))
            } else {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
                return
            }
        })
    case "serverlist": // [p]serverlist
        helpers.RequireBotAdmin(msg, func() {
            resultText := ""
            totalMembers := 0
            totalChannels := 0
            for _, guild := range session.State.Guilds {
                resultText += fmt.Sprintf("`%s` (`#%s`): Channels `%d`, Members: `%d`, Region: `%s`\n",
                    guild.Name, guild.ID, len(guild.Channels), guild.MemberCount, guild.Region)
                totalChannels += len(guild.Channels)
                totalMembers += guild.MemberCount
            }
            resultText += fmt.Sprintf("Total Stats: Servers `%d`, Channels: `%d`, Members: `%d`", len(session.State.Guilds), totalChannels, totalMembers)

            for _, resultPage := range helpers.Pagify(resultText, "\n") {
                _, err := session.ChannelMessageSend(msg.ChannelID, resultPage)
                helpers.Relax(err)
            }
        })
    case "echo": // [p]echo <channel> <message>
        helpers.RequireMod(msg, func() {
            args := strings.Fields(content)
            if len(args) >= 2 {
                sourceChannel, err := session.Channel(msg.ChannelID)
                helpers.Relax(err)
                targetChannel, err := helpers.GetChannelFromMention(args[0])
                if err != nil || targetChannel.ID == "" {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
                    return
                }
                if sourceChannel.GuildID != targetChannel.GuildID {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.mod.echo-error-wrong-server"))
                    return
                }
                session.ChannelMessageSend(targetChannel.ID, strings.Join(args[1:], " "))
            } else {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
                return
            }
        })
    case "inspect": // [p]inspect <user>
        helpers.RequireMod(msg, func() {
            session.ChannelTyping(msg.ChannelID)
            args := strings.Fields(content)
            var targetUser *discordgo.User
            var err error
            if len(args) >= 1 && args[0] != "" {
                targetUser, err = helpers.GetUserFromMention(args[0])
                helpers.Relax(err)
                if targetUser.ID == "" {
                    _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                    helpers.Relax(err)
                    return
                }
            } else {
                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                helpers.Relax(err)
                return
            }
            channel, err := session.Channel(msg.ChannelID)
            helpers.Relax(err)

            resultEmbed := &discordgo.MessageEmbed{
                Title:       helpers.GetTextF("plugins.mod.inspect-embed-title", targetUser.Username, targetUser.Discriminator),
                Description: helpers.GetTextF("plugins.mod.inspect-embed-description-in-progress", 0, len(session.State.Guilds)),
                URL:         helpers.GetAvatarUrl(targetUser),
                Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: helpers.GetAvatarUrl(targetUser)},
                Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.mod.inspect-embed-footer", targetUser.ID, len(session.State.Guilds))},
                Color:       0x0FADED,
            }
            resultMessage, err := session.ChannelMessageSendEmbed(msg.ChannelID, resultEmbed)

            bannedOnServerList, checkFailedServerList := m.inspectUserBans(targetUser, func(progressN int) {
                resultEmbed.Description = helpers.GetTextF("plugins.mod.inspect-embed-description-in-progress", progressN, len(session.State.Guilds))
                session.ChannelMessageEditEmbed(msg.ChannelID, resultMessage.ID, resultEmbed)
            })

            resultEmbed.Description = helpers.GetTextF("plugins.mod.inspect-embed-description-done")

            resultBansText := ""
            if len(bannedOnServerList) <= 0 {
                resultBansText += fmt.Sprintf("✅ User is banned on none servers.\n▪Checked %d servers.", len(session.State.Guilds)-len(checkFailedServerList))
            } else {
                resultBansText += fmt.Sprintf("⚠ User is banned on **%d** servers.\n▪Checked %d servers.", len(bannedOnServerList), len(session.State.Guilds)-len(checkFailedServerList))
            }

            isOnServerList := m.inspectCommonServers(targetUser)
            commonGuildsText := ""
            if len(isOnServerList)-1 > 0 { // -1 to exclude the server the user is currently on
                commonGuildsText += fmt.Sprintf("✅ User is on **%d** other servers with Robyul.", len(isOnServerList)-1)
            } else {
                commonGuildsText += "❓ User is on **none** other servers with Robyul."
            }

            joinedTime := helpers.GetTimeFromSnowflake(targetUser.ID)
            oneDayAgo := time.Now().AddDate(0, 0, -1)
            oneWeekAgo := time.Now().AddDate(0, 0, -7)
            joinedTimeText := ""
            if !joinedTime.After(oneWeekAgo) {
                joinedTimeText += fmt.Sprintf("✅ User Account is %s old.\n▪Joined at %s.", helpers.SinceInDaysText(joinedTime), joinedTime.Format(time.ANSIC))
            } else if !joinedTime.After(oneDayAgo) {
                joinedTimeText += fmt.Sprintf("❓ User Account is less than one Week old.\n▪Joined at %s.", joinedTime.Format(time.ANSIC))
            } else {
                joinedTimeText += fmt.Sprintf("⚠ User Account is less than one Day old.\n▪Joined at %s.", joinedTime.Format(time.ANSIC))
            }

            resultEmbed.Fields = []*discordgo.MessageEmbedField{
                {Name: "Bans", Value: resultBansText, Inline: false},
                {Name: "Common Servers", Value: commonGuildsText, Inline: false},
                {Name: "Account Age", Value: joinedTimeText, Inline: false},
            }

            for _, failedServer := range checkFailedServerList {
                if failedServer.ID == channel.GuildID {
                    resultEmbed.Description += "\n⚠ I wasn't able to gather the ban list for this server!\nPlease give Robyul the permission `Ban Members` to help other servers."
                    break
                }
            }

            // @TODO: check accoutn age

            _, err = session.ChannelMessageEditEmbed(msg.ChannelID, resultMessage.ID, resultEmbed)
            helpers.Relax(err)
        })
    case "auto-inspects-channel": // [p]auto-inspects-channel [<channel id>]
        helpers.RequireAdmin(msg, func() {
            channel, err := session.State.Channel(msg.ChannelID)
            helpers.Relax(err)
            settings := helpers.GuildSettingsGetCached(channel.GuildID)
            args := strings.Fields(content)
            successMessage := ""
            // Add Text
            if len(args) >= 1 {
                targetChannel, err := helpers.GetChannelFromMention(args[0])
                if err != nil || targetChannel.ID == "" {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                    return
                }

                settings.InspectsEnabled = true
                settings.InspectsChannel = targetChannel.ID

                successMessage = helpers.GetText("plugins.mod.inspects-channel-set")
            } else {
                settings.InspectsEnabled = false
                successMessage = helpers.GetText("plugins.mod.inspects-channel-disabled")
            }
            err = helpers.GuildSettingsSet(channel.GuildID, settings)
            helpers.Relax(err)
            _, err = session.ChannelMessageSend(msg.ChannelID, successMessage)
            helpers.Relax(err)
        })
    }
}

func (m *Mod) inspectUserBans(user *discordgo.User, callbackProgress func(progressN int)) ([]*discordgo.Guild, []*discordgo.Guild) {
    bannedOnServerList := make([]*discordgo.Guild, 0)
    checkFailedServerList := make([]*discordgo.Guild, 0)

    i := 1
    for _, botGuild := range cache.GetSession().State.Guilds {
        callbackProgress(i)
        guildBans, err := cache.GetSession().GuildBans(botGuild.ID)
        if err != nil {
            checkFailedServerList = append(checkFailedServerList, botGuild)
        } else {
            for _, guildBan := range guildBans {
                if guildBan.User.ID == user.ID {
                    bannedOnServerList = append(bannedOnServerList, botGuild)
                }
            }
        }
        i++
    }
    return bannedOnServerList, checkFailedServerList
}

func (m *Mod) inspectCommonServers(user *discordgo.User) []*discordgo.Guild {
    isOnServerList := make([]*discordgo.Guild, 0)
    for _, botGuild := range cache.GetSession().State.Guilds {
        _, err := cache.GetSession().GuildMember(botGuild.ID, user.ID)
        if err == nil {
            isOnServerList = append(isOnServerList, botGuild)
        }
    }
    return isOnServerList
}

func (m *Mod) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {
    go func() {
        if helpers.GuildSettingsGetCached(member.GuildID).InspectsEnabled {
            guild, err := session.State.Guild(member.GuildID)
            if err != nil {
                raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
                return
            }

            bannedOnServerList, checkFailedServerList := m.inspectUserBans(member.User, func(_ int) {})

            logger.INFO.L("mod", fmt.Sprintf("Inspected user %s (%s) because he joined Guild %s (#%s): Banned On: %d, Banned Checks Failed: %d",
                member.User.Username, member.User.ID, guild.Name, guild.ID, len(bannedOnServerList), len(checkFailedServerList)))

            if len(bannedOnServerList) > 0 {
                resultEmbed := &discordgo.MessageEmbed{
                    Title: helpers.GetTextF("plugins.mod.inspect-embed-title", member.User.Username, member.User.Discriminator),
                    Description: helpers.GetText("plugins.mod.inspect-embed-description-done") +
                        "\n_inspected because User joined this Server._",
                    URL:       helpers.GetAvatarUrl(member.User),
                    Thumbnail: &discordgo.MessageEmbedThumbnail{URL: helpers.GetAvatarUrl(member.User)},
                    Footer:    &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.mod.inspect-embed-footer", member.User.ID, len(session.State.Guilds))},
                    Color:     0x0FADED,
                }

                resultBansText := ""
                if len(bannedOnServerList) <= 0 {
                    resultBansText += fmt.Sprintf("✅ User is banned on none servers.\n▪Checked %d servers.", len(session.State.Guilds)-len(checkFailedServerList))
                } else {
                    resultBansText += fmt.Sprintf("⚠ User is banned on **%d** servers.\n▪Checked %d servers.", len(bannedOnServerList), len(session.State.Guilds)-len(checkFailedServerList))
                }

                isOnServerList := m.inspectCommonServers(member.User)
                commonGuildsText := ""
                if len(isOnServerList)-1 > 0 { // -1 to exclude the server the user is currently on
                    commonGuildsText += fmt.Sprintf("✅ User is on **%d** other servers with Robyul.", len(isOnServerList)-1)
                } else {
                    commonGuildsText += "❓ User is on **none** other servers with Robyul."
                }

                joinedTime := helpers.GetTimeFromSnowflake(member.User.ID)
                oneDayAgo := time.Now().AddDate(0, 0, -1)
                oneWeekAgo := time.Now().AddDate(0, 0, -7)
                joinedTimeText := ""
                if !joinedTime.After(oneWeekAgo) {
                    joinedTimeText += fmt.Sprintf("✅ User Account is %s old.\n▪Joined at %s.", helpers.SinceInDaysText(joinedTime), joinedTime.Format(time.ANSIC))
                } else if !joinedTime.After(oneDayAgo) {
                    joinedTimeText += fmt.Sprintf("❓ User Account is less than one Week old.\n▪Joined at %s.", joinedTime.Format(time.ANSIC))
                } else {
                    joinedTimeText += fmt.Sprintf("⚠ User Account is less than one Day old.\n▪Joined at %s.", joinedTime.Format(time.ANSIC))
                }

                resultEmbed.Fields = []*discordgo.MessageEmbedField{
                    {Name: "Bans", Value: resultBansText, Inline: false},
                    {Name: "Common Servers", Value: commonGuildsText, Inline: false},
                    {Name: "Account Age", Value: joinedTimeText, Inline: false},
                }

                for _, failedServer := range checkFailedServerList {
                    if failedServer.ID == member.GuildID {
                        resultEmbed.Description += "\n⚠ I wasn't able to gather the ban list for this server!\nPlease give Robyul the permission `Ban Members` to help other servers."
                        break
                    }
                }

                _, err := session.ChannelMessageSendEmbed(helpers.GuildSettingsGetCached(member.GuildID).InspectsChannel, resultEmbed)
                if err != nil {
                    raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
                    return
                }
            }
        }
    }()
}

func (m *Mod) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {
}

func (m *Mod) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {
}

func (m *Mod) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {
}

func (m *Mod) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {

}
func (m *Mod) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {
    go func() {
        bannedOnGuild, err := session.State.Guild(user.GuildID)
        if err != nil {
            raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
            return
        }
        for _, targetGuild := range cache.GetSession().State.Guilds {
            if targetGuild.ID != user.GuildID && helpers.GuildSettingsGetCached(targetGuild.ID).InspectsEnabled {
                _, err := cache.GetSession().GuildMember(targetGuild.ID, user.User.ID)
                // check if user is on this guild
                if err == nil {
                    guild, err := session.State.Guild(targetGuild.ID)
                    if err != nil {
                        raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
                        continue
                    }

                    bannedOnServerList, checkFailedServerList := m.inspectUserBans(user.User, func(_ int) {})

                    logger.INFO.L("mod", fmt.Sprintf("Inspected user %s (%s) because he got banned on Guild %s (#%s) for Guild %s (#%s): Banned On: %d, Banned Checks Failed: %d",
                        user.User.Username, user.User.ID, bannedOnGuild.Name, bannedOnGuild.ID, guild.Name, guild.ID, len(bannedOnServerList), len(checkFailedServerList)))

                    resultEmbed := &discordgo.MessageEmbed{
                        Title: helpers.GetTextF("plugins.mod.inspect-embed-title", user.User.Username, user.User.Discriminator),
                        Description: helpers.GetText("plugins.mod.inspect-embed-description-done") +
                            "\n_inspected because User got banned on a different Server._",
                        URL:       helpers.GetAvatarUrl(user.User),
                        Thumbnail: &discordgo.MessageEmbedThumbnail{URL: helpers.GetAvatarUrl(user.User)},
                        Footer:    &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.mod.inspect-embed-footer", user.User.ID, len(session.State.Guilds))},
                        Color:     0x0FADED,
                    }

                    resultBansText := ""
                    if len(bannedOnServerList) <= 0 {
                        resultBansText += fmt.Sprintf("✅ User is banned on none servers.\n▪Checked %d servers.", len(session.State.Guilds)-len(checkFailedServerList))
                    } else {
                        resultBansText += fmt.Sprintf("⚠ User is banned on **%d** servers.\n▪Checked %d servers.", len(bannedOnServerList), len(session.State.Guilds)-len(checkFailedServerList))
                    }

                    isOnServerList := m.inspectCommonServers(user.User)
                    commonGuildsText := ""
                    if len(isOnServerList)-1 > 0 { // -1 to exclude the server the user is currently on
                        commonGuildsText += fmt.Sprintf("✅ User is on **%d** other servers with Robyul.", len(isOnServerList)-1)
                    } else {
                        commonGuildsText += "❓ User is on **none** other servers with Robyul."
                    }

                    joinedTime := helpers.GetTimeFromSnowflake(user.User.ID)
                    oneDayAgo := time.Now().AddDate(0, 0, -1)
                    oneWeekAgo := time.Now().AddDate(0, 0, -7)
                    joinedTimeText := ""
                    if !joinedTime.After(oneWeekAgo) {
                        joinedTimeText += fmt.Sprintf("✅ User Account is %s old.\n▪Joined at %s.", helpers.SinceInDaysText(joinedTime), joinedTime.Format(time.ANSIC))
                    } else if !joinedTime.After(oneDayAgo) {
                        joinedTimeText += fmt.Sprintf("❓ User Account is less than one Week old.\n▪Joined at %s.", joinedTime.Format(time.ANSIC))
                    } else {
                        joinedTimeText += fmt.Sprintf("⚠ User Account is less than one Day old.\n▪Joined at %s.", joinedTime.Format(time.ANSIC))
                    }

                    resultEmbed.Fields = []*discordgo.MessageEmbedField{
                        {Name: "Bans", Value: resultBansText, Inline: false},
                        {Name: "Common Servers", Value: commonGuildsText, Inline: false},
                        {Name: "Account Age", Value: joinedTimeText, Inline: false},
                    }

                    for _, failedServer := range checkFailedServerList {
                        if failedServer.ID == user.GuildID {
                            resultEmbed.Description += "\n⚠ I wasn't able to gather the ban list for this server!\nPlease give Robyul the permission `Ban Members` to help other servers."
                            break
                        }
                    }

                    _, err = session.ChannelMessageSendEmbed(helpers.GuildSettingsGetCached(targetGuild.ID).InspectsChannel, resultEmbed)
                    if err != nil {
                        raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
                        continue
                    }
                }
            }
        }
    }()
}

func (m *Mod) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}
