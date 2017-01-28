package main

import (
    "fmt"
    "github.com/sn0w/discordgo"
    "github.com/getsentry/raven-go"
    "git.lukas.moe/sn0w/Karen/cache"
    "git.lukas.moe/sn0w/Karen/helpers"
    Logger "git.lukas.moe/sn0w/Karen/logger"
    "git.lukas.moe/sn0w/Karen/metrics"
    "git.lukas.moe/sn0w/Karen/plugins"
    "regexp"
    "strings"
    "time"
)

// BotOnReady gets called after the gateway connected
func BotOnReady(session *discordgo.Session, event *discordgo.Ready) {
    Logger.INFO.L("bot", "Connected to discord!")
    Logger.VERBOSE.L("bot", "Invite link: " + fmt.Sprintf(
        "https://discordapp.com/oauth2/authorize?client_id=%s&scope=bot&permissions=%s",
        helpers.GetConfig().Path("discord.id").Data().(string),
        helpers.GetConfig().Path("discord.perms").Data().(string),
    ))

    cache.SetSession(session)

    // Init plugins
    fmt.Println()
    tmpl := "[PLUG] %s reacts to [ %s]"
    for _, plugin := range plugins.PluginList {
        cmds := ""

        for _, cmd := range plugin.Commands() {
            cmds += cmd + " "
        }

        Logger.INFO.L("bot", fmt.Sprintf(
            tmpl,
            helpers.Typeof(plugin),
            cmds,
        ))

        plugin.Init(session)
    }

    // Init trigger plugins
    fmt.Println()
    tmpl = "[TRIG] %s gets triggered by [ %s]"
    for _, plugin := range plugins.TriggerPluginList {
        cmds := ""

        for _, cmd := range plugin.Triggers() {
            cmds += cmd + " "
        }

        Logger.INFO.L("bot", fmt.Sprintf(
            tmpl,
            helpers.Typeof(plugin),
            cmds,
        ))
    }

    go func() {
        time.Sleep(3 * time.Second)

        configName := helpers.GetConfig().Path("bot.name").Data().(string)
        configAvatar := helpers.GetConfig().Path("bot.avatar").Data().(string)

        // Change avatar if desired
        if configAvatar != "" && configAvatar != session.State.User.Avatar {
            session.UserUpdate(
                "",
                "",
                session.State.User.Username,
                configAvatar,
                "",
            )
        }

        // Change name if desired
        if configName != "" && configName != session.State.User.Username {
            session.UserUpdate(
                "",
                "",
                configName,
                session.State.User.Avatar,
                "",
            )
        }
    }()

    // Run async game-changer
    go changeGameInterval(session)

    // Run auto-leaver for non-beta guilds
    go autoLeaver(session)
}

