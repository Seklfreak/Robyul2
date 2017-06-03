package plugins

import (
    "github.com/bwmarrin/discordgo"
    "strings"
    "github.com/Seklfreak/Robyul2/helpers"
    "time"
    "regexp"
    rethink "github.com/gorethink/gorethink"
    "fmt"
)

type Nuke struct{}

type DBNukeLogEntry struct {
    ID       string             `gorethink:"id,omitempty"`
    UserID   string             `gorethink:"userid"`
    UserName string `gorethink:"username"`
    NukerID  string             `gorethink:"nukerid"`
    Reason   string             `gorethink:"reason"`
    NukedAt  time.Time `gorethink:"nukedat"`
}

func (n *Nuke) Commands() []string {
    return []string{
        "nuke",
    }
}

var nukeMods = []string{
    "116620585638821891", // Sekl
    "134298438559858688", // Kakkela
}

func (n *Nuke) Init(session *discordgo.Session) {
    splitChooseRegex = regexp.MustCompile(`'.*?'|".*?"|\S+`)
}

func (n *Nuke) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    args := strings.Fields(content)
    if len(args) >= 1 {
        switch args[0] {
        case "user": // [p]nuke user <user id/mention> "<reason>"
            if !n.IsNukeMod(msg.Author.ID) {
                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.nuke.no-nukemod-permissions"))
                helpers.Relax(err)
                return
            } else {
                session.ChannelTyping(msg.ChannelID)

                safeArgs := splitChooseRegex.FindAllString(content, -1)
                if len(safeArgs) < 3 {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
                    return
                } else {
                    var err error
                    var targetUser *discordgo.User
                    targetUser, err = helpers.GetUserFromMention(strings.Trim(safeArgs[1], "\""))
                    if err != nil {
                        if err, ok := err.(*discordgo.RESTError); ok && err.Message.Code == 10013 {
                            _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.nuke.user-not-found"))
                            helpers.Relax(err)
                            return
                        } else {
                            helpers.Relax(err)
                            return
                        }
                    }

                    reason := strings.Trim(safeArgs[2], "\"")

                    if helpers.ConfirmEmbed(msg.ChannelID, msg.Author, helpers.GetTextF("plugins.nuke.nuke-confirm",
                        targetUser.Username, targetUser.ID, targetUser.ID, reason), "✅", "🚫") == true {
                        var entryBucket DBNukeLogEntry
                        listCursor, err := rethink.Table("nukelog").Filter(
                            rethink.Row.Field("userid").Eq(targetUser.ID),
                        ).Run(helpers.GetDB())
                        helpers.Relax(err)
                        defer listCursor.Close()
                        err = listCursor.One(&entryBucket)
                        if err != rethink.ErrEmptyResult || entryBucket.ID != "" {
                            _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.nuke.already-nuked"))
                            helpers.Relax(err)
                            return
                        }

                        nukeLogEntry := n.getEntryByOrCreateEmpty("id", "")
                        nukeLogEntry.UserID = targetUser.ID
                        nukeLogEntry.UserName = targetUser.Username
                        nukeLogEntry.NukedAt = time.Now().UTC()
                        nukeLogEntry.NukerID = msg.Author.ID
                        nukeLogEntry.Reason = reason
                        n.setEntry(nukeLogEntry)

                        _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.nuke.nuke-saved-in-db"))
                        helpers.Relax(err)

                        bannedOnN := 0

                        for _, targetGuild := range session.State.Guilds {
                            targetGuildSettings := helpers.GuildSettingsGetCached(targetGuild.ID)
                            fmt.Println("checking server: ", targetGuild.Name)
                            if targetGuildSettings.NukeIsParticipating == true {
                                err = session.GuildBanCreate(targetGuild.ID, targetUser.ID, 1)
                                if err != nil {
                                    if err, ok := err.(*discordgo.RESTError); ok {
                                        session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.nuke.ban-error",
                                            targetGuild.Name, targetGuild.ID, err.Message.Message))
                                        if targetGuildSettings.NukeLogChannel != "" {
                                            session.ChannelMessageSend(targetGuildSettings.NukeLogChannel,
                                                helpers.GetTextF("plugins.nuke.onserver-banned-error",
                                                    targetUser.Username, targetUser.ID,
                                                    err.Message.Message,
                                                    msg.Author.Username, msg.Author.ID,
                                                    reason))
                                        }
                                    } else {
                                        session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.nuke.ban-error",
                                            targetGuild.Name, targetGuild.ID, err.Error()))
                                        if targetGuildSettings.NukeLogChannel != "" {
                                            session.ChannelMessageSend(targetGuildSettings.NukeLogChannel,
                                                helpers.GetTextF("plugins.nuke.onserver-banned-error",
                                                    targetUser.Username, targetUser.ID,
                                                    err.Error(),
                                                    msg.Author.Username, msg.Author.ID,
                                                    reason))
                                        }
                                    }
                                } else {
                                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.nuke.banned-on-server",
                                        targetGuild.Name, targetGuild.ID))
                                    if targetGuildSettings.NukeLogChannel != "" {
                                        session.ChannelMessageSend(targetGuildSettings.NukeLogChannel,
                                            helpers.GetTextF("plugins.nuke.onserver-banned-success",
                                                targetUser.Username, targetUser.ID,
                                                msg.Author.Username, msg.Author.ID,
                                                reason))
                                    }
                                    bannedOnN += 1
                                }
                            }
                        }

                        _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.nuke.nuke-completed", bannedOnN))
                        helpers.Relax(err)
                    }
                }
            }
            return
        case "participate": // [p]nuke participate [<log channel>]
            helpers.RequireAdmin(msg, func() {
                session.ChannelTyping(msg.ChannelID)

                channel, err := session.State.Channel(msg.ChannelID)
                helpers.Relax(err)

                settings := helpers.GuildSettingsGetCached(channel.GuildID)

                if len(args) >= 2 {
                    targetChannel, err := helpers.GetChannelFromMention(args[1])
                    if err != nil {
                        session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
                        return
                    }

                    nukeModMentions := make([]string, 0)
                    for _, nukeMod := range nukeMods {
                        nukeModMentions = append(nukeModMentions, "<@"+nukeMod+">")
                    }

                    if helpers.ConfirmEmbed(msg.ChannelID, msg.Author, helpers.GetTextF("plugins.nuke.participation-confirm", strings.Join(nukeModMentions, ", ")), "✅", "🚫") == true {
                        settings.NukeIsParticipating = true
                        settings.NukeLogChannel = targetChannel.ID
                        err = helpers.GuildSettingsSet(channel.GuildID, settings)
                        helpers.Relax(err)

                        _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.nuke.participation-enabled"))
                        helpers.Relax(err)
                        // TODO: ask to ban people already nuked?
                    }
                } else {
                    settings.NukeIsParticipating = false
                    err = helpers.GuildSettingsSet(channel.GuildID, settings)
                    helpers.Relax(err)

                    _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.nuke.participation-disabled"))
                    helpers.Relax(err)
                }
            })
            return
        case "log": // [p]nuke log
            helpers.RequireMod(msg, func() {
                session.ChannelTyping(msg.ChannelID)

                var entryBucket []DBNukeLogEntry
                listCursor, err := rethink.Table("nukelog").Run(helpers.GetDB())
                helpers.Relax(err)
                defer listCursor.Close()
                err = listCursor.All(&entryBucket)

                logMessage := "**Nuke Log:**\n"
                for _, logEntry := range entryBucket {
                    logMessage += fmt.Sprintf("ID: `#%s`, Username: `%s`\n", logEntry.UserID, logEntry.UserName)
                }
                logMessage += "All usernames are from the time they got nuked."

                for _, page := range helpers.Pagify(logMessage, "\n") {
                    _, err := session.ChannelMessageSend(msg.ChannelID, page)
                    helpers.Relax(err)
                }
            })
            return
        }
    }
}

func (n *Nuke) getEntryByOrCreateEmpty(key string, id string) DBNukeLogEntry {
    var entryBucket DBNukeLogEntry
    listCursor, err := rethink.Table("nukelog").Filter(
        rethink.Row.Field(key).Eq(id),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.One(&entryBucket)

    // If user has no DB entries create an empty document
    if err == rethink.ErrEmptyResult {
        insert := rethink.Table("nukelog").Insert(DBNukeLogEntry{})
        res, e := insert.RunWrite(helpers.GetDB())
        // If the creation was successful read the document
        if e != nil {
            panic(e)
        } else {
            return n.getEntryByOrCreateEmpty("id", res.GeneratedKeys[0])
        }
    } else if err != nil {
        panic(err)
    }

    return entryBucket
}

func (n *Nuke) setEntry(entry DBNukeLogEntry) {
    _, err := rethink.Table("nukelog").Update(entry).Run(helpers.GetDB())
    helpers.Relax(err)
}

func (n *Nuke) IsNukeMod(id string) bool {
    for _, s := range nukeMods {
        if s == id {
            return true
        }
    }

    return false
}
