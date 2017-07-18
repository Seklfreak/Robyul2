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
    "github.com/Seklfreak/Robyul2/emojis"
    "github.com/renstrom/fuzzysearch/fuzzy"
    "github.com/Jeffail/gabs"
    "github.com/bradfitz/slice"
    "bytes"
    rethink "github.com/gorethink/gorethink"
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
        "inspect-extended",
        "auto-inspects-channel",
        "search-user",
        "audit-log",
        "invites",
        "leave-server",
        "say",
        "edit",
        "upload",
        "get",
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

                        messagesToDeleteIds := []string{}

                        nextAfterID := afterMessageId
                    AllMessagesLoop:
                        for {
                            messagesToDelete, _ := session.ChannelMessages(msg.ChannelID, 100, "", nextAfterID, "")
                            slice.Sort(messagesToDelete, func(i, j int) bool {
                                return messagesToDelete[i].Timestamp < messagesToDelete[j].Timestamp
                            })
                            for _, messageToDelete := range messagesToDelete {
                                messagesToDeleteIds = append(messagesToDeleteIds, messageToDelete.ID)
                                nextAfterID = messageToDelete.ID
                                if messageToDelete.ID == untilMessageId {
                                    break AllMessagesLoop
                                }
                            }
                            if len(messagesToDelete) <= 0 {
                                break AllMessagesLoop
                            }
                        }

                        msgAlreadyIn := false
                        for _, messageID := range messagesToDeleteIds {
                            if messageID == msg.ID {
                                msgAlreadyIn = true
                            }
                        }
                        if msgAlreadyIn == false {
                            messagesToDeleteIds = append(messagesToDeleteIds, msg.ID)
                        }

                        messagesToDeleteIds = m.removeDuplicates(messagesToDeleteIds)

                        if len(messagesToDeleteIds) <= 10 {
                            err := session.ChannelMessagesBulkDelete(msg.ChannelID, messagesToDeleteIds)
                            logger.PLUGIN.L("mod", fmt.Sprintf("Deleted %d messages (command issued by %s (#%s))", len(messagesToDeleteIds), msg.Author.Username, msg.Author.ID))
                            if err != nil {
                                if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == 50034 {
                                    if errD.Message.Code == 50034 {
                                        session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mod.deleting-messages-failed-too-old"))
                                        return
                                    } else if errD.Message.Code == 50013 {
                                        session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mod.deleting-messages-failed-too-old"))
                                        return
                                    } else {
                                        helpers.Relax(errD)
                                    }
                                } else {
                                    helpers.Relax(err)
                                }
                                return
                            }
                        } else {
                            if helpers.ConfirmEmbed(msg.ChannelID, msg.Author, helpers.GetTextF("plugins.mod.deleting-message-bulkdelete-confirm", len(messagesToDeleteIds)), "✅", "🚫") == true {
                                for i := 0; i < len(messagesToDeleteIds); i += 100 {
                                    batch := messagesToDeleteIds[i:m.Min(i+100, len(messagesToDeleteIds))]
                                    err := session.ChannelMessagesBulkDelete(msg.ChannelID, batch)
                                    logger.PLUGIN.L("mod", fmt.Sprintf("Deleted %d messages (command issued by %s (#%s))", len(batch), msg.Author.Username, msg.Author.ID))
                                    if err != nil {
                                        if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == 50034 {
                                            if errD.Message.Code == 50034 {
                                                session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mod.deleting-messages-failed-too-old"))
                                                return
                                            } else if errD.Message.Code == 50013 {
                                                session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mod.deleting-messages-failed-too-old"))
                                                return
                                            } else {
                                                helpers.Relax(errD)
                                            }
                                        } else {
                                            helpers.Relax(err)
                                        }
                                        return
                                    }
                                }
                            } else {
                                session.ChannelMessageDelete(msg.ChannelID, msg.ID)
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

                        messagesToDeleteIds := []string{}

                        messagesLeft := numOfMessagesToDelete + 1
                        lastBeforeID := ""
                        for {
                            messagesToGet := messagesLeft
                            if messagesLeft > 100 {
                                messagesToGet = 100
                            }
                            messagesLeft -= messagesToGet

                            messagesToDelete, _ := session.ChannelMessages(msg.ChannelID, messagesToGet, lastBeforeID, "", "")
                            slice.Sort(messagesToDelete, func(i, j int) bool {
                                return messagesToDelete[i].Timestamp < messagesToDelete[j].Timestamp
                            })
                            for _, messageToDelete := range messagesToDelete {
                                messagesToDeleteIds = append(messagesToDeleteIds, messageToDelete.ID)
                                lastBeforeID = messageToDelete.ID
                            }

                            if messagesLeft <= 0 {
                                break
                            }
                        }

                        messagesToDeleteIds = m.removeDuplicates(messagesToDeleteIds)

                        if len(messagesToDeleteIds) <= 10 {
                            err := session.ChannelMessagesBulkDelete(msg.ChannelID, messagesToDeleteIds)
                            logger.PLUGIN.L("mod", fmt.Sprintf("Deleted %d messages (command issued by %s (#%s))", len(messagesToDeleteIds), msg.Author.Username, msg.Author.ID))
                            if err != nil {
                                if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == 50034 {
                                    if errD.Message.Code == 50034 {
                                        session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mod.deleting-messages-failed-too-old"))
                                        return
                                    } else if errD.Message.Code == 50013 {
                                        session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mod.deleting-messages-failed-too-old"))
                                        return
                                    } else {
                                        helpers.Relax(errD)
                                    }
                                } else {
                                    helpers.Relax(err)
                                }
                                return
                            }
                        } else {
                            if helpers.ConfirmEmbed(msg.ChannelID, msg.Author, helpers.GetTextF("plugins.mod.deleting-message-bulkdelete-confirm", len(messagesToDeleteIds)-1), "✅", "🚫") == true {
                                for i := 0; i < len(messagesToDeleteIds); i += 100 {
                                    batch := messagesToDeleteIds[i:m.Min(i+100, len(messagesToDeleteIds))]
                                    err := session.ChannelMessagesBulkDelete(msg.ChannelID, batch)
                                    logger.PLUGIN.L("mod", fmt.Sprintf("Deleted %d messages (command issued by %s (#%s))", len(batch), msg.Author.Username, msg.Author.ID))
                                    if err != nil {
                                        if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == 50034 {
                                            if errD.Message.Code == 50034 {
                                                session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mod.deleting-messages-failed-too-old"))
                                                return
                                            } else if errD.Message.Code == 50013 {
                                                session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mod.deleting-messages-failed-too-old"))
                                                return
                                            } else {
                                                helpers.Relax(errD)
                                            }
                                        } else {
                                            helpers.Relax(err)
                                        }
                                        return
                                    }
                                }
                            } else {
                                session.ChannelMessageDelete(msg.ChannelID, msg.ID)
                            }
                            return
                        }
                    }
                }
            }
        })
    case "mute": // [p]mute server <User>
        helpers.RequireMod(msg, func() {
            session.ChannelTyping(msg.ChannelID)
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
                    if err != nil {
                        if err, ok := err.(*discordgo.RESTError); ok && err.Message.Code == 50013 {
                            session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.mod.get-mute-role-no-permissions"))
                            return
                        } else {
                            helpers.Relax(err)
                        }
                    }
                    err = session.GuildMemberRoleAdd(channel.GuildID, targetUser.ID, muteRole.ID)
                    helpers.Relax(err)

                    settings := helpers.GuildSettingsGetCached(channel.GuildID)

                    alreadyMutedInSettings := false
                    for _, mutedMember := range settings.MutedMembers {
                        if mutedMember == targetUser.ID {
                            alreadyMutedInSettings = true
                        }
                    }
                    if alreadyMutedInSettings == false {
                        settings.MutedMembers = append(settings.MutedMembers, targetUser.ID)
                        err = helpers.GuildSettingsSet(channel.GuildID, settings)
                        helpers.Relax(err)
                    }

                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.mod.user-muted-success", targetUser.Username, targetUser.ID))
                }
            } else {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
                return
            }
        })
    case "unmute": // [p]unmute server <User>
        helpers.RequireMod(msg, func() {
            session.ChannelTyping(msg.ChannelID)
            args := strings.Fields(content)
            if len(args) >= 2 {
                targetUser, _ := helpers.GetUserFromMention(args[1])
                if targetUser == nil {
                    targetUser = new(discordgo.User)
                    targetUser.ID = args[1]
                    targetUser.Username = "N/A"
                }
                switch args[0] {
                case "server", "global":
                    channel, err := session.Channel(msg.ChannelID)
                    helpers.Relax(err)
                    muteRole, err := helpers.GetMuteRole(channel.GuildID)
                    helpers.Relax(err)
                    err = session.GuildMemberRoleRemove(channel.GuildID, targetUser.ID, muteRole.ID)
                    roleRemoved := true
                    if err != nil {
                        roleRemoved = false
                        if err, ok := err.(*discordgo.RESTError); ok && err.Message.Code != 0 {
                            if err.Message.Code != 10007 && err.Message.Code != 10013 {
                                helpers.Relax(err)
                            }
                        } else if err.Response.StatusCode != 400 {
                            helpers.Relax(err)
                        }
                    }

                    settings := helpers.GuildSettingsGetCached(channel.GuildID)

                    removedFromDb := false
                    newMutedMembers := make([]string, 0)
                    for _, mutedMember := range settings.MutedMembers {
                        if mutedMember != targetUser.ID {
                            newMutedMembers = append(newMutedMembers, mutedMember)
                        } else {
                            removedFromDb = true
                        }
                    }

                    if removedFromDb {
                        settings.MutedMembers = newMutedMembers
                        err = helpers.GuildSettingsSet(channel.GuildID, settings)
                        helpers.Relax(err)
                    }

                    if !removedFromDb && !roleRemoved {
                        session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mod.user-unmuted-error"))
                    } else {
                        session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.mod.user-unmuted-success", targetUser.Username, targetUser.ID))
                    }
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
                if msg.Author.ID == guild.OwnerID {
                    userCanBan = true
                }
                if userCanBan == false {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.mod.disallowed"))
                    return
                }
                // Ban user
                err = session.GuildBanCreate(guild.ID, targetUser.ID, days)
                if err != nil {
                    if err, ok := err.(*discordgo.RESTError); ok && err.Message != nil {
                        if err.Message.Code == 0 {
                            session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.mod.user-banned-failed-too-low"))
                            return
                        } else {
                            helpers.Relax(err)
                        }
                    } else {
                        helpers.Relax(err)
                    }
                }
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
                if msg.Author.ID == guild.OwnerID {
                    userCanKick = true
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
    case "echo", "say": // [p]echo <channel> <message>
        helpers.RequireMod(msg, func() {
            args := strings.Fields(content)
            if len(args) >= 2 {
                sourceChannel, err := session.Channel(msg.ChannelID)
                helpers.Relax(err)
                targetChannel, err := helpers.GetChannelFromMention(msg, args[0])
                if err != nil || targetChannel.ID == "" {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                    return
                }
                if sourceChannel.GuildID != targetChannel.GuildID {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mod.echo-error-wrong-server"))
                    return
                }

                newText := strings.TrimSpace(strings.Replace(content, strings.Join(args[:1], " "), "", 1))
                session.ChannelMessageSend(targetChannel.ID, newText)
            } else {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                return
            }
        })
    case "edit": // [p]edit <channel> <message id> <message>
        helpers.RequireAdmin(msg, func() {
            args := strings.Fields(content)
            if len(args) >= 3 {
                sourceChannel, err := session.Channel(msg.ChannelID)
                helpers.Relax(err)
                targetChannel, err := helpers.GetChannelFromMention(msg, args[0])
                if err != nil || targetChannel.ID == "" {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                    return
                }
                if sourceChannel.GuildID != targetChannel.GuildID {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mod.echo-error-wrong-server"))
                    return
                }
                targetMessage, err := session.ChannelMessage(targetChannel.ID, args[1])
                if err != nil {
                    if err, ok := err.(*discordgo.RESTError); ok && err.Message.Code == 10008 {
                        session.ChannelMessageSend(sourceChannel.ID, helpers.GetText("plugins.mod.edit-error-not-found"))
                        return
                    } else {
                        helpers.Relax(err)
                    }
                }
                newText := strings.TrimSpace(strings.Replace(content, strings.Join(args[:2], " "), "", 1))
                session.ChannelMessageEdit(targetChannel.ID, targetMessage.ID, newText)
            } else {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
                return
            }
        })
    case "upload": // [p]upload <channel> + UPLOAD
        helpers.RequireMod(msg, func() {
            args := strings.Fields(content)
            if len(args) >= 1 && len(msg.Attachments) > 0 {
                fileToUpload := helpers.NetGet(msg.Attachments[0].URL)
                sourceChannel, err := session.Channel(msg.ChannelID)
                helpers.Relax(err)
                targetChannel, err := helpers.GetChannelFromMention(msg, args[0])
                if err != nil || targetChannel.ID == "" {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                    return
                }
                if sourceChannel.GuildID != targetChannel.GuildID {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mod.echo-error-wrong-server"))
                    return
                }
                session.ChannelFileSend(targetChannel.ID, msg.Attachments[0].Filename, bytes.NewReader(fileToUpload))
            } else {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                return
            }
        })
    case "get": // [p]get <channel> <message id>
        helpers.RequireMod(msg, func() {
            args := strings.Fields(content)
            if len(args) >= 2 {
                sourceChannel, err := session.Channel(msg.ChannelID)
                helpers.Relax(err)
                targetChannel, err := helpers.GetChannelFromMention(msg, args[0])
                if err != nil || targetChannel.ID == "" {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                    return
                }
                if sourceChannel.GuildID != targetChannel.GuildID {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mod.echo-error-wrong-server"))
                    return
                }
                targetMessage, err := session.ChannelMessage(targetChannel.ID, args[1])
                if err != nil {
                    if err, ok := err.(*discordgo.RESTError); ok && err.Message.Code == 10008 {
                        session.ChannelMessageSend(sourceChannel.ID, helpers.GetText("plugins.mod.edit-error-not-found"))
                        return
                    } else {
                        helpers.Relax(err)
                    }
                }
                newMessage := fmt.Sprintf("```%s```", targetMessage.Content)
                _, err = session.ChannelMessageSend(msg.ChannelID, newMessage)
                helpers.Relax(err)
            } else {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
                return
            }
        })
    case "inspect", "inspect-extended": // [p]inspect[-extended] <user>
        helpers.RequireMod(msg, func() {
            isExtendedInspect := false
            if command == "inspect-extended" {
                if helpers.IsBotAdmin(msg.Author.ID) {
                    isExtendedInspect = true
                }
                if helpers.IsNukeMod(msg.Author.ID) {
                    isExtendedInspect = true
                }
                if isExtendedInspect == false {
                    _, err := session.ChannelMessageSend(msg.ChannelID, "You aren't allowed to do this!")
                    helpers.Relax(err)
                    return
                }
            }
            session.ChannelTyping(msg.ChannelID)
            args := strings.Fields(content)
            var targetUser *discordgo.User
            var err error
            if len(args) >= 1 && args[0] != "" {
                targetUser, err = helpers.GetUserFromMention(args[0])
                if err != nil {
                    if err, ok := err.(*discordgo.RESTError); ok && err.Message.Code == 10013 {
                        _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mod.user-not-found"))
                        helpers.Relax(err)
                        return
                    } else {
                        helpers.Relax(err)
                    }
                }
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
            textVersion := false
            if len(args) >= 2 && args[1] == "text" {
                textVersion = true
            }
            channel, err := session.Channel(msg.ChannelID)
            helpers.Relax(err)

            resultEmbed := &discordgo.MessageEmbed{
                Title:       helpers.GetTextF("plugins.mod.inspect-embed-title", targetUser.Username, targetUser.Discriminator),
                Description: helpers.GetTextF("plugins.mod.inspect-in-progress", 0, len(session.State.Guilds)),
                URL:         helpers.GetAvatarUrl(targetUser),
                Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: helpers.GetAvatarUrl(targetUser)},
                Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.mod.inspect-embed-footer", targetUser.ID, len(session.State.Guilds))},
                Color:       0x0FADED,
            }
            resultMessage := new(discordgo.Message)
            if textVersion == false {
                resultMessage, err = session.ChannelMessageSendEmbed(msg.ChannelID, resultEmbed)
                helpers.Relax(err)
            } else {
                resultMessage, err = session.ChannelMessageSend(msg.ChannelID,
                    helpers.GetTextF("plugins.mod.inspect-in-progress", 0, len(session.State.Guilds)))
                helpers.Relax(err)
            }

            bannedOnServerList, checkFailedServerList := m.inspectUserBans(targetUser, func(progressN int) {
                progressText := helpers.GetTextF("plugins.mod.inspect-in-progress", progressN, len(session.State.Guilds))
                if textVersion == false {
                    resultEmbed.Description = progressText
                    session.ChannelMessageEditEmbed(msg.ChannelID, resultMessage.ID, resultEmbed)
                } else {
                    session.ChannelMessageEdit(msg.ChannelID, resultMessage.ID, progressText)
                }
            })

            resultEmbed.Description = helpers.GetTextF("plugins.mod.inspect-description-done", targetUser.ID)
            resultText := helpers.GetTextF("plugins.mod.inspect-description-done", targetUser.ID)
            resultText += fmt.Sprintf("Username: `%s#%s,` ID: `#%s`",
                targetUser.Username, targetUser.Discriminator, targetUser.ID)
            if helpers.GetAvatarUrl(targetUser) != "" {
                resultText += fmt.Sprintf(", DP: <%s>", helpers.GetAvatarUrl(targetUser))
            }
            resultText += "\n"

            resultBansText := ""
            if len(bannedOnServerList) <= 0 {
                resultBansText += fmt.Sprintf("✅ User is banned on none servers.\n◾Checked %d servers.\n", len(session.State.Guilds)-len(checkFailedServerList))
            } else {
                if isExtendedInspect == false {
                    resultBansText += fmt.Sprintf("⚠ User is banned on **%d** servers.\n◾Checked %d servers.\n", len(bannedOnServerList), len(session.State.Guilds)-len(checkFailedServerList))
                } else {
                    resultBansText += fmt.Sprintf("⚠ User is banned on **%d** servers:\n", len(bannedOnServerList))
                    i := 0
                    BannedOnLoop:
                    for _, bannedOnServer := range bannedOnServerList {
                        resultBansText += fmt.Sprintf("▪`%s` (#%s)\n", bannedOnServer.Name, bannedOnServer.ID)
                        i++
                        if i >= 4 && textVersion == false {
                            resultBansText += fmt.Sprintf("▪ and %d other server(s)\n", len(bannedOnServerList)-(i+1))
                            break BannedOnLoop
                        }
                    }
                    resultBansText += fmt.Sprintf("◾Checked %d servers.\n", len(session.State.Guilds)-len(checkFailedServerList))
                }
            }

            isOnServerList := m.inspectCommonServers(targetUser)
            commonGuildsText := ""
            if len(isOnServerList) > 0 { // -1 to exclude the server the user is currently on
                if isExtendedInspect == false {
                    commonGuildsText += fmt.Sprintf("✅ User is on **%d** server(s) with Robyul.\n", len(isOnServerList))
                } else {
                    commonGuildsText += fmt.Sprintf("✅ User is on **%d** server(s) with Robyul:\n", len(isOnServerList))
                    i := 0
                    ServerListLoop:
                    for _, isOnServer := range isOnServerList {
                        commonGuildsText += fmt.Sprintf("▪`%s` (#%s)\n", isOnServer.Name, isOnServer.ID)
                        i++
                        if i >= 4 && textVersion == false {
                            commonGuildsText += fmt.Sprintf("▪ and %d other server(s)\n", len(isOnServerList)-(i))
                            break ServerListLoop
                        }
                    }
                }
            } else {
                commonGuildsText += "❓ User is on **none** other servers with Robyul.\n"
            }

            joinedTime := helpers.GetTimeFromSnowflake(targetUser.ID)
            oneDayAgo := time.Now().AddDate(0, 0, -1)
            oneWeekAgo := time.Now().AddDate(0, 0, -7)
            joinedTimeText := ""
            if !joinedTime.After(oneWeekAgo) {
                joinedTimeText += fmt.Sprintf("✅ User Account got created %s.\n◾Joined at %s.\n", helpers.SinceInDaysText(joinedTime), joinedTime.Format(time.ANSIC))
            } else if !joinedTime.After(oneDayAgo) {
                joinedTimeText += fmt.Sprintf("❓ User Account is less than one Week old.\n◾Joined at %s.\n", joinedTime.Format(time.ANSIC))
            } else {
                joinedTimeText += fmt.Sprintf("⚠ User Account is less than one Day old.\n◾Joined at %s.\n", joinedTime.Format(time.ANSIC))
            }

            troublemakerReports := m.getTroublemakerReports(targetUser)
            troublemakerReportsText := ""
            if len(troublemakerReports) <= 0 {
                troublemakerReportsText = "✅ User never got reported\n"
            } else {
                troublemakerReportsText = fmt.Sprintf("⚠ User got reported %d time(s)\nUse `_troublemaker list %s` to view the details.\n", len(troublemakerReports), targetUser.ID)
            }

            resultEmbed.Fields = []*discordgo.MessageEmbedField{
                {Name: "Bans", Value: resultBansText, Inline: false},
                {Name: "Troublemaker Reports", Value: troublemakerReportsText, Inline: false},
                {Name: "Common Servers", Value: commonGuildsText, Inline: false},
                {Name: "Account Age", Value: joinedTimeText, Inline: false},
            }
            resultText += resultBansText
            resultText += troublemakerReportsText
            resultText += commonGuildsText
            resultText += joinedTimeText

            for _, failedServer := range checkFailedServerList {
                if failedServer.ID == channel.GuildID {
                    noAccessToBansText := "\n⚠ I wasn't able to gather the ban list for this server!\nPlease give Robyul the permission `Ban Members` to help other servers.\n"
                    resultEmbed.Description += noAccessToBansText
                    resultText += noAccessToBansText
                    break
                }
            }

            if textVersion == false {
                _, err = session.ChannelMessageEditEmbed(msg.ChannelID, resultMessage.ID, resultEmbed)
                helpers.Relax(err)
            } else {
                pages := helpers.Pagify(resultText, "\n")
                if len(pages) <= 1 {
                    _, err = session.ChannelMessageEdit(msg.ChannelID, resultMessage.ID, resultText)
                    helpers.Relax(err)
                } else {
                    session.ChannelMessageDelete(msg.ChannelID, resultMessage.ID)
                    session.ChannelMessageEdit(msg.ChannelID, resultMessage.ID, "Inspect completed.")
                    for _, page := range pages {
                        _, err = session.ChannelMessageEdit(msg.ChannelID, resultMessage.ID, page)
                        helpers.Relax(err)
                    }
                }
            }
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
                targetChannel, err := helpers.GetChannelFromMention(msg, args[0])
                if err != nil || targetChannel.ID == "" {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                    return
                }

                chooseEmbed := &discordgo.MessageEmbed{
                    Title:       fmt.Sprintf("@%s Enable Auto Inspect Triggers", msg.Author.Username),
                    Description: "**Please wait a second...** :construction_site:",
                    Footer:      &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Robyul is currently on %d servers.", len(session.State.Guilds))},
                    Color:       0x0FADED,
                }
                chooseMessage, err := session.ChannelMessageSendEmbed(msg.ChannelID, chooseEmbed)

                allowedEmotes := []string{emojis.From("1"), emojis.From("2"), emojis.From("3"), emojis.From("4"), "💾"}
                for _, allowedEmote := range allowedEmotes {
                    err = session.MessageReactionAdd(msg.ChannelID, chooseMessage.ID, allowedEmote)
                    helpers.Relax(err)
                }

                needEmbedUpdate := true
                emotesLocked := false

                // @TODO: use reaction event, see stats.go
            HandleChooseReactions:
                for {
                    saveAndExits, _ := cache.GetSession().MessageReactions(msg.ChannelID, chooseMessage.ID, "💾", 100)
                    for _, saveAndExit := range saveAndExits {
                        if saveAndExit.ID == msg.Author.ID {
                            // user wants to exit
                            session.MessageReactionRemove(msg.ChannelID, chooseMessage.ID, "💾", msg.Author.ID)
                            break HandleChooseReactions
                        }
                    }
                    numberOnes, _ := cache.GetSession().MessageReactions(msg.ChannelID, chooseMessage.ID, emojis.From("1"), 100)
                    for _, numberOne := range numberOnes {
                        if numberOne.ID == msg.Author.ID {
                            if settings.InspectTriggersEnabled.UserBannedOnOtherServers && emotesLocked == false {
                                settings.InspectTriggersEnabled.UserBannedOnOtherServers = false
                            } else {
                                settings.InspectTriggersEnabled.UserBannedOnOtherServers = true
                            }
                            needEmbedUpdate = true
                            err := session.MessageReactionRemove(msg.ChannelID, chooseMessage.ID, emojis.From("1"), msg.Author.ID)
                            if err != nil {
                                emotesLocked = true
                            }
                        }
                    }
                    numberTwos, _ := cache.GetSession().MessageReactions(msg.ChannelID, chooseMessage.ID, emojis.From("2"), 100)
                    for _, numberTwo := range numberTwos {
                        if numberTwo.ID == msg.Author.ID {
                            if settings.InspectTriggersEnabled.UserNoCommonServers && emotesLocked == false {
                                settings.InspectTriggersEnabled.UserNoCommonServers = false
                            } else {
                                settings.InspectTriggersEnabled.UserNoCommonServers = true
                            }
                            needEmbedUpdate = true
                            err := session.MessageReactionRemove(msg.ChannelID, chooseMessage.ID, emojis.From("2"), msg.Author.ID)
                            if err != nil {
                                emotesLocked = true
                            }
                        }
                    }
                    NumberThrees, _ := cache.GetSession().MessageReactions(msg.ChannelID, chooseMessage.ID, emojis.From("3"), 100)
                    for _, NumberThree := range NumberThrees {
                        if NumberThree.ID == msg.Author.ID {
                            if settings.InspectTriggersEnabled.UserNewlyCreatedAccount && emotesLocked == false {
                                settings.InspectTriggersEnabled.UserNewlyCreatedAccount = false
                            } else {
                                settings.InspectTriggersEnabled.UserNewlyCreatedAccount = true
                            }
                            needEmbedUpdate = true
                            err := session.MessageReactionRemove(msg.ChannelID, chooseMessage.ID, emojis.From("3"), msg.Author.ID)
                            if err != nil {
                                emotesLocked = true
                            }
                        }
                    }
                    NumberFours, _ := cache.GetSession().MessageReactions(msg.ChannelID, chooseMessage.ID, emojis.From("4"), 100)
                    for _, NumberFour := range NumberFours {
                        if NumberFour.ID == msg.Author.ID {
                            if settings.InspectTriggersEnabled.UserReported && emotesLocked == false {
                                settings.InspectTriggersEnabled.UserReported = false
                            } else {
                                settings.InspectTriggersEnabled.UserReported = true
                            }
                            needEmbedUpdate = true
                            err := session.MessageReactionRemove(msg.ChannelID, chooseMessage.ID, emojis.From("4"), msg.Author.ID)
                            if err != nil {
                                emotesLocked = true
                            }
                        }
                    }

                    if needEmbedUpdate == true {
                        chooseEmbed.Description = fmt.Sprintf(
                            "Choose which warnings should trigger an automatic inspect post in <#%s>.\n"+
                                "**Available Triggers**\n",
                            targetChannel.ID)
                        enabledEmote := "🔲"
                        if settings.InspectTriggersEnabled.UserBannedOnOtherServers {
                            enabledEmote = "✔"
                        }
                        chooseEmbed.Description += fmt.Sprintf("%s %s User is banned on a different server with Robyul on. Gets checked everytime an user joins or gets banned on a different server with Robyul on.\n",
                            emojis.From("1"), enabledEmote)
                        enabledEmote = "🔲"
                        if settings.InspectTriggersEnabled.UserNoCommonServers {
                            enabledEmote = "✔"
                        }
                        chooseEmbed.Description += fmt.Sprintf("%s %s User has none other common servers with Robyul. Gets checked everytime an user joins.\n",
                            emojis.From("2"), enabledEmote)
                        enabledEmote = "🔲"
                        if settings.InspectTriggersEnabled.UserNewlyCreatedAccount {
                            enabledEmote = "✔"
                        }
                        chooseEmbed.Description += fmt.Sprintf("%s %s Account is less than one week old. Gets checked everytime an user joins.\n",
                            emojis.From("3"), enabledEmote)
                        enabledEmote = "🔲"
                        if settings.InspectTriggersEnabled.UserReported {
                            enabledEmote = "✔"
                        }
                        chooseEmbed.Description += fmt.Sprintf("%s %s Account got reported as a troublemaker. Gets checked everytime an user joins.\n",
                            emojis.From("4"), enabledEmote)
                        if emotesLocked == true {
                            chooseEmbed.Description += fmt.Sprintf("⚠ Please give Robyul the `Manage Messages` permission to be able to disable triggers or disable all triggers using `%sauto-inspects-channel`.\n",
                                helpers.GetPrefixForServer(channel.GuildID),
                            )
                        }
                        chooseEmbed.Description += "Use 💾 to save and exit."
                        chooseMessage, err = session.ChannelMessageEditEmbed(msg.ChannelID, chooseMessage.ID, chooseEmbed)
                        helpers.Relax(err)
                        needEmbedUpdate = false
                    }

                    time.Sleep(1 * time.Second)
                }

                for _, allowedEmote := range allowedEmotes {
                    session.MessageReactionRemove(msg.ChannelID, chooseMessage.ID, allowedEmote, session.State.User.ID)
                }
                settings.InspectsChannel = targetChannel.ID

                chooseEmbed.Description = strings.Replace(chooseEmbed.Description, "Use 💾 to save and exit.", "Saved.", -1)
                session.ChannelMessageEditEmbed(msg.ChannelID, chooseMessage.ID, chooseEmbed)

                successMessage = helpers.GetText("plugins.mod.inspects-channel-set")
            } else {
                settings.InspectTriggersEnabled.UserBannedOnOtherServers = false
                settings.InspectTriggersEnabled.UserNoCommonServers = false
                settings.InspectTriggersEnabled.UserNewlyCreatedAccount = false
                successMessage = helpers.GetText("plugins.mod.inspects-channel-disabled")
            }
            err = helpers.GuildSettingsSet(channel.GuildID, settings)
            helpers.Relax(err)
            _, err = session.ChannelMessageSend(msg.ChannelID, successMessage)
            helpers.Relax(err)
        })
    case "search-user": // [p]search-user <name>
        helpers.RequireMod(msg, func() {
            searchText := strings.TrimSpace(content)
            if len(searchText) > 3 {
                globalCheck := helpers.IsBotAdmin(msg.Author.ID)
                if globalCheck == true {
                    session.ChannelMessageSend(msg.ChannelID, "Searching for users on all servers with Robyul. 💬")
                } else {
                    session.ChannelMessageSend(msg.ChannelID, "Searching for users on this server. 💬")
                }

                currentChannel, err := session.State.Channel(msg.ChannelID)
                helpers.Relax(err)

                usersMatched := make([]*discordgo.User, 0)
                for _, serverGuild := range session.State.Guilds {
                    if globalCheck == true || serverGuild.ID == currentChannel.GuildID {
                        lastAfterMemberId := ""
                        for {
                            members, err := session.GuildMembers(serverGuild.ID, lastAfterMemberId, 1000)
                            if len(members) <= 0 {
                                break
                            }
                            lastAfterMemberId = members[len(members)-1].User.ID
                            helpers.Relax(err)
                            for _, serverMember := range members {
                                fullUserNameToSearch := serverMember.User.Username + "#" + serverMember.User.Discriminator + " ~ " + serverMember.Nick + " ~ " + serverMember.User.ID
                                if fuzzy.MatchFold(searchText, fullUserNameToSearch) {
                                    userIsAlreadyInList := false
                                    for _, userAlreadyInList := range usersMatched {
                                        if userAlreadyInList.ID == serverMember.User.ID {
                                            userIsAlreadyInList = true
                                        }
                                    }
                                    if userIsAlreadyInList == false {
                                        usersMatched = append(usersMatched, serverMember.User)
                                    }
                                }
                            }
                        }
                    }
                }

                if len(usersMatched) <= 0 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, "Found no user who matches your search text. 🕵")
                    helpers.Relax(err)
                    return
                } else {
                    resultText := fmt.Sprintf("Found %d users which matches your search text:\n", len(usersMatched))
                    for _, userMatched := range usersMatched {
                        resultText += fmt.Sprintf("`%s#%s` (User ID: `%s`)\n", userMatched.Username, userMatched.Discriminator, userMatched.ID)
                    }
                    for _, page := range helpers.Pagify(resultText, "\n") {
                        _, err := session.ChannelMessageSend(msg.ChannelID, page)
                        helpers.Relax(err)
                    }
                    return
                }
            } else {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
                return
            }
        })
    case "audit-log":
        helpers.RequireBotAdmin(msg, func() {
            session.ChannelTyping(msg.ChannelID)
            channel, err := session.State.Channel(msg.ChannelID)
            helpers.Relax(err)
            auditLogUrl := fmt.Sprintf(discordgo.EndpointAPI+"guilds/%s/audit-logs?limit=10", channel.GuildID)
            resp, err := session.Request("GET", auditLogUrl, nil)
            helpers.Relax(err)
            parsedResult, err := gabs.ParseJSON(resp)
            helpers.Relax(err)

            logMessage := ""

            var user *discordgo.Member
            var target *discordgo.Member

            logEntries, err := parsedResult.Path("audit_log_entries").Children()
            for _, logEntry := range logEntries {
                user, err = session.State.Member(channel.GuildID, strings.Replace(logEntry.Path("user_id").String(), "\"", "", -1))
                if err != nil {
                    user = new(discordgo.Member)
                    user.User = new(discordgo.User)
                    user.User.Username = "N/A"
                }
                target, err = session.State.Member(channel.GuildID, strings.Replace(logEntry.Path("target_id").String(), "\"", "", -1))
                if err != nil {
                    target = new(discordgo.Member)
                    target.User = new(discordgo.User)
                    target.User.Username = "N/A"
                }
                logMessage += "**Action** `" + logEntry.Path("action_type").String() + "` from `" + user.User.Username + "` for `" + target.User.Username + "`:\n"
                logEntryChanges, err := logEntry.Path("changes").Children()
                helpers.Relax(err)
                for _, logEntryChange := range logEntryChanges {
                    logMessage += logEntryChange.Path("key").String() + ": " + logEntryChange.Path("new_value").String() + "\n"
                }
            }

            for _, page := range helpers.Pagify(logMessage, "\n") {
                _, err := session.ChannelMessageSend(msg.ChannelID, page)
                helpers.Relax(err)
            }
        })
    case "invites":
        helpers.RequireBotAdmin(msg, func() {
            session.ChannelTyping(msg.ChannelID)
            channel, err := session.State.Channel(msg.ChannelID)
            helpers.Relax(err)
            invitesUrl := fmt.Sprintf(discordgo.EndpointAPI+"guilds/%s/invites", channel.GuildID)
            resp, err := session.Request("GET", invitesUrl, nil)
            helpers.Relax(err)
            parsedResult, err := gabs.ParseJSON(resp)
            helpers.Relax(err)

            invites, err := parsedResult.Children()
            helpers.Relax(err)

            inviteMessage := ""
            for _, invite := range invites {
                inviteCode := strings.Trim(invite.Path("code").String(), "\"")
                inviteInviter, err := session.State.Member(channel.GuildID, strings.Trim(invite.Path("inviter.id").String(), "\""))
                if err != nil {
                    inviteInviter = new(discordgo.Member)
                    inviteInviter.User = new(discordgo.User)
                    inviteInviter.User.Username = "N/A"
                }
                inviteUses := invite.Path("uses").String()
                inviteChannelID := strings.Trim(invite.Path("channel.id").String(), "\"")
                inviteMessage += fmt.Sprintf("`%s` by `%s` to <#%s>: **%s** uses\n",
                    inviteCode, inviteInviter.User.Username, inviteChannelID, inviteUses)
            }

            for _, page := range helpers.Pagify(inviteMessage, "\n") {
                _, err := session.ChannelMessageSend(msg.ChannelID, page)
                helpers.Relax(err)
            }
        })
    case "leave-server":
        helpers.RequireBotAdmin(msg, func() {
            session.ChannelTyping(msg.ChannelID)
            args := strings.Fields(content)
            if len(args) < 1 {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                return
            }

            targetGuild, err := session.Guild(args[0])
            helpers.Relax(err)

            if helpers.ConfirmEmbed(msg.ChannelID, msg.Author,
                fmt.Sprintf("Are you sure you want me to leave the server `%s` (`#%s`)?",
                    targetGuild.Name, targetGuild.ID), "✅", "🚫") {
                session.ChannelMessageSend(msg.ChannelID, "Goodbye <:blobwave:317048219098021888>")
                err = session.GuildLeave(targetGuild.ID)
                helpers.Relax(err)
            }
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
                    logger.INFO.L("mod", fmt.Sprintf("user %s (%s) is banned on Guild %s (#%s)",
                        user.Username, user.ID, botGuild.Name, botGuild.ID))
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

func (m *Mod) getTroublemakerReports(user *discordgo.User) []DB_Troublemaker_Entry {
    var entryBucket []DB_Troublemaker_Entry
    listCursor, err := rethink.Table("troublemakerlog").Filter(
        rethink.Row.Field("userid").Eq(user.ID),
    ).Run(helpers.GetDB())
    helpers.Relax(err)
    defer listCursor.Close()
    listCursor.All(&entryBucket)

    return entryBucket
}

func (m *Mod) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {
    go func() {
        settings := helpers.GuildSettingsGetCached(member.GuildID)

        for _, mutedMember := range settings.MutedMembers {
            if mutedMember == member.User.ID {
                muteRole, err := helpers.GetMuteRole(member.GuildID)
                if err != nil {
                    raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
                    return
                }
                err = session.GuildMemberRoleAdd(member.GuildID, member.User.ID, muteRole.ID)
                if err != nil {
                    raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
                    return
                }
            }
        }
    }()
    go func() {
        if helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserBannedOnOtherServers ||
            helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserNoCommonServers ||
            helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserNewlyCreatedAccount {
            guild, err := session.State.Guild(member.GuildID)
            if err != nil {
                raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
                return
            }

            bannedOnServerList, checkFailedServerList := m.inspectUserBans(member.User, func(_ int) {})
            troublemakerReports := m.getTroublemakerReports(member.User)

            logger.INFO.L("mod", fmt.Sprintf("Inspected user %s (%s) because he joined Guild %s (#%s): Banned On: %d, Banned Checks Failed: %d, Reports: %d",
                member.User.Username, member.User.ID, guild.Name, guild.ID, len(bannedOnServerList), len(checkFailedServerList), len(troublemakerReports)))

            isOnServerList := m.inspectCommonServers(member.User)

            joinedTime := helpers.GetTimeFromSnowflake(member.User.ID)
            oneDayAgo := time.Now().AddDate(0, 0, -1)
            oneWeekAgo := time.Now().AddDate(0, 0, -7)

            if !(
                (helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserBannedOnOtherServers && len(bannedOnServerList) > 0) ||
                    (helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserNoCommonServers && (len(isOnServerList)-1) <= 0) ||
                    (helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserNewlyCreatedAccount && joinedTime.After(oneWeekAgo)) ||
                    (helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserReported && len(troublemakerReports) > 0) ) {
                return
            }

            resultEmbed := &discordgo.MessageEmbed{
                Title: helpers.GetTextF("plugins.mod.inspect-embed-title", member.User.Username, member.User.Discriminator),
                Description: helpers.GetTextF("plugins.mod.inspect-description-done", member.User.ID) +
                    "\n_inspected because User joined this Server._",
                URL:       helpers.GetAvatarUrl(member.User),
                Thumbnail: &discordgo.MessageEmbedThumbnail{URL: helpers.GetAvatarUrl(member.User)},
                Footer:    &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.mod.inspect-embed-footer", member.User.ID, len(session.State.Guilds))},
                Color:     0x0FADED,
            }

            resultBansText := ""
            if len(bannedOnServerList) <= 0 {
                resultBansText += fmt.Sprintf("✅ User is banned on none servers.\n◾Checked %d servers.", len(session.State.Guilds)-len(checkFailedServerList))
            } else {
                resultBansText += fmt.Sprintf("⚠ User is banned on **%d** server(s).\n◾Checked %d servers.", len(bannedOnServerList), len(session.State.Guilds)-len(checkFailedServerList))
            }

            commonGuildsText := ""
            if len(isOnServerList)-1 > 0 { // -1 to exclude the server the user is currently on
                commonGuildsText += fmt.Sprintf("✅ User is on **%d** other server(s) with Robyul.", len(isOnServerList)-1)
            } else {
                commonGuildsText += "❓ User is on **none** other servers with Robyul."
            }
            joinedTimeText := ""
            if !joinedTime.After(oneWeekAgo) {
                joinedTimeText += fmt.Sprintf("✅ User Account got created %s.\n◾Joined at %s.", helpers.SinceInDaysText(joinedTime), joinedTime.Format(time.ANSIC))
            } else if !joinedTime.After(oneDayAgo) {
                joinedTimeText += fmt.Sprintf("❓ User Account is less than one Week old.\n◾Joined at %s.", joinedTime.Format(time.ANSIC))
            } else {
                joinedTimeText += fmt.Sprintf("⚠ User Account is less than one Day old.\n◾Joined at %s.", joinedTime.Format(time.ANSIC))
            }

            troublemakerReportsText := ""
            if len(troublemakerReports) <= 0 {
                troublemakerReportsText = "✅ User never got reported"
            } else {
                troublemakerReportsText = fmt.Sprintf("⚠ User got reported %d time(s)\nUse `_troublemaker list %s` to view the details.", len(troublemakerReports), member.User.ID)
            }

            resultEmbed.Fields = []*discordgo.MessageEmbedField{
                {Name: "Bans", Value: resultBansText, Inline: false},
                {Name: "Troublemaker Reports", Value: troublemakerReportsText, Inline: false},
                {Name: "Common Servers", Value: commonGuildsText, Inline: false},
                {Name: "Account Age", Value: joinedTimeText, Inline: false},
            }

            for _, failedServer := range checkFailedServerList {
                if failedServer.ID == member.GuildID {
                    resultEmbed.Description += "\n⚠ I wasn't able to gather the ban list for this server!\nPlease give Robyul the permission `Ban Members` to help other servers."
                    break
                }
            }

            _, err = session.ChannelMessageSendEmbed(helpers.GuildSettingsGetCached(member.GuildID).InspectsChannel, resultEmbed)
            if err != nil {
                raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
                return
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
        // don't post if bot can't access the ban list of the server the user got banned on
        _, err = session.GuildBans(user.GuildID)
        if err != nil {
            return
        }
        for _, targetGuild := range cache.GetSession().State.Guilds {
            if targetGuild.ID != user.GuildID && helpers.GuildSettingsGetCached(targetGuild.ID).InspectTriggersEnabled.UserBannedOnOtherServers {
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
                        Description: helpers.GetTextF("plugins.mod.inspect-description-done", user.User.ID) +
                            "\n_inspected because User got banned on a different Server._",
                        URL:       helpers.GetAvatarUrl(user.User),
                        Thumbnail: &discordgo.MessageEmbedThumbnail{URL: helpers.GetAvatarUrl(user.User)},
                        Footer:    &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.mod.inspect-embed-footer", user.User.ID, len(session.State.Guilds))},
                        Color:     0x0FADED,
                    }

                    resultBansText := ""
                    if len(bannedOnServerList) <= 0 {
                        resultBansText += fmt.Sprintf("✅ User is banned on none servers.\n◾Checked %d servers.", len(session.State.Guilds)-len(checkFailedServerList))
                    } else {
                        resultBansText += fmt.Sprintf("⚠ User is banned on **%d** server(s).\n◾Checked %d servers.", len(bannedOnServerList), len(session.State.Guilds)-len(checkFailedServerList))
                    }

                    isOnServerList := m.inspectCommonServers(user.User)
                    commonGuildsText := ""
                    if len(isOnServerList)-1 > 0 { // -1 to exclude the server the user is currently on
                        commonGuildsText += fmt.Sprintf("✅ User is on **%d** other server(s) with Robyul.", len(isOnServerList)-1)
                    } else {
                        commonGuildsText += "❓ User is on **none** other servers with Robyul."
                    }

                    joinedTime := helpers.GetTimeFromSnowflake(user.User.ID)
                    oneDayAgo := time.Now().AddDate(0, 0, -1)
                    oneWeekAgo := time.Now().AddDate(0, 0, -7)
                    joinedTimeText := ""
                    if !joinedTime.After(oneWeekAgo) {
                        joinedTimeText += fmt.Sprintf("✅ User Account got created %s.\n◾Joined at %s.", helpers.SinceInDaysText(joinedTime), joinedTime.Format(time.ANSIC))
                    } else if !joinedTime.After(oneDayAgo) {
                        joinedTimeText += fmt.Sprintf("❓ User Account is less than one Week old.\n◾Joined at %s.", joinedTime.Format(time.ANSIC))
                    } else {
                        joinedTimeText += fmt.Sprintf("⚠ User Account is less than one Day old.\n◾Joined at %s.", joinedTime.Format(time.ANSIC))
                    }

                    troublemakerReports := m.getTroublemakerReports(user.User)
                    troublemakerReportsText := ""
                    if len(troublemakerReports) <= 0 {
                        troublemakerReportsText = "✅ User never got reported"
                    } else {
                        troublemakerReportsText = fmt.Sprintf("⚠ User got reported %d time(s)\nUse `_troublemaker list %s` to view the details.", len(troublemakerReports), user.User.ID)
                    }

                    resultEmbed.Fields = []*discordgo.MessageEmbedField{
                        {Name: "Bans", Value: resultBansText, Inline: false},
                        {Name: "Troublemaker Reports", Value: troublemakerReportsText, Inline: false},
                        {Name: "Common Servers", Value: commonGuildsText, Inline: false},
                        {Name: "Account Age", Value: joinedTimeText, Inline: false},
                    }

                    for _, failedServer := range checkFailedServerList {
                        if failedServer.ID == targetGuild.ID {
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

func (m *Mod) Min(a, b int) int {
    if a <= b {
        return a
    }
    return b
}

// https://www.dotnetperls.com/duplicates-go
func (m *Mod) removeDuplicates(elements []string) []string {
    encountered := map[string]bool{}

    // Create a map of all unique elements.
    for v:= range elements {
        encountered[elements[v]] = true
    }

    // Place all keys from the map into a slice.
    result := []string{}
    for key := range encountered {
        result = append(result, key)
    }
    return result
}
