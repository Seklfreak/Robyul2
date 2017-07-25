package plugins

import (
    "fmt"
    "github.com/Seklfreak/Robyul2/helpers"
    "github.com/bwmarrin/discordgo"
    "strings"
    "time"
    rethink "github.com/gorethink/gorethink"
    "sort"
    "encoding/json"
    "bytes"
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
                        for _, page := range helpers.Pagify(helpers.GetTextF("plugins.bias.bias-help-message",
                            biasListText, exampleRoleName, exampleRoleName), ",") {
                            session.ChannelMessageSend(msg.ChannelID, page)
                        }
                        return
                    }
                }

                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.bias.no-bias-config"))
                helpers.Relax(err)
            })
        case "refresh":
            helpers.RequireBotAdmin(msg, func() {
                session.ChannelTyping(msg.ChannelID)
                biasChannels = m.GetBiasChannels()
                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.bias.refreshed-config"))
                helpers.Relax(err)
            })
        case "set-config":
            helpers.RequireAdmin(msg, func() {
                session.ChannelTyping(msg.ChannelID)

                if len(args) < 2 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                    helpers.Relax(err)
                    return
                }

                if len(msg.Attachments) <= 0 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                    helpers.Relax(err)
                    return
                }

                targetChannel, err := helpers.GetChannelFromMention(msg, args[1])
                if err != nil || targetChannel.ID == "" {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                    return
                }

                var channelConfig []AssignableRole_Category
                channelConfigJson := helpers.NetGet(msg.Attachments[0].URL)
                err = json.Unmarshal(channelConfigJson, &channelConfig)
                helpers.Relax(err)

                channelDb := m.getChannelConfigByOrCreateEmpty("channelid", targetChannel.ID)
                channelDb.ServerID = targetChannel.GuildID
                channelDb.ChannelID = targetChannel.ID
                channelDb.Categories = channelConfig
                m.setChannelConfig(channelDb)

                biasChannels = m.GetBiasChannels()
                _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.bias.updated-config"))
                helpers.Relax(err)
                return
            })
        case "get-config":
            helpers.RequireAdmin(msg, func() {
                session.ChannelTyping(msg.ChannelID)

                if len(args) < 2 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                    helpers.Relax(err)
                    return
                }

                targetChannel, err := helpers.GetChannelFromMention(msg, args[1])
                if err != nil || targetChannel.ID == "" {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                    return
                }

                channelDb := m.getChannelConfigBy("channelid", targetChannel.ID)
                if channelDb.ChannelID == "" {
                    _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.bias.no-bias-config"))
                    helpers.Relax(err)
                    return
                }

                channelConfigJson, err := json.MarshalIndent(channelDb.Categories, "", "    ")
                helpers.Relax(err)

                _, err = session.ChannelFileSend(msg.ChannelID, targetChannel.Name + "-robyul-bias-config.json", bytes.NewReader(channelConfigJson))
                helpers.Relax(err)

                return
            })
        case "stats":
            session.ChannelTyping(msg.ChannelID)

            channel, err := helpers.GetChannel(msg.ChannelID)
            helpers.Relax(err)
            guild, err := helpers.GetGuild(channel.GuildID)
            helpers.Relax(err)

            members := make([]*discordgo.Member, 0)
            lastAfterMemberId := ""
            for {
                additionalMembers, err := session.GuildMembers(guild.ID, lastAfterMemberId, 1000)
                if len(additionalMembers) <= 0 {
                    break
                }
                lastAfterMemberId = additionalMembers[len(additionalMembers)-1].User.ID
                helpers.Relax(err)
                for _, member := range additionalMembers {
                    members = append(members, member)
                }
            }

            statsText := ""

            statsPrinted := 0
            for _, biasChannel := range biasChannels {
                if biasChannel.ServerID == channel.GuildID {
                    for _, biasCategory := range biasChannel.Categories {
                        categoryNumbers := make(BiasRoleStatList, 0)
                        if biasCategory.Hidden == true && biasCategory.Pool == "" {
                            continue
                        }
                        for _, biasRole := range biasCategory.Roles {
                            discordRole := m.GetDiscordRole(biasRole, guild)
                            if discordRole != nil {
                                categoryNumbers = append(categoryNumbers, BiasRoleStat{
                                    RoleName: discordRole.Name, Members: 0,
                                })
                                for _, member := range members {
                                    for _, memberRole := range member.Roles {
                                        if memberRole == discordRole.ID {
                                            // user has the role
                                            for i, biasRoleStat := range categoryNumbers {
                                                if biasRoleStat.RoleName == discordRole.Name {
                                                    categoryNumbers[i].Members++
                                                }
                                            }
                                        }
                                    }
                                }
                            }
                        }
                        sort.Sort(categoryNumbers)
                        if len(categoryNumbers) > 0 {
                            statsText += fmt.Sprintf("__**%s:**__\n", biasCategory.Label)
                            for _, biasRoleStat := range categoryNumbers {
                                statsText += fmt.Sprintf("**%s**: %d Members\n",
                                    biasRoleStat.RoleName, biasRoleStat.Members )
                            }
                        }
                    }
                    statsPrinted++
                }
            }

            if statsPrinted <= 0 {
                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.bias.no-stats"))
                helpers.Relax(err)
            } else {
                for _, page := range helpers.Pagify(statsText, "\n") {
                    _, err := session.ChannelMessageSend(msg.ChannelID, page)
                    helpers.Relax(err)
                }
            }
        }
    }
}

