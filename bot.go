package main

import (
    "github.com/Seklfreak/Robyul2/cache"
    "github.com/Seklfreak/Robyul2/helpers"
    Logger "github.com/Seklfreak/Robyul2/logger"
    "github.com/Seklfreak/Robyul2/metrics"
    "github.com/Seklfreak/Robyul2/modules"
    "github.com/Seklfreak/Robyul2/ratelimits"
    "github.com/getsentry/raven-go"
    "github.com/bwmarrin/discordgo"
    "regexp"
    "strings"
    "time"
    "github.com/Seklfreak/Robyul2/emojis"
    "fmt"
)

// BotOnReady gets called after the gateway connected
func BotOnReady(session *discordgo.Session, event *discordgo.Ready) {
    Logger.INFO.L("bot", "Connected to discord!")
    Logger.VERBOSE.L("bot", "Invite link: "+fmt.Sprintf(
        "https://discordapp.com/oauth2/authorize?client_id=%s&scope=bot&permissions=%s",
        helpers.GetConfig().Path("discord.id").Data().(string),
        helpers.GetConfig().Path("discord.perms").Data().(string),
    ))

    // Cache the session
    cache.SetSession(session)

    // Load and init all modules
    modules.Init(session)

    // Run async worker for guild changes
    go helpers.GuildSettingsUpdater()

    // Run async game-changer
    //go changeGameInterval(session)

    // Run auto-leaver for non-beta guilds
    //go autoLeaver(session)

    // Run ratelimiter
    ratelimits.Container.Init()

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
    //go changeGameInterval(session)

    // Run auto-leaver for non-beta guilds
    //go autoLeaver(session)
}

func BotOnGuildMemberAdd(session *discordgo.Session, member *discordgo.GuildMemberAdd) {
    modules.CallExtendedPluginOnGuildMemberAdd(
        member.Member,
    )
}

func BotOnGuildMemberRemove(session *discordgo.Session, member *discordgo.GuildMemberRemove) {
    modules.CallExtendedPluginOnGuildMemberRemove(
        member.Member,
    )
}

// BotOnMessageCreate gets called after a new message was sent
// This will be called after *every* message on *every* server so it should die as soon as possible
// or spawn costly work inside of coroutines.
func BotOnMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
    // Ignore other bots and @everyone/@here
    if message.Author.Bot || message.MentionEveryone {
        return
    }

    // Check if the user is allowed to request commands
    if !ratelimits.Container.HasKeys(message.Author.ID) {
        session.ChannelMessageSend(message.ChannelID, helpers.GetTextF("bot.ratelimit.hit", message.Author.ID))

        ratelimits.Container.Set(message.Author.ID, -1)
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
    if strings.HasPrefix(message.Content, "<@") && len(message.Mentions) > 0 && message.Mentions[0].ID == session.State.User.ID {
        // Consume a key for this action
        e := ratelimits.Container.Drain(1, message.Author.ID)
        if e != nil {
            return
        }

        // Prepare content for editing
        msg := message.Content

        /// Remove our @mention
        msg = strings.Replace(msg, "<@"+session.State.User.ID+">", "", -1)

        // Trim message
        msg = strings.TrimSpace(msg)

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
            prefix := helpers.GetPrefixForServer(channel.GuildID)
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
                prefix := strings.Fields(regexp.MustCompile("(?i)^SET PREFIX\\s").ReplaceAllString(msg, ""))[0]

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

            // Mark typing
            session.ChannelTyping(message.ChannelID)

            // Resolve other @mentions before sending the message
            for _, user := range message.Mentions {
                msg = strings.Replace(msg, "<@"+user.ID+">", user.Username, -1)
            }

            // Remove smileys
            msg = regexp.MustCompile(`:\w+:`).ReplaceAllString(msg, "")

            // Send to cleverbot
            helpers.CleverbotSend(session, channel.ID, msg)
            return
        }
    }

    modules.CallExtendedPlugin(
        message.Content,
        message.Message,
    )

    // Only continue if a prefix is set
    prefix := helpers.GetPrefixForServer(channel.GuildID)
    if prefix == "" {
        return
    }

    // Check if the message is prefixed for us
    // If not exit
    if !strings.HasPrefix(message.Content, prefix) {
        return
    }

    // Split the message into parts
    parts := strings.Fields(message.Content)

    // Save a sanitized version of the command (no prefix)
    cmd := strings.Replace(parts[0], prefix, "", 1)

    // Check if the user calls for help
    if cmd == "h" || cmd == "help" {
        metrics.CommandsExecuted.Add(1)
        sendHelp(message)
        return
    }

    // Separate arguments from the command
    content := strings.TrimSpace(strings.Replace(message.Content, prefix+cmd, "", -1))

    // Check if a module matches said command
    modules.CallBotPlugin(cmd, content, message.Message)

    // Check if a trigger matches
    modules.CallTriggerPlugin(cmd, content, message.Message)
}

// BotOnReactionAdd gets called after a reaction is added
// This will be called after *every* reaction added on *every* server so it
// should die as soon as possible or spawn costly work inside of coroutines.
// This is currently used for the *poll* plugin.
func BotOnReactionAdd(session *discordgo.Session, reaction *discordgo.MessageReactionAdd) {
    modules.CallExtendedPluginOnReactionAdd(reaction)

    if user, err := session.User(reaction.UserID); err == nil && user.Bot {
        return
    }

    channel, err := session.Channel(reaction.ChannelID)
    if err != nil {
        return
    }
    if emojis.ToNumber(reaction.Emoji.Name) == -1 {
        //session.MessageReactionRemove(reaction.ChannelID, reaction.MessageID, reaction.Emoji.Name, reaction.UserID)
        return
    }
    if helpers.VotePollIfItsOne(channel.GuildID, reaction.MessageReaction) {
        helpers.UpdatePollMsg(channel.GuildID, reaction.MessageID)
    }

}

func BotOnReactionRemove(session *discordgo.Session, reaction *discordgo.MessageReactionRemove) {
    modules.CallExtendedPluginOnReactionRemove(reaction)
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
