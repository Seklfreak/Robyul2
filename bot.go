package main

import (
    "fmt"
    "github.com/bwmarrin/discordgo"
    "github.com/getsentry/raven-go"
    "github.com/sn0w/Karen/cache"
    "github.com/sn0w/Karen/helpers"
    Logger "github.com/sn0w/Karen/logger"
    "github.com/sn0w/Karen/metrics"
    "github.com/sn0w/Karen/plugins"
    "math/rand"
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
    tmpl = "[TRIG] Registered %s"
    for _, plugin := range plugins.TriggerPluginList {
        Logger.INFO.L("bot", fmt.Sprintf(
            tmpl,
            helpers.Typeof(plugin),
        ))

        plugin.Init(session)
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

    // Check if the message contains @mentions
    if len(message.Mentions) >= 1 {
        // Check if someone is mentioning us
        if strings.HasPrefix(message.Content, "<@") && message.Mentions[0].ID == session.State.User.ID {
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
                        "Seems like there is no prefix yet :thinking:\n" +
                            "Admins can set one by typing for example `@Karen set prefix ?`",
                    )
                }

                cache.GetSession().ChannelMessageSend(
                    channel.ID,
                    "The prefix is `" + prefix + "` :smiley:",
                )
                return

            case regexp.MustCompile("(?i)^REFRESH CHAT SESSION$").Match(bmsg):
                metrics.CommandsExecuted.Add(1)
                helpers.RequireAdmin(message.Message, func() {
                    // Refresh cleverbot session
                    helpers.CleverbotRefreshSession(channel.ID)
                    cache.GetSession().ChannelMessageSend(channel.ID, ":cyclone: Refreshed!")
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
                        cache.GetSession().ChannelMessageSend(channel.ID, ":white_check_mark: Saved! \n The prefix is now `" + prefix + "`")
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
    }

    // Only continue if a prefix is set
    // Else check if any instant-replies match
    prefix, _ := helpers.GetPrefixForServer(channel.GuildID)
    if prefix == "" {
        plugins.CallTriggerPlugins(message.Message)
        return
    }

    // Check if the message is prefixed for us
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
    // Do nothing otherwise
    plugins.CallBotPlugin(
        cmd,
        strings.Replace(message.Content, prefix + cmd, "", -1),
        message.Message,
    )
}

func sendHelp(message *discordgo.MessageCreate) {
    cache.GetSession().ChannelMessageSend(
        message.ChannelID,
        fmt.Sprintf("<@%s> Check out <http://meetkaren.xyz/commands>", message.Author.ID),
    )
}

// Changes the game interval every 10 seconds after called
func changeGameInterval(session *discordgo.Session) {
    for {
        err := session.UpdateStatus(
            0,
            games[rand.Intn(len(games))],
        )
        if err != nil {
            raven.CaptureError(err, map[string]string{})
        }

        time.Sleep(10 * time.Second)
    }
}

// List of games to randomly show
var games = []string{
    // Random stuff
    "async is the future!",
    "down with OOP!",
    "spoopy stuff",
    "Planking",

    // Kaomoji
    "ʕ•ᴥ•ʔ",
    "༼ つ ◕_◕ ༽つ",
    "(ﾉ◕ヮ◕)ﾉ*:･ﾟ✧",
    "( ͡° ͜ʖ ͡°)",
    "¯\\_(ツ)_/¯",
    "(ง ͠° ͟ل͜ ͡°)ง",
    "ಠ_ಠ",
    "(╯°□°)╯︵ ʞooqǝɔɐɟ",
    "♪~ ᕕ(ᐛ)ᕗ",
    "\\ (•◡•) /",
    "｡◕‿◕｡",

    // actual games
    "Hearthstone",
    "Overwatch",
    "HuniePop",
    "Candy Crush",
    "Hyperdimension Neptunia",
    "Final Fantasy MCMX",
    "CIV V",
    "Pokemon Go",
    "Simulation Simulator 2016",
    "Half Life 3",
    "Nekopara",

    // software
    "with FFMPEG",
    "with libav",
    "with gophers",
    "with python",
    "with reflections",

    // names
    "with Shinobu-Chan",
    "with Ako-Chan",
    "with Nadeko",
    "with Miku",
    "with you O_o",
    "with cats",
    "with JOHN CENA",
    "with senpai",
    "with Serraniel#8978",
    "with 0xFADED#3237",
    "with C0untLizzi",
    "with moot",
    "with your waifu",
    "with Trump",
}