func (m *Bias) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {
    go func() {
        for _, biasChannel := range biasChannels {
            if msg.ChannelID == biasChannel.ChannelID {
                channel, err := helpers.GetChannel(msg.ChannelID)
                helpers.Relax(err)
                guild, err := helpers.GetGuild(channel.GuildID)
                helpers.Relax(err)
                member, err := helpers.GetGuildMember(guild.ID, msg.Author.ID)
                helpers.Relax(err)
                guildRoles, err := session.GuildRoles(guild.ID)
                if err != nil {
                    if err, ok := err.(*discordgo.RESTError); ok && err.Message.Code == 50013 {
                        newMessage, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.bias.generic-error"))
                        helpers.Relax(err)
                        // Delete messages after ten seconds
                        time.Sleep(10 * time.Second)
                        session.ChannelMessageDelete(newMessage.ChannelID, newMessage.ID)
                        session.ChannelMessageDelete(msg.ChannelID, msg.ID)
                        return
                    } else {
                        helpers.Relax(err)
                    }
                }
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
                                        categoryRolesAssigned := m.CategoryRolesAssigned(member, guildRoles, category)
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
                                                            if poolDiscordRole != nil && poolDiscordRole.ID != "" && m.MemberHasRole(member, poolDiscordRole) {
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
                                newMessage, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.bias.generic-error"))
                                helpers.Relax(err)
                                messagesToDelete = append(messagesToDelete, newMessage)
                            } else {
                                newMessage, err := session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("<@%s> %s", msg.Author.ID, helpers.GetText("plugins.bias.role-added")))
                                helpers.Relax(err)
                                messagesToDelete = append(messagesToDelete, newMessage)
                            }
                        } else {
                            err := session.GuildMemberRoleRemove(guild.ID, msg.Author.ID, roleToAddOrDelete.DiscordRole.ID)
                            if err != nil {
                                newMessage, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.bias.generic-error"))
                                helpers.Relax(err)
                                messagesToDelete = append(messagesToDelete, newMessage)
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
    }()
}

func (m *Bias) getChannelConfigByOrCreateEmpty(key string, id string) AssignableRole_Channel {
    var entryBucket AssignableRole_Channel
    listCursor, err := rethink.Table("bias").Filter(
        rethink.Row.Field(key).Eq(id),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.One(&entryBucket)

    // If user has no DB entries create an empty document
    if err == rethink.ErrEmptyResult {
        insert := rethink.Table("bias").Insert(AssignableRole_Channel{})
        res, e := insert.RunWrite(helpers.GetDB())
        // If the creation was successful read the document
        if e != nil {
            panic(e)
        } else {
            return m.getChannelConfigByOrCreateEmpty("id", res.GeneratedKeys[0])
        }
    } else if err != nil {
        panic(err)
    }

    return entryBucket
}

func (m *Bias) getChannelConfigBy(key string, id string) AssignableRole_Channel {
    var entryBucket AssignableRole_Channel
    listCursor, err := rethink.Table("bias").Filter(
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

func (m *Bias) setChannelConfig(entry AssignableRole_Channel) {
    _, err := rethink.Table("bias").Update(entry).Run(helpers.GetDB())
    helpers.Relax(err)
}

func (m *Bias) GetBiasChannels() []AssignableRole_Channel {
    var entryBucket []AssignableRole_Channel
    cursor, err := rethink.Table("bias").Run(helpers.GetDB())
    helpers.Relax(err)

    err = cursor.All(&entryBucket)
    helpers.Relax(err)

    return entryBucket
}

func (m *Bias) CategoryRolesAssigned(member *discordgo.Member, guildRoles []*discordgo.Role, category AssignableRole_Category) []AssignableRole_Role {
    var rolesAssigned []AssignableRole_Role
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
func (m *Bias) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {

}
func (m *Bias) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}

type BiasRoleStat struct {
    RoleName string
    Members int
}

type BiasRoleStatList []BiasRoleStat

func (p BiasRoleStatList) Len() int { return len(p) }
func (p BiasRoleStatList) Less(i, j int) bool { return p[i].Members > p[j].Members }
func (p BiasRoleStatList) Swap(i, j int){ p[i], p[j] = p[j], p[i] }
