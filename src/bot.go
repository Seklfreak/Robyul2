package main

import (
    "fmt"
    Logger "./logger"
    "github.com/bwmarrin/discordgo"
    "math/rand"
    "time"
    "strings"
    "regexp"
    "./plugins"
    "./utils"
)

func onReady(session *discordgo.Session, event *discordgo.Ready) {
    Logger.INF("Connected to discord!")

    discordSession = session

    // Print plugin list
    tmpl := "[PLUG] %s reacts to [ %s]"

    for _, plugin := range plugins.GetPlugins() {
        cmds := ""

        for cmd := range plugin.Commands() {
            cmds += cmd + " "
        }

        Logger.INF(fmt.Sprintf(tmpl, plugin.Name(), cmds))
    }

    // Async stats
    go func() {
        time.Sleep(3 * time.Second)

        users := make(map[string]string)
        channels := 0
        guilds := session.State.Guilds

        for _, guild := range guilds {
            channels += len(guild.Channels)

            for _, u := range guild.Members {
                users[u.User.ID] = u.User.Username
            }
        }

        Logger.INF(fmt.Sprintf("Servers:%d | Channels:%d | Users:%d", len(guilds), channels, len(users)))
    }()

    // Run async game-changer
    go changeGameInterval(session)
}

func onMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
    // Ignore other bots and @everyone/@here
    if (!message.Author.Bot && !message.MentionEveryone) {
        // We only do things in guilds.
        // Get a friend already and stop chatting with bots
        channel, _ := session.Channel(message.ChannelID)
        if (!channel.IsPrivate) {
            // Check if the message contains @mentions
            if (len(message.Mentions) >= 1) {
                // Check if someone is mentioning us
                if (message.Mentions[0].ID == session.State.User.ID) {
                    go utils.CCTV(session, message.Message)

                    // Prepare content for editing
                    msg := message.Content

                    /// Remove our @mention
                    msg = strings.Replace(msg, "<@" + session.State.User.ID + ">", "", -1)

                    switch {
                    case regexp.MustCompile("^REFRESH CHAT SESSION$").Match([]byte(msg)):
                        // Refresh cleverbot session
                        utils.CleverbotRefreshSession(channel.ID)
                        discordSession.ChannelMessageSend(channel.ID, ":cyclone: Refreshed!")
                        return

                    case regexp.MustCompile("^SET PREFIX (.){0,10}$").Match([]byte(msg)):
                        // Set new prefix
                        err := utils.SetPrefixForServer(channel.GuildID, strings.Replace(msg, "SET PREFIX ", "", 1))

                        if err != nil {
                            utils.SendError(session, channel.ID, err)
                        } else {
                            discordSession.ChannelMessageSend(channel.ID, ":white_check_mark: Saved!")
                        }
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
            prefix, err := utils.GetPrefixForServer(channel.GuildID)
            if err == nil {
                // Split the message into parts
                parts := strings.Split(message.Content, " ")

                // Save a sanitized version of the command (no prefix)
                cmd := strings.Replace(parts[0], prefix, "", 1)

                // Check if the message is prefixed for us
                if (strings.HasPrefix(message.Content, prefix)) {
                    // Check if the user calls for help
                    if cmd == "h" || cmd == "help" {
                        // Print help of all plugins
                        msg := ""

                        msg += "Hi " + message.Author.Username + " :smiley:\n"
                        msg += "These are all usable commands:\n"
                        msg += "```\n"

                        for _, plugin := range plugins.GetPlugins() {
                            description := plugin.Description()

                            if description == "" {
                                description = "no description"
                            }

                            msg += fmt.Sprintf("%s [%s]", plugin.Name(), description) + "\n"

                            for cmd, usage := range plugin.Commands() {
                                if usage == "" {
                                    usage = "[no usage information]"
                                }

                                msg += fmt.Sprintf("\t %s \t\t - %s\n", prefix + cmd, usage)
                            }

                            msg += "\n"
                        }

                        msg += "\n```"

                        discordSession.ChannelMessageSend(
                            message.ChannelID,
                            fmt.Sprintf("<@%s> :mailbox_with_mail:", message.Author.ID),
                        )

                        uc, err := discordSession.UserChannelCreate(message.Author.ID)
                        if err == nil {
                            discordSession.ChannelMessageSend(uc.ID, msg)
                        }
                    } else {
                        // Check if a module matches said command
                        // Do nothing otherwise
                        plugins.CallBotPlugin(
                            cmd,
                            strings.Replace(message.Content, prefix+cmd, "", -1),
                            message.Message,
                            discordSession,
                        )
                    }
                }
            }
        }
    }
}

func changeGameInterval(session *discordgo.Session) {
    for {
        session.UpdateStatus(0, games[rand.Intn(len(games))])
        time.Sleep(10 * time.Second)
    }
}

var games = []string{
    // Random stuff
    "async is the future!",
    "down with OOP!",
    "spoopy stuff",
    "with human pets",
    "Planking",
    "Rare Pepe",

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
    "CIV V (forever!)",
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