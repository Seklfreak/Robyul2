package plugins

import (
    "fmt"
    "github.com/Seklfreak/Robyul2/cache"
    "github.com/Seklfreak/Robyul2/helpers"
    "github.com/bwmarrin/discordgo"
    "strings"
    "time"
    rethink "github.com/gorethink/gorethink"
)

type Bias struct{}

type AssignableRole_Channel struct {
    ID         string  `gorethink:"id,omitempty"`
    ServerID   string  `gorethink:"serverid"`
    ChannelID  string  `gorethink:"channelid"`
    Categories []AssignableRole_Category  `gorethink:"categories"`
}

type AssignableRole_Category struct {
    Label   string
    Pool    string
    Hidden  bool
    Limit   int
    Roles   []AssignableRole_Role
    Message string
}

type AssignableRole_Role struct {
    Name    string
    Print   string
    Aliases []string
}

func (m *Bias) Commands() []string {
    return []string{
        "bias",
    }
}

var (
    biasChannels []AssignableRole_Channel
)

func (m *Bias) Init(session *discordgo.Session) {
    biasChannels = m.GetBiasChannels()
}

// @TODO: Make "bias role stuff text" optional
func (m *Bias) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    args := strings.Fields(content)
    if len(args) >= 1 {
        switch args[0] {
        case "help":
            helpers.RequireAdmin(msg, func() {
                session.ChannelTyping(msg.ChannelID)
                for _, biasChannel := range biasChannels {
                    if msg.ChannelID == biasChannel.ChannelID {
                        exampleRoleName := ""
                        biasListText := ""
                        for _, biasCategory := range biasChannel.Categories {
                            if biasCategory.Hidden == true {
                                continue
                            }
                            if biasCategory.Message != "" {
                                biasListText += "\n" + biasCategory.Message
                            }
                            biasListText += fmt.Sprintf("\n%s: ", biasCategory.Label)
                            for i, biasRole := range biasCategory.Roles {
                                if exampleRoleName == "" {
                                    exampleRoleName = biasRole.Print
                                }
                                if i != 0 {
                                    if i+1 < len(biasCategory.Roles) {
                                        biasListText += ", "
                                    } else {
                                        biasListText += " and "
                                    }
                                }
                                biasListText += fmt.Sprintf("**`%s`**", biasRole.Print)
                            }
                            calculatedLimit := biasCategory.Limit
                            if biasCategory.Pool != "" {
                                calculatedLimit = 0
                                for _, poolCategorie := range biasChannel.Categories {
                                    if poolCategorie.Pool == biasCategory.Pool {
                                        calculatedLimit += poolCategorie.Limit
                                    }
                                }
                            }
                            if calculatedLimit == 1 {
                                biasListText += " (**`One Role`** Max)"
                            } else if calculatedLimit > 1 {
                                biasListText += fmt.Sprintf(" (**`%s Roles`** Max)", strings.Title(helpers.HumanizeNumber(calculatedLimit)))
                            }
                        }
                        _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.bias.bias-help-message",
                            biasListText, exampleRoleName, exampleRoleName))
                        helpers.Relax(err)
                        return
                    }
                }

                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.bias.no-bias-config"))
                helpers.Relax(err)
            })
        case "refresh":
            helpers.RequireAdmin(msg, func() {
                session.ChannelTyping(msg.ChannelID)
                biasChannels = m.GetBiasChannels()
                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.bias.refreshed-config"))
                helpers.Relax(err)
            })
        case "new-config":
            helpers.RequireBotAdmin(msg, func() {
                session.ChannelTyping(msg.ChannelID)

                insert := rethink.Table("bias").Insert(AssignableRole_Channel{})
                _, err := insert.RunWrite(helpers.GetDB())
                helpers.Relax(err)

                _, err = session.ChannelMessageSend(msg.ChannelID, "Created a new entry in the Database. Please fill it manually.")
                helpers.Relax(err)
            })
        }
    }
}

