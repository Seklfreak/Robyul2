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

type Gallery struct{}

type DB_Gallery_Entry struct {
    ID                        string `gorethink:"id,omitempty"`
    SourceChannelID           string `gorethink:"source_channel_id"`
    TargetChannelID           string `gorethink:"target_channel_id"`
    TargetChannelWebhookID    string `gorethink:"target_channel_webhook_id"`
    TargetChannelWebhookToken string `gorethink:"target_channel_webhook_token"`
    GuildID                   string `gorethink:"guild_id"`
    AddedByUserID             string `gorethink:"addedby_user_id"`
}

func (g *Gallery) Commands() []string {
    return []string{
        "gallery",
    }
}

const (
    galleryUrlRegexText string = `(<?https?:\/\/[^\s]+>?)`
)

var (
    galleryUrlRegex *regexp.Regexp
    galleries       []DB_Gallery_Entry
)

func (g *Gallery) Init(session *discordgo.Session) {
    galleryUrlRegex = regexp.MustCompile(galleryUrlRegexText)
    galleries = g.GetGalleries()
}

func (g *Gallery) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    args := strings.Split(content, " ")
    if len(args) >= 1 {
        switch args[0] {
        case "add": // [p]gallery add <source channel> <target channel> <webhook id> <webhook token>
            // @TODO: more secure way to exchange token
            helpers.RequireAdmin(msg, func() {
                session.ChannelMessageDelete(msg.ChannelID, msg.ID) // Delete command message to prevent people seeing the token
                progressMessage, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.gallery.add-progress"))
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
                sourceChannel, err := helpers.GetChannelFromMention(args[1])
                if err != nil || sourceChannel.ID == "" || sourceChannel.GuildID != channel.GuildID {
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

                newGalleryEntry := g.getEntryByOrCreateEmpty("id", "")
                newGalleryEntry.SourceChannelID = sourceChannel.ID
                newGalleryEntry.TargetChannelID = targetChannel.ID
                newGalleryEntry.TargetChannelWebhookID = targetChannelWebhookId
                newGalleryEntry.TargetChannelWebhookToken = targetChannelWebhookToken
                newGalleryEntry.AddedByUserID = msg.Author.ID
                newGalleryEntry.GuildID = channel.GuildID
                g.setEntry(newGalleryEntry)

                logger.INFO.L("galleries", fmt.Sprintf("Added Gallery on Server %s (%s) posting from #%s (%s) to #%s (%s)",
                    guild.Name, guild.ID, sourceChannel.Name, sourceChannel.ID, targetChannel.Name, targetChannel.ID))
                _, err = session.ChannelMessageEdit(msg.ChannelID, progressMessage.ID, helpers.GetText("plugins.gallery.add-success"))
                helpers.Relax(err)

                galleries = g.GetGalleries()
                return
            })
        case "list": // [p]gallery list
            session.ChannelTyping(msg.ChannelID)
            channel, err := session.Channel(msg.ChannelID)
            helpers.Relax(err)
            var entryBucket []DB_Gallery_Entry
            listCursor, err := rethink.Table("galleries").Filter(
                rethink.Row.Field("guild_id").Eq(channel.GuildID),
            ).Run(helpers.GetDB())
            helpers.Relax(err)
            defer listCursor.Close()
            err = listCursor.All(&entryBucket)

            if err == rethink.ErrEmptyResult || len(entryBucket) <= 0 {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.gallery.list-empty"))
                return
            }
            helpers.Relax(err)

            resultMessage := ":frame_photo: Galleries on this server:\n"
            for _, entry := range entryBucket {
                resultMessage += fmt.Sprintf("`%s`: posting from <#%s> to <#%s> (Webhook ID: `%s`)\n",
                    entry.ID, entry.SourceChannelID, entry.TargetChannelID, entry.TargetChannelWebhookID)
            }
            resultMessage += fmt.Sprintf("Found **%d** Galleries in total.", len(entryBucket))

            for _, resultPage := range helpers.Pagify(resultMessage, "\n") {
                _, err = session.ChannelMessageSend(msg.ChannelID, resultPage)
                helpers.Relax(err)
            }
            return
        case "delete", "del": // [p]gallery delete <gallery id>
            helpers.RequireAdmin(msg, func() {
                session.ChannelTyping(msg.ChannelID)
                if len(args) < 2 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                    helpers.Relax(err)
                    return
                }
                entryId := args[1]
                entryBucket := g.getEntryBy("id", entryId)
                if entryBucket.ID == "" {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.gallery.delete-not-found"))
                    return
                }
                galleryGuild, _ := session.Guild(entryBucket.GuildID)
                sourceChannel, _ := session.Channel(entryBucket.SourceChannelID)
                targetChannel, _ := session.Channel(entryBucket.TargetChannelID)
                g.deleteEntryById(entryBucket.ID)

                logger.INFO.L("galleries", fmt.Sprintf("Deleted Gallery on Server %s (%s) posting from #%s (%s) to #%s (%s)",
                    galleryGuild.Name, galleryGuild.ID, sourceChannel.Name, sourceChannel.ID, targetChannel.Name, targetChannel.ID))
                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.gallery.delete-success"))
                helpers.Relax(err)

                galleries = g.GetGalleries()
                return
            })
        case "refresh": // [p]gallery refresh
            helpers.RequireAdmin(msg, func() {
                session.ChannelTyping(msg.ChannelID)
                galleries = g.GetGalleries()
                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.gallery.refreshed-config"))
                helpers.Relax(err)
            })
        }
    }
}

