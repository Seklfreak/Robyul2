package plugins

import (
    "fmt"
    "github.com/Seklfreak/Robyul2/helpers"
    "github.com/Seklfreak/Robyul2/logger"
    "github.com/bwmarrin/discordgo"
    "regexp"
    "strconv"
    "strings"
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
    }
}

func (m *Mod) Init(session *discordgo.Session) {

}

func (m *Mod) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    regexNumberOnly := regexp.MustCompile(`^\d+$`)

    switch command {
    case "cleanup":
        helpers.RequireMod(msg, func() {
            args := strings.Split(content, " ")
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
            args := strings.Split(content, " ")
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
            args := strings.Split(content, " ")
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
            args := strings.Split(content, " ")
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
            args := strings.Split(content, " ")
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

            _, err := session.ChannelMessageSend(msg.ChannelID, resultText) // TODO: pagify
            helpers.Relax(err)
        })
    case "echo": // [p]echo <channel> <message>
        helpers.RequireMod(msg, func() {
            args := strings.Split(content, " ")
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
    }
}
