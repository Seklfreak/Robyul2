package main

import (
    "fmt"
    "github.com/bwmarrin/discordgo"
    "github.com/getsentry/raven-go"
    "github.com/sn0w/Karen/helpers"
    Logger "github.com/sn0w/Karen/logger"
    "github.com/sn0w/Karen/plugins"
    "github.com/sn0w/Karen/utils"
    "math/rand"
    "regexp"
    "strings"
    "time"
)

// Called after the gateway connected
func BotOnReady(session *discordgo.Session, event *discordgo.Ready) {
    Logger.INF("Connected to discord!")
    fmt.Printf(
        "\n To add me to your discord server visit https://discordapp.com/oauth2/authorize?client_id=%s&scope=bot&permissions=%s\n\n",
        utils.GetConfig().Path("discord.id").Data().(string),
        utils.GetConfig().Path("discord.perms").Data().(string),
    )

    discordSession = session

    // Init plugins
    tmpl := "[PLUG] %s reacts to [ %s]"
    for _, plugin := range plugins.PluginList {
        cmds := ""

        for _, cmd := range plugin.Commands() {
            cmds += cmd + " "
        }

        Logger.INF(fmt.Sprintf(
            tmpl,
            helpers.Typeof(plugin),
            cmds,
        ))

        plugin.Init(session)
    }

    // Init trigger plugins
    tmpl = "[TRIG] Registered %s"
    for _, plugin := range plugins.TriggerPluginList {
        Logger.INF(fmt.Sprintf(
            tmpl,
            helpers.Typeof(plugin),
        ))

        plugin.Init(session)
    }

    // Async stats
    go func() {
        time.Sleep(3 * time.Second)

        configName := utils.GetConfig().Path("bot.name").Data().(string)
        configAvatar := utils.GetConfig().Path("bot.avatar").Data().(string)

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

// Called after a new message was sent
// This will be called after *every* message on *every* server so it should die as soon as possible.
func BotOnMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
    // Ignore other bots and @everyone/@here
    if !message.Author.Bot && !message.MentionEveryone {
        // Get the channel
        // Ignore the event if we cannot resolve the channel
        channel, err := session.Channel(message.ChannelID)
        if err != nil {
            go raven.CaptureError(err, map[string]string{})
            return
        }

        // We only do things in guilds.
        // Get a friend already and stop chatting with bots
        if !channel.IsPrivate {
            // Check if the message contains @mentions
            if len(message.Mentions) >= 1 {
                // Check if someone is mentioning us
                if message.Mentions[0].ID == session.State.User.ID {
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
                        sendHelp(message)
                        return

                    case regexp.MustCompile("(?i)^PREFIX.*").Match(bmsg):
                        prefix, _ := utils.GetPrefixForServer(channel.GuildID)
                        if prefix == "" {
                            discordSession.ChannelMessageSend(
                                channel.ID,
                                "Seems like there is no prefix yet :thinking:\n" +
                                    "Admins can set one by typing for example `@Karen set prefix ?`",
                            )
                        }

                        discordSession.ChannelMessageSend(
                            channel.ID,
                            "The prefix is `" + prefix + "` :smiley:",
                        )
                        return

                    case regexp.MustCompile("(?i)^REFRESH CHAT SESSION$").Match(bmsg):
                        utils.RequireAdmin(session, message.Message, func() {
                            // Refresh cleverbot session
                            utils.CleverbotRefreshSession(channel.ID)
                            discordSession.ChannelMessageSend(channel.ID, ":cyclone: Refreshed!")
                        })
                        return

                    case regexp.MustCompile("(?i)^SET PREFIX (.){1,25}$").Match(bmsg):
                        utils.RequireAdmin(session, message.Message, func() {
                            // Extract prefix
                            prefix := strings.Split(
                                regexp.MustCompile("(?i)^SET PREFIX\\s").ReplaceAllString(msg, ""),
                                " ",
                            )[0]

                            // Set new prefix
                            err := utils.SetPrefixForServer(
                                channel.GuildID,
                                prefix,
                            )

                            if err != nil {
                                helpers.SendError(session, message.Message, err)
                            } else {
                                discordSession.ChannelMessageSend(channel.ID, ":white_check_mark: Saved! \n The prefix is now `" + prefix + "`")
                            }
                        })
                        return

                    default:
                        // Send to cleverbot
                        session.ChannelTyping(message.ChannelID)

                        // Resolve other @mentions before sending the message
                        for _, user := range message.Mentions {
                            msg = strings.Replace(msg, "<@" + user.ID + ">", user.Username, -1)
                        }

                        // Remove smileys
                        msg = regexp.MustCompile(`:\w+:`).ReplaceAllString(msg, "")

                        // Send to cleverbot
                        utils.CleverbotSend(session, channel.ID, msg)
                        return
                    }
                }
            }

            // Only continue if a prefix is set
            // Else check if any instant-replies match
            prefix, _ := utils.GetPrefixForServer(channel.GuildID)
            if prefix == "" {
                plugins.CallTriggerPlugins(message.Message, session)
                return
            }

            // Check if the message is prefixed for us
            if strings.HasPrefix(message.Content, prefix) {
                // Split the message into parts
                parts := strings.Split(message.Content, " ")

                // Save a sanitized version of the command (no prefix)
                cmd := strings.Replace(parts[0], prefix, "", 1)

                // Check if the user calls for help
                if cmd == "h" || cmd == "help" {
                    sendHelp(message)
                    return
                } else {
                    // Check if a module matches said command
                    // Do nothing otherwise
                    plugins.CallBotPlugin(
                        cmd,
                        strings.Replace(message.Content, prefix + cmd, "", -1),
                        message.Message,
                        discordSession,
                    )
                }
            }
        }
    }
}

func sendHelp(message *discordgo.MessageCreate) {
    discordSession.ChannelMessageSend(
        message.ChannelID,
        fmt.Sprintf("<@%s> Check out <http://meetkaren.xyz/commands>", message.Author.ID),
    )
}

// Changes the game interval every 10 seconds after called
func changeGameInterval(session *discordgo.Session) {
    for {
        err := session.UpdateStatus(0, games[rand.Intn(len(games))])
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