// BotOnMessageCreate gets called after a new message was sent
// This will be called after *every* message on *every* server so it should die as soon as possible
// or spawn costly work inside of coroutines.
func BotOnMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
    // Ignore other bots and @everyone/@here
    if message.Author.Bot || message.MentionEveryone {
        return
    }

    // Get the channel
    // Ignore the event if we cannot resolve the channel
    channel, err := cache.Channel(message.ChannelID)
    if err != nil {
        go raven.CaptureError(err, map[string]string{})
        return
    }

    // We only do things in guilds.
    // Get a friend already and stop chatting with bots
    if channel.IsPrivate {
        return
    }

    // Check if the message contains @mentions for us
    if len(message.Mentions) > 0 && strings.HasPrefix(message.Content, "<@") && message.Mentions[0].ID == session.State.User.ID {
        // Prepare content for editing
        msg := message.Content

        /// Remove our @mention
        msg = strings.Replace(msg, "<@" + session.State.User.ID + ">", "", -1)

        // Trim message
        msg = strings.Trim(msg, " ")

        // Convert to []byte before matching
        bmsg := []byte(msg)

        // Match against common task patterns
        // Send to cleverbot if nothing matches
        switch {
        case regexp.MustCompile("(?i)^HELP.*").Match(bmsg):
            metrics.CommandsExecuted.Add(1)
            sendHelp(message)
            return

        case regexp.MustCompile("(?i)^PREFIX.*").Match(bmsg):
            metrics.CommandsExecuted.Add(1)
            prefix, _ := helpers.GetPrefixForServer(channel.GuildID)
            if prefix == "" {
                cache.GetSession().ChannelMessageSend(
                    channel.ID,
                    helpers.GetText("bot.prefix.not-set"),
                )
            }

            cache.GetSession().ChannelMessageSend(
                channel.ID,
                helpers.GetTextF("bot.prefix.is", prefix),
            )
            return

        case regexp.MustCompile("(?i)^REFRESH CHAT SESSION$").Match(bmsg):
            metrics.CommandsExecuted.Add(1)
            helpers.RequireAdmin(message.Message, func() {
                // Refresh cleverbot session
                helpers.CleverbotRefreshSession(channel.ID)
                cache.GetSession().ChannelMessageSend(channel.ID, helpers.GetText("bot.cleverbot.refreshed"))
            })
            return

        case regexp.MustCompile("(?i)^SET PREFIX (.){1,25}$").Match(bmsg):
            metrics.CommandsExecuted.Add(1)
            helpers.RequireAdmin(message.Message, func() {
                // Extract prefix
                prefix := strings.Split(
                    regexp.MustCompile("(?i)^SET PREFIX\\s").ReplaceAllString(msg, ""),
                    " ",
                )[0]

                // Set new prefix
                err := helpers.SetPrefixForServer(
                    channel.GuildID,
                    prefix,
                )

                if err != nil {
                    helpers.SendError(message.Message, err)
                } else {
                    cache.GetSession().ChannelMessageSend(channel.ID, helpers.GetTextF("bot.prefix.saved", prefix))
                }
            })
            return

        default:
            // Track usage
            metrics.CleverbotRequests.Add(1)

            // Send to cleverbot
            session.ChannelTyping(message.ChannelID)

            // Resolve other @mentions before sending the message
            for _, user := range message.Mentions {
                msg = strings.Replace(msg, "<@" + user.ID + ">", user.Username, -1)
            }

            // Remove smileys
            msg = regexp.MustCompile(`:\w+:`).ReplaceAllString(msg, "")

            // Send to cleverbot
            helpers.CleverbotSend(session, channel.ID, msg)
            return
        }
    }

    // Only continue if a prefix is set
    prefix, _ := helpers.GetPrefixForServer(channel.GuildID)
    if prefix == "" {
        return
    }

    // Check if the message is prefixed for us
    // If not exit
    if !strings.HasPrefix(message.Content, prefix) {
        return
    }

    // Split the message into parts
    parts := strings.Split(message.Content, " ")

    // Save a sanitized version of the command (no prefix)
    cmd := strings.Replace(parts[0], prefix, "", 1)

    // Check if the user calls for help
    if cmd == "h" || cmd == "help" {
        metrics.CommandsExecuted.Add(1)
        sendHelp(message)
        return
    }

    // Check if a module matches said command
    plugins.CallBotPlugin(
        cmd,
        strings.Replace(message.Content, prefix + cmd, "", -1),
        message.Message,
    )

    // Check if a trigger matches
    plugins.CallTriggerPlugin(
        cmd,
        strings.Replace(message.Content, prefix + cmd, "", -1),
        message.Message,
    )

    // Else exit
    return
}

func sendHelp(message *discordgo.MessageCreate) {
    cache.GetSession().ChannelMessageSend(
        message.ChannelID,
        helpers.GetTextF("bot.help", message.Author.ID),
    )
}

// Changes the game interval every 10 seconds after called
func changeGameInterval(session *discordgo.Session) {
    for {
        err := session.UpdateStatus(0, helpers.GetText("games"))
        if err != nil {
            raven.CaptureError(err, map[string]string{})
        }

        time.Sleep(10 * time.Second)
    }
}
