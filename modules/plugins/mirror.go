package plugins

import (
    "github.com/bwmarrin/discordgo"
    "fmt"
    "github.com/Seklfreak/Robyul2/logger"
    "github.com/Seklfreak/Robyul2/helpers"
    rethink "github.com/gorethink/gorethink"
    "strings"
    "regexp"
    "github.com/Seklfreak/Robyul2/metrics"
)

type Mirror struct{}

type DB_Mirror_Entry struct {
    ID                string `gorethink:"id,omitempty"`
    ConnectedChannels []Mirror_Channel
}

type Mirror_Channel struct {
    ChannelID           string
    ChannelWebhookID    string
    ChannelWebhookToken string
    GuildID             string
}

func (m *Mirror) Commands() []string {
    return []string{
        "mirror",
    }
}

const (
    mirrorUrlRegexText string = `(<?https?:\/\/[^\s]+>?)`
)

var (
    mirrorUrlRegex *regexp.Regexp
    mirrors        []DB_Mirror_Entry
)

func (m *Mirror) Init(session *discordgo.Session) {
    mirrorUrlRegex = regexp.MustCompile(mirrorUrlRegexText)
    mirrors = m.GetMirrors()
}

func (m *Mirror) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    args := strings.Split(content, " ")
    if len(args) >= 1 {
        switch args[0] {
        case "create": // [p]mirror create
            helpers.RequireBotAdmin(msg, func() {
                channel, err := session.Channel(msg.ChannelID)
                helpers.Relax(err)
                newMirrorEntry := m.getEntryByOrCreateEmpty("id", "")
                newMirrorEntry.ConnectedChannels = make([]Mirror_Channel, 0)
                m.setEntry(newMirrorEntry)

                logger.INFO.L("galleries", fmt.Sprintf("Created new Gallery by %s (#%s)", msg.Author.Username, msg.Author.ID))
                _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.mirror.create-success",
                    helpers.GetPrefixForServer(channel.GuildID), newMirrorEntry.ID))
                helpers.Relax(err)

                mirrors = m.GetMirrors()
                return
            })
        case "add-channel": // [p]mirror add-channel <mirror id> <channel> <webhook id> <webhook token>
            // @TODO: more secure way to exchange token
            helpers.RequireBotAdmin(msg, func() {
                session.ChannelMessageDelete(msg.ChannelID, msg.ID) // Delete command message to prevent people seeing the token
                progressMessage, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mirror.add-channel-progress"))
                helpers.Relax(err)
                if len(args) < 5 {
                    _, err := session.ChannelMessageEdit(msg.ChannelID, progressMessage.ID, helpers.GetText("bot.arguments.too-few"))
                    helpers.Relax(err)
                    return
                }
                channel, err := session.Channel(msg.ChannelID)
                helpers.Relax(err)
                guild, err := session.Guild(channel.GuildID)
                helpers.Relax(err)

                mirrorID := args[1]
                mirrorEntry := m.getEntryBy("id", mirrorID)
                if mirrorEntry.ID == "" {
                    _, err := session.ChannelMessageEdit(msg.ChannelID, progressMessage.ID, helpers.GetText("bot.arguments.invalid"))
                    helpers.Relax(err)
                    return
                }

                targetChannel, err := helpers.GetChannelFromMention(args[2])
                if err != nil || targetChannel.ID == "" || targetChannel.GuildID != channel.GuildID {
                    _, err := session.ChannelMessageEdit(msg.ChannelID, progressMessage.ID, helpers.GetText("bot.arguments.invalid"))
                    helpers.Relax(err)
                    return
                }

                targetChannelWebhookId := args[3]
                targetChannelWebhookToken := args[4]

                webhook, err := session.WebhookWithToken(targetChannelWebhookId, targetChannelWebhookToken)
                if err != nil || webhook.GuildID != targetChannel.GuildID || webhook.ChannelID != targetChannel.ID {
                    _, err := session.ChannelMessageEdit(msg.ChannelID, progressMessage.ID, helpers.GetText("bot.arguments.invalid"))
                    helpers.Relax(err)
                    return
                }

                mirrorEntry.ConnectedChannels = append(mirrorEntry.ConnectedChannels, Mirror_Channel{
                    ChannelID:           targetChannel.ID,
                    GuildID:             targetChannel.GuildID,
                    ChannelWebhookID:    targetChannelWebhookId,
                    ChannelWebhookToken: targetChannelWebhookToken,
                })

                m.setEntry(mirrorEntry)

                logger.INFO.L("galleries", fmt.Sprintf("Added Channel %s (#%s) on Server %s (#%s) to Mirror %s by %s (#%s)",
                    targetChannel.Name, targetChannel.ID, guild.Name, guild.ID, mirrorEntry.ID, msg.Author.Username, msg.Author.ID))
                _, err = session.ChannelMessageEdit(msg.ChannelID, progressMessage.ID, helpers.GetText("plugins.mirror.add-channel-success"))
                helpers.Relax(err)

                mirrors = m.GetMirrors()
                return
            })
        case "list": // [p]mirror list
            helpers.RequireBotAdmin(msg, func() {
                session.ChannelTyping(msg.ChannelID)
                var entryBucket []DB_Mirror_Entry
                listCursor, err := rethink.Table("mirrors").Run(helpers.GetDB())
                helpers.Relax(err)
                defer listCursor.Close()
                err = listCursor.All(&entryBucket)

                if err == rethink.ErrEmptyResult || len(entryBucket) <= 0 {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mirror.list-empty"))
                    return
                }
                helpers.Relax(err)

                resultMessage := ":fax: Mirrors:\n"
                for _, entry := range entryBucket {
                    resultMessage += fmt.Sprintf(":satellite: Mirror `%s` (%d channels):\n", entry.ID, len(entry.ConnectedChannels))
                    for _, mirroredChannelEntry := range entry.ConnectedChannels {
                        mirroredChannel, err := session.Channel(mirroredChannelEntry.ChannelID)
                        helpers.Relax(err)
                        mirroredChannelGuild, err := session.Guild(mirroredChannelEntry.GuildID)
                        helpers.Relax(err)
                        resultMessage += fmt.Sprintf(":arrow_forward: `#%s` `(#%s)` on `%s` `(#%s)`: <#%s> (Webhook ID: `%s`)\n",
                            mirroredChannel.Name, mirroredChannel.ID,
                            mirroredChannelGuild.Name, mirroredChannelGuild.ID,
                            mirroredChannel.ID,
                            mirroredChannelEntry.ChannelWebhookID,
                        )
                    }
                }
                resultMessage += fmt.Sprintf("Found **%d** Mirrors in total.", len(entryBucket))
                for _, resultPage := range helpers.Pagify(resultMessage, "\n") {
                    _, err = session.ChannelMessageSend(msg.ChannelID, resultPage)
                    helpers.Relax(err)
                }
                return
            })
        case "delete", "del": // [p]mirror delete <mirror id>
            helpers.RequireBotAdmin(msg, func() {
                session.ChannelTyping(msg.ChannelID)
                if len(args) < 2 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                    helpers.Relax(err)
                    return
                }
                entryId := args[1]
                entryBucket := m.getEntryBy("id", entryId)
                if entryBucket.ID == "" {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mirror.delete-not-found"))
                    return
                }
                m.deleteEntryById(entryBucket.ID)

                logger.INFO.L("galleries", fmt.Sprintf("Deleted Mirror %s by %s (#%s)",
                    entryBucket.ID, msg.Author.Username, msg.Author.ID))
                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mirror.delete-success"))
                helpers.Relax(err)

                mirrors = m.GetMirrors()
                return
            })
        case "refresh": // [p]mirror refresh
            helpers.RequireBotAdmin(msg, func() {
                session.ChannelTyping(msg.ChannelID)
                mirrors = m.GetMirrors()
                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mirror.refreshed-config"))
                helpers.Relax(err)
            })
        }
    }
}

