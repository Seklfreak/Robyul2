package plugins

import (
    "github.com/bwmarrin/discordgo"
    "strings"
    "github.com/Seklfreak/Robyul2/helpers"
    "github.com/Seklfreak/Robyul2/logger"
    "fmt"
    "strconv"
    "github.com/getsentry/raven-go"
    "time"
    rethink "github.com/gorethink/gorethink"
)

type Troublemaker struct{}

func (t *Troublemaker) Commands() []string {
    return []string{
        "troublemaker",
        "troublemakers",
        "tm",
    }
}

type DB_Troublemaker_Entry struct {
    ID                string         `gorethink:"id,omitempty"`
    UserID            string         `gorethink:"userid"`
    Reason            string         `gorethink:"reason"`
    CreatedAt         time.Time      `gorethink:"createdat"`
    ReportedByGuildID string         `gorethink:"reportedby_guildid"`
    ReportedByUserID  string         `gorethink:"reportedby_userid"`
}

func (t *Troublemaker) Init(session *discordgo.Session) {

}

func (t *Troublemaker) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    args := strings.Fields(content)
    if len(args) >= 1 {
        switch args[0] {
        case "participate":
            helpers.RequireAdmin(msg, func() {
                session.ChannelTyping(msg.ChannelID)

                channel, err := session.State.Channel(msg.ChannelID)
                helpers.Relax(err)

                settings := helpers.GuildSettingsGetCached(channel.GuildID)

                if len(args) >= 2 {
                    // Set new log channel
                    targetChannel, err := helpers.GetChannelFromMention(args[1])
                    if err != nil || targetChannel.ID == "" {
                        session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                        return
                    }

                    settings.TroublemakerIsParticipating = true
                    settings.TroublemakerLogChannel = targetChannel.ID
                    err = helpers.GuildSettingsSet(channel.GuildID, settings)
                    helpers.Relax(err)

                    _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.troublemaker.participation-enabled"))
                    helpers.Relax(err)

                    return
                } else {
                    // Disable logging
                    settings.TroublemakerIsParticipating = false
                    err = helpers.GuildSettingsSet(channel.GuildID, settings)
                    helpers.Relax(err)

                    _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.troublemaker.participation-disabled"))
                    helpers.Relax(err)

                    return
                }
                return
            })
            break
        default:
            helpers.RequireMod(msg, func() {
                session.ChannelTyping(msg.ChannelID)

                channel, err := session.State.Channel(msg.ChannelID)
                helpers.Relax(err)
                guild, err := session.State.Guild(channel.GuildID)
                helpers.Relax(err)

                if len(args) < 2 {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                    return
                }

                targetUser, err := helpers.GetUserFromMention(args[0])
                if err != nil || targetUser.ID == "" {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                    return
                }

                reasonText := strings.TrimSpace(strings.Replace(content, strings.Join(args[:1], " "), "", 1))

                if helpers.ConfirmEmbed(msg.ChannelID, msg.Author, helpers.GetTextF("plugins.troublemaker.report-confirm",
                    targetUser.Username, targetUser.Discriminator, targetUser.ID, targetUser.ID, reasonText,
                ), "✅", "🚫") == true {
                    // Save to log DB
                    troublemakerLogEntry := t.getEntryByOrCreateEmpty("id", "")
                    troublemakerLogEntry.UserID = targetUser.ID
                    troublemakerLogEntry.Reason = reasonText
                    troublemakerLogEntry.CreatedAt = time.Now()
                    troublemakerLogEntry.ReportedByGuildID = guild.ID
                    troublemakerLogEntry.ReportedByUserID = msg.Author.ID
                    t.setEntry(troublemakerLogEntry)

                    logger.INFO.L("troublemaker", fmt.Sprintf("will notify about troublemaker %s (#%s) by %s (#%s) on %s (#%s) reason %s",
                        targetUser.Username, targetUser.ID,
                        msg.Author.Username, msg.Author.ID,
                        guild.Name, guild.ID,
                        reasonText,
                    ))

                    guildsToNotify := make([]*discordgo.Guild, 0)

                    for _, guildToNotify := range session.State.Guilds {
                        if guildToNotify.ID != guild.ID {
                            guildToNotifySettings := helpers.GuildSettingsGetCached(guildToNotify.ID)
                            if guildToNotifySettings.TroublemakerIsParticipating == true && guildToNotifySettings.TroublemakerLogChannel != "" {
                                guildsToNotify = append(guildsToNotify, guildToNotify)
                            }
                        }
                    }

                    successMessage, _ := session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.troublemaker.report-successful", len(guildsToNotify)))

                    // Send notifications out
                    go func() {

                        for _, guildToNotify := range guildsToNotify {
                            guildToNotifySettings := helpers.GuildSettingsGetCached(guildToNotify.ID)
                            if guildToNotifySettings.TroublemakerIsParticipating == true && guildToNotifySettings.TroublemakerLogChannel != "" {
                                targetUserIsOnServer := false
                                _, err := session.GuildMember(guildToNotify.ID, targetUser.ID)
                                if err == nil {
                                    targetUserIsOnServer = true
                                }

                                reportEmbed := &discordgo.MessageEmbed{
                                    Title: helpers.GetTextF("plugins.troublemaker.report-embed-title", targetUser.Username, targetUser.Discriminator),
                                    Description: helpers.GetTextF("plugins.troublemaker.report-embed-description", targetUser.ID, targetUser.ID),
                                    URL:       helpers.GetAvatarUrl(targetUser),
                                    Thumbnail: &discordgo.MessageEmbedThumbnail{URL: helpers.GetAvatarUrl(targetUser)},
                                    Footer:    &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.troublemaker.report-embed-footer", len(guildsToNotify))},
                                    Color:     0x0FADED,
                                    Fields: []*discordgo.MessageEmbedField{
                                        {Name: "Reason stated", Value: reasonText, Inline: false},
                                    },
                                }

                                if targetUserIsOnServer == true {
                                    reportEmbed.Fields = append(reportEmbed.Fields, &discordgo.MessageEmbedField{
                                        Name: "Member status", Value: "⚠ User is on this server", Inline: false,
                                    })
                                } else {
                                    reportEmbed.Fields = append(reportEmbed.Fields, &discordgo.MessageEmbedField{
                                        Name: "Member status", Value: "✅ User is not on this server", Inline: false,
                                    })
                                }

                                reportEmbed.Fields = append(reportEmbed.Fields, &discordgo.MessageEmbedField{
                                    Name: "Reported by", Value: fmt.Sprintf("**%s** (#%s) <@%s>\non **%s** (#%s)",
                                    msg.Author.Username, msg.Author.ID, msg.Author.ID, guild.Name, guild.ID,
                                ), Inline: false})

                                _, err = session.ChannelMessageSendEmbed(guildToNotifySettings.TroublemakerLogChannel, reportEmbed)
                                if err != nil {
                                    raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{
                                        "ChannelID":       msg.ChannelID,
                                        "Content":         msg.Content,
                                        "Timestamp":       string(msg.Timestamp),
                                        "TTS":             strconv.FormatBool(msg.Tts),
                                        "MentionEveryone": strconv.FormatBool(msg.MentionEveryone),
                                        "IsBot":           strconv.FormatBool(msg.Author.Bot),
                                    })
                                }
                            }
                        }

                        if successMessage.ID != "" {
                            session.MessageReactionAdd(msg.ChannelID, successMessage.ID, "👌")
                        }
                        return
                    }()
                    return
                }
                return
            })
            break
        }
    }
}

func (t *Troublemaker) getEntryByOrCreateEmpty(key string, id string) DB_Troublemaker_Entry {
    var entryBucket DB_Troublemaker_Entry
    listCursor, err := rethink.Table("troublemakerlog").Filter(
        rethink.Row.Field(key).Eq(id),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.One(&entryBucket)

    // If user has no DB entries create an empty document
    if err == rethink.ErrEmptyResult {
        insert := rethink.Table("troublemakerlog").Insert(DB_Troublemaker_Entry{})
        res, e := insert.RunWrite(helpers.GetDB())
        // If the creation was successful read the document
        if e != nil {
            panic(e)
        } else {
            return t.getEntryByOrCreateEmpty("id", res.GeneratedKeys[0])
        }
    } else if err != nil {
        panic(err)
    }

    return entryBucket
}

func (t *Troublemaker) setEntry(entry DB_Troublemaker_Entry) {
    _, err := rethink.Table("troublemakerlog").Update(entry).Run(helpers.GetDB())
    helpers.Relax(err)
}
