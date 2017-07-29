package plugins

import (
    "github.com/bwmarrin/discordgo"
    "strings"
    "github.com/Seklfreak/Robyul2/helpers"
    "fmt"
    "github.com/getsentry/raven-go"
)

type AutoRoles struct{}

func (a *AutoRoles) Commands() []string {
    return []string{
        "autorole",
        "autoroles",
    }
}

func (a *AutoRoles) Init(session *discordgo.Session) {
}

func (a *AutoRoles) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    args := strings.Fields(content)
    if len(args) >= 1 {
        switch args[0] {
        case "add":
            session.ChannelTyping(msg.ChannelID)
            helpers.RequireAdmin(msg, func() {
                if len(args) < 2 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                    helpers.Relax(err)
                    return
                }
                channel, err := helpers.GetChannel(msg.ChannelID)
                helpers.Relax(err)

                serverRoles, err := session.GuildRoles(channel.GuildID)
                if err != nil {
                    if errD := err.(*discordgo.RESTError); errD != nil {
                        if errD.Message.Code == 50013 {
                            _, err = session.ChannelMessageSend(msg.ChannelID, "Please give me the `Manage Roles` permission to use this feature.")
                            helpers.Relax(err)
                            return
                        } else {
                            helpers.Relax(err)
                        }
                    } else {
                        helpers.Relax(err)
                    }
                }

                roleNameToMatch := strings.TrimSpace(strings.Replace(content, strings.Join(args[:1], " "), "", 1))

                var targetRole *discordgo.Role
                for _, role := range serverRoles {
                    if strings.ToLower(role.Name) == strings.ToLower(roleNameToMatch) || role.ID == roleNameToMatch {
                        targetRole = role
                    }
                }
                if targetRole == nil || targetRole.ID == "" {
                    _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                    helpers.Relax(err)
                    return
                }

                settings := helpers.GuildSettingsGetCached(channel.GuildID)

                for _, role := range settings.AutoRoleIDs {
                    if role == targetRole.ID {
                        _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.autorole.role-add-error-duplicate"))
                        helpers.Relax(err)
                        return
                    }
                }

                settings.AutoRoleIDs = append(settings.AutoRoleIDs, targetRole.ID)

                err = helpers.GuildSettingsSet(channel.GuildID, settings)
                helpers.Relax(err)

                _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.autorole.role-add-success",
                    targetRole.Name))
                helpers.Relax(err)
                return
            })
            return
        case "list":
            session.ChannelTyping(msg.ChannelID)
            channel, err := helpers.GetChannel(msg.ChannelID)
            helpers.Relax(err)
            settings := helpers.GuildSettingsGetCached(channel.GuildID)

            if len(settings.AutoRoleIDs) <= 0 {
                _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.autorole.role-list-none"))
                helpers.Relax(err)
                return
            }

            result := "AutoRoles on this server: "

            for _, roleID := range settings.AutoRoleIDs {
                role, err := session.State.Role(channel.GuildID, roleID)
                if err == nil {
                    result += fmt.Sprintf("`%s (#%s)` ", role.Name, role.ID)
                } else {
                    result += fmt.Sprintf("`N/A (#%s)` ", roleID)
                }
            }

            result += fmt.Sprintf("(%d role(s))", len(settings.AutoRoleIDs))

            _, err = session.ChannelMessageSend(msg.ChannelID, result)
            helpers.Relax(err)
            return
        case "delete", "remove":
            session.ChannelTyping(msg.ChannelID)
            helpers.RequireAdmin(msg, func() {
                if len(args) < 2 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                    helpers.Relax(err)
                    return
                }
                channel, err := helpers.GetChannel(msg.ChannelID)
                helpers.Relax(err)

                serverRoles, err := session.GuildRoles(channel.GuildID)
                if err != nil {
                    if errD := err.(*discordgo.RESTError); errD != nil {
                        if errD.Message.Code == 50013 {
                            _, err = session.ChannelMessageSend(msg.ChannelID, "Please give me the `Manage Roles` permission to use this feature.")
                            helpers.Relax(err)
                            return
                        } else {
                            helpers.Relax(err)
                        }
                    } else {
                        helpers.Relax(err)
                    }
                }

                roleNameToMatch := strings.TrimSpace(strings.Replace(content, strings.Join(args[:1], " "), "", 1))

                var targetRole *discordgo.Role
                for _, role := range serverRoles {
                    if strings.ToLower(role.Name) == strings.ToLower(roleNameToMatch) || role.ID == roleNameToMatch {
                        targetRole = role
                    }
                }
                if targetRole == nil || targetRole.ID == "" {
                    _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                    helpers.Relax(err)
                    return
                }

                settings := helpers.GuildSettingsGetCached(channel.GuildID)

                roleWasInList := false
                newRoleIDs := make([]string, 0)

                for _, role := range settings.AutoRoleIDs {
                    if role == targetRole.ID {
                        roleWasInList = true
                    } else {
                        newRoleIDs = append(newRoleIDs, role)
                    }
                }

                if roleWasInList == false {
                    _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.autorole.role-remove-error-not-found"))
                    helpers.Relax(err)
                    return
                }

                settings.AutoRoleIDs = newRoleIDs

                err = helpers.GuildSettingsSet(channel.GuildID, settings)
                helpers.Relax(err)

                _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.autorole.role-remove-success"))
                helpers.Relax(err)
                return
            })
            return
        case "apply":
            session.ChannelTyping(msg.ChannelID)
            helpers.RequireAdmin(msg, func() {
                if len(args) < 2 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                    helpers.Relax(err)
                    return
                }
                channel, err := helpers.GetChannel(msg.ChannelID)
                helpers.Relax(err)

                serverRoles, err := session.GuildRoles(channel.GuildID)
                if err != nil {
                    if errD := err.(*discordgo.RESTError); errD != nil {
                        if errD.Message.Code == 50013 {
                            _, err = session.ChannelMessageSend(msg.ChannelID, "Please give me the `Manage Roles` permission to use this feature.")
                            helpers.Relax(err)
                            return
                        } else {
                            helpers.Relax(err)
                        }
                    } else {
                        helpers.Relax(err)
                    }
                }

                roleNameToMatch := strings.TrimSpace(strings.Replace(content, strings.Join(args[:1], " "), "", 1))

                var targetRole *discordgo.Role
                for _, role := range serverRoles {
                    if strings.ToLower(role.Name) == strings.ToLower(roleNameToMatch) || role.ID == roleNameToMatch {
                        targetRole = role
                    }
                }
                if targetRole == nil || targetRole.ID == "" {
                    _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                    helpers.Relax(err)
                    return
                }

                users := make([]string, 0)
                lastAfterMemberId := ""
                for {
                    members, err := session.GuildMembers(channel.GuildID, lastAfterMemberId, 1000)
                    helpers.Relax(err)
                    if len(members) <= 0 {
                        break
                    }

                    lastAfterMemberId = members[len(members)-1].User.ID
                    for _, u := range members {
                        users = append(users, u.User.ID)
                    }
                }

                if helpers.ConfirmEmbed(msg.ChannelID, msg.Author, helpers.GetTextF("plugins.autorole.apply-confirm",
                    targetRole.Name, targetRole.ID, len(users)),"✅", "🚫") {
                    _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.autorole.apply-started"))
                    helpers.Relax(err)

                    addedSuccess := 0
                    addedError := 0

                    for _, userID := range users {
                        err := session.GuildMemberRoleAdd(channel.GuildID, userID, targetRole.ID)
                        if err != nil {
                            addedError += 1
                        } else {
                            addedSuccess += 1
                        }
                    }

                    _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.autorole.apply-done",
                    msg.Author.ID, addedSuccess, addedError))
                    helpers.Relax(err)
                    return
                }
                return
            })
            return
        }
    }
}

func (a *AutoRoles) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {

}

func (a *AutoRoles) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {
    go func() {
        settings := helpers.GuildSettingsGetCached(member.GuildID)
        for _, roleID := range settings.AutoRoleIDs {
            err := session.GuildMemberRoleAdd(member.GuildID, member.User.ID, roleID)
            if err != nil {
                raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
                continue
            }
        }
    }()
}

func (a *AutoRoles) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {

}

func (a *AutoRoles) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {

}

func (a *AutoRoles) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {

}

func (a *AutoRoles) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {

}

func (a *AutoRoles) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}