func (m *Mirror) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {
TryNextMirror:
    for _, mirrorEntry := range mirrors {
        for _, mirroredChannelEntry := range mirrorEntry.ConnectedChannels {
            if mirroredChannelEntry.ChannelID == msg.ChannelID {
                // ignore bot messages
                if msg.Author.Bot == true {
                    continue TryNextMirror
                }
                sourceChannel, err := session.Channel(msg.ChannelID)
                helpers.Relax(err)
                // ignore commands
                prefix := helpers.GetPrefixForServer(sourceChannel.GuildID)
                if prefix != "" {
                    if strings.HasPrefix(content, prefix) {
                        return
                    }
                }
                var linksToRepost []string
                // get mirror attachements
                if len(msg.Attachments) > 0 {
                    for _, attachement := range msg.Attachments {
                        linksToRepost = append(linksToRepost, attachement.URL)
                    }
                }
                // get mirror links
                if strings.Contains(msg.Content, "http") {
                    linksFound := galleryUrlRegex.FindAllString(msg.Content, -1)
                    if len(linksFound) > 0 {
                        for _, linkFound := range linksFound {
                            if strings.HasPrefix(linkFound, "<") == false && strings.HasSuffix(linkFound, ">") == false {
                                linksToRepost = append(linksToRepost, linkFound)
                            }
                        }
                    }
                }
                // post mirror links
                if len(linksToRepost) > 0 {
                    sourceGuild, err := session.Guild(sourceChannel.GuildID)
                    helpers.Relax(err)
                    for _, linkToRepost := range linksToRepost {
                        for _, channelToMirrorToEntry := range mirrorEntry.ConnectedChannels {
                            if channelToMirrorToEntry.ChannelID != msg.ChannelID {
                                err := session.WebhookExecute(channelToMirrorToEntry.ChannelWebhookID, channelToMirrorToEntry.ChannelWebhookToken,
                                    false, &discordgo.WebhookParams{
                                        Content: fmt.Sprintf("posted %s in `#%s` on the `%s` server (<#%s>)",
                                            linkToRepost, sourceChannel.Name, sourceGuild.Name, sourceChannel.ID,
                                        ),
                                        Username:  msg.Author.Username,
                                        AvatarURL: helpers.GetAvatarUrl(msg.Author),
                                    })
                                helpers.Relax(err)
                                metrics.MirrorsPostsSent.Add(1)
                            }
                        }
                    }
                }
            }
        }
    }

}