func (m *Bias) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {
    for _, biasChannel := range biasChannels {
        if msg.ChannelID == biasChannel.ChannelID {
            channel, err := session.Channel(msg.ChannelID)
            helpers.Relax(err)
            guild, err := session.Guild(channel.GuildID)
            helpers.Relax(err)
            member, err := session.GuildMember(guild.ID, msg.Author.ID)
            helpers.Relax(err)
            var messagesToDelete []*discordgo.Message
            messagesToDelete = append(messagesToDelete, msg)
            var requestIsAddRole bool
            isRequest := false
            if strings.HasPrefix(content, "+") {
                requestIsAddRole = true
                isRequest = true
            } else if strings.HasPrefix(content, "-") {
                requestIsAddRole = false
                isRequest = true
            }
            if isRequest == true {
                requestedRoleName := m.CleanUpRoleName(content)
                denyReason := ""
                type Role_Information struct {
                    Role        AssignableRole_Role
                    DiscordRole *discordgo.Role
                }
                var roleToAddOrDelete Role_Information
            FindRoleLoop:
                for _, category := range biasChannel.Categories {
                TryRoleLoop:
                    for _, role := range category.Roles {
                        for _, label := range role.Aliases {
                            if strings.ToLower(label) == requestedRoleName {
                                discordRole := m.GetDiscordRole(role, guild)
                                if discordRole != nil && discordRole.ID != "" {
                                    memberHasRole := m.MemberHasRole(member, discordRole)
                                    if requestIsAddRole == true && memberHasRole == true {
                                        denyReason = helpers.GetText("plugins.bias.add-role-already")
                                        continue TryRoleLoop
                                    }
                                    if requestIsAddRole == false && memberHasRole == false {
                                        denyReason = helpers.GetText("plugins.bias.remove-role-not-found")
                                        continue TryRoleLoop
                                    }
                                    categoryRolesAssigned := m.CategoryRolesAssigned(member, guild.ID, category)
                                    if requestIsAddRole == true && (category.Limit >= 0 && len(categoryRolesAssigned) >= category.Limit) {
                                        denyReason = helpers.GetText("plugins.bias.role-limit-reached")
                                        continue TryRoleLoop
                                    }
                                    if requestIsAddRole == true && category.Pool != "" {
                                        for _, poolCategories := range biasChannel.Categories {
                                            if poolCategories.Pool == category.Pool {
                                                for _, poolRole := range poolCategories.Roles {
                                                    if poolRole.Print == role.Print {
                                                        poolDiscordRole := m.GetDiscordRole(poolRole, guild)
                                                        if poolDiscordRole.ID != "" && m.MemberHasRole(member, poolDiscordRole) {
                                                            denyReason = helpers.GetText("plugins.bias.add-role-already")
                                                            continue TryRoleLoop
                                                        }
                                                    }
                                                }
                                            }
                                        }
                                    }

                                    roleToAddOrDelete = Role_Information{Role: role, DiscordRole: discordRole}

                                    break FindRoleLoop
                                }

                            }
                        }
                    }
                }
                if roleToAddOrDelete.Role.Name != "" && roleToAddOrDelete.DiscordRole != nil {
                    if requestIsAddRole == true {
                        err := session.GuildMemberRoleAdd(guild.ID, msg.Author.ID, roleToAddOrDelete.DiscordRole.ID)
                        if err != nil {
                            session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.bias.generic-error"))
                        } else {
                            newMessage, err := session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("<@%s> %s", msg.Author.ID, helpers.GetText("plugins.bias.role-added")))
                            helpers.Relax(err)
                            messagesToDelete = append(messagesToDelete, newMessage)
                        }
                    } else {
                        err := session.GuildMemberRoleRemove(guild.ID, msg.Author.ID, roleToAddOrDelete.DiscordRole.ID)
                        if err != nil {
                            session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.bias.generic-error"))
                        } else {
                            newMessage, err := session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("<@%s> %s", msg.Author.ID, helpers.GetText("plugins.bias.role-removed")))
                            helpers.Relax(err)
                            messagesToDelete = append(messagesToDelete, newMessage)
                        }
                    }
                } else if denyReason != "" {
                    newMessage, err := session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("<@%s> %s", msg.Author.ID, denyReason))
                    helpers.Relax(err)
                    messagesToDelete = append(messagesToDelete, newMessage)
                } else {
                    newMessage, err := session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("<@%s> %s", msg.Author.ID, helpers.GetText("plugins.bias.role-not-found")))
                    helpers.Relax(err)
                    messagesToDelete = append(messagesToDelete, newMessage)
                }
            }
            // Delete messages after ten seconds
            time.Sleep(10 * time.Second)
            for _, messagsToDelete := range messagesToDelete {
                session.ChannelMessageDelete(msg.ChannelID, messagsToDelete.ID)

            }
        }
    }
}

func (m *Bias) GetBiasChannels() []AssignableRole_Channel {
    var entryBucket []AssignableRole_Channel
    cursor, err := rethink.Table("bias").Run(helpers.GetDB())
    helpers.Relax(err)

    err = cursor.All(&entryBucket)
    helpers.Relax(err)

    return entryBucket
}

func (m *Bias) CategoryRolesAssigned(member *discordgo.Member, guildID string, category AssignableRole_Category) []AssignableRole_Role {
    var rolesAssigned []AssignableRole_Role
    guildRoles, err := cache.GetSession().GuildRoles(guildID)
    helpers.Relax(err)
    for _, discordRoleId := range member.Roles {
        for _, discordGuildRole := range guildRoles {
            if discordRoleId == discordGuildRole.ID {
                for _, assignableRole := range category.Roles {
                    if strings.ToLower(assignableRole.Name) == strings.ToLower(discordGuildRole.Name) {
                        rolesAssigned = append(rolesAssigned, assignableRole)
                    }
                }
            }
        }
    }

    return rolesAssigned
}

func (m *Bias) GetDiscordRole(role AssignableRole_Role, guild *discordgo.Guild) *discordgo.Role {
    for _, discordRole := range guild.Roles {
        if strings.ToLower(role.Name) == strings.ToLower(discordRole.Name) {
            return discordRole
        }
    }
    var discordRole *discordgo.Role
    return discordRole
}

func (m *Bias) MemberHasRole(member *discordgo.Member, role *discordgo.Role) bool {
    for _, memberRole := range member.Roles {
        if memberRole == role.ID {
            return true
        }
    }
    return false
}

func (m *Bias) CleanUpRoleName(inputName string) string {
    inputName = strings.TrimPrefix(inputName, "+")
    inputName = strings.TrimPrefix(inputName, "-")
    inputName = strings.TrimSpace(inputName)
    inputName = strings.TrimPrefix(inputName, "name")
    inputName = strings.TrimSpace(inputName)
    inputName = strings.ToLower(inputName)
    return inputName
}

func (m *Bias) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {

}
func (m *Bias) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {

}
func (m *Bias) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {

}
func (m *Bias) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {

}