func (g *Gallery) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {
TryNextGallery:
    for _, gallery := range galleries {
        if gallery.SourceChannelID == msg.ChannelID {
            // ignore bot messages
            if msg.Author.Bot == true {
                continue TryNextGallery
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
                for _, linkToRepost := range linksToRepost {
                    err := session.WebhookExecute(gallery.TargetChannelWebhookID, gallery.TargetChannelWebhookToken,
                        false, &discordgo.WebhookParams{
                            Content:   fmt.Sprintf("posted %s", linkToRepost),
                            Username:  msg.Author.Username,
                            AvatarURL: helpers.GetAvatarUrl(msg.Author),
                        })
                    helpers.Relax(err)
                    metrics.GalleryPostsSent.Add(1)
                }
            }
        }
    }

}

func (g *Gallery) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {
}

func (g *Gallery) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {
}

func (g *Gallery) getEntryBy(key string, id string) DB_Gallery_Entry {
    var entryBucket DB_Gallery_Entry
    listCursor, err := rethink.Table("galleries").Filter(
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

func (g *Gallery) getEntryByOrCreateEmpty(key string, id string) DB_Gallery_Entry {
    var entryBucket DB_Gallery_Entry
    listCursor, err := rethink.Table("galleries").Filter(
        rethink.Row.Field(key).Eq(id),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.One(&entryBucket)

    if err == rethink.ErrEmptyResult {
        insert := rethink.Table("galleries").Insert(DB_Gallery_Entry{})
        res, e := insert.RunWrite(helpers.GetDB())
        if e != nil {
            panic(e)
        } else {
            return g.getEntryByOrCreateEmpty("id", res.GeneratedKeys[0])
        }
    } else if err != nil {
        panic(err)
    }

    return entryBucket
}

func (g *Gallery) setEntry(entry DB_Gallery_Entry) {
    _, err := rethink.Table("galleries").Update(entry).Run(helpers.GetDB())
    helpers.Relax(err)
}

func (g *Gallery) deleteEntryById(id string) {
    _, err := rethink.Table("galleries").Filter(
        rethink.Row.Field("id").Eq(id),
    ).Delete().RunWrite(helpers.GetDB())
    helpers.Relax(err)
}

func (g *Gallery) GetGalleries() []DB_Gallery_Entry {
    var entryBucket []DB_Gallery_Entry
    listCursor, err := rethink.Table("galleries").Run(helpers.GetDB())
    helpers.Relax(err)
    defer listCursor.Close()
    err = listCursor.All(&entryBucket)

    helpers.Relax(err)
    return entryBucket
}