func (m *Mirror) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {
}

func (m *Mirror) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {
}

func (m *Mirror) getEntryBy(key string, id string) DB_Mirror_Entry {
    var entryBucket DB_Mirror_Entry
    listCursor, err := rethink.Table("mirrors").Filter(
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

func (m *Mirror) getEntryByOrCreateEmpty(key string, id string) DB_Mirror_Entry {
    var entryBucket DB_Mirror_Entry
    listCursor, err := rethink.Table("mirrors").Filter(
        rethink.Row.Field(key).Eq(id),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.One(&entryBucket)

    if err == rethink.ErrEmptyResult {
        insert := rethink.Table("mirrors").Insert(DB_Mirror_Entry{})
        res, e := insert.RunWrite(helpers.GetDB())
        if e != nil {
            panic(e)
        } else {
            return m.getEntryByOrCreateEmpty("id", res.GeneratedKeys[0])
        }
    } else if err != nil {
        panic(err)
    }

    return entryBucket
}

func (m *Mirror) setEntry(entry DB_Mirror_Entry) {
    _, err := rethink.Table("mirrors").Update(entry).Run(helpers.GetDB())
    helpers.Relax(err)
}

func (m *Mirror) deleteEntryById(id string) {
    _, err := rethink.Table("mirrors").Filter(
        rethink.Row.Field("id").Eq(id),
    ).Delete().RunWrite(helpers.GetDB())
    helpers.Relax(err)
}

func (m *Mirror) GetMirrors() []DB_Mirror_Entry {
    var entryBucket []DB_Mirror_Entry
    listCursor, err := rethink.Table("mirrors").Run(helpers.GetDB())
    helpers.Relax(err)
    defer listCursor.Close()
    err = listCursor.All(&entryBucket)

    helpers.Relax(err)
    return entryBucket
}
