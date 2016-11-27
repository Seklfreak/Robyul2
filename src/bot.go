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
)

func onReady(session *discordgo.Session, event *discordgo.Ready) {
    Logger.INF("Connected to discord!")

    discordSession = session

    guilds, e := session.UserGuilds()
    if e == nil {
        channels := 0
        users := 0

        for _, guild := range guilds {
            channels += len(guild.Channels)
            users += len(guild.Members)
        }

        Logger.INF(fmt.Sprintf("Servers:%d | Channels:%d | Users:%d", len(guilds), channels, users))
    } else {
        Logger.ERR("Error retrieving stats")
        fmt.Println(e.Error())
    }

    fmt.Printf(
        "\n To add me to your discord server visit https://discordapp.com/oauth2/authorize?client_id=%s&scope=bot&permissions=%d\n\n",
        "249908516880515072",
        65535,
    )

    // Run async game-changer
    go changeGameInterval(session);
}

func onMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
    // Ignore other bots and @everyone/@here
    if (!message.Author.Bot && !message.MentionEveryone) {
        // We only do things in guilds.
        // Get a friend already and stop chatting with bots
        channel, _ := session.Channel(message.ChannelID)
        if (!channel.IsPrivate) {
            // Split the message into parts
            parts := strings.Split(message.Content, " ")
            msg := strings.Trim(strings.Join(append(parts[:0], parts[1:]...), " "), " ")

            // Check if the message contains @mentions
            if (len(message.Mentions) >= 1) {
                // Check if someone is mentioning us
                if (message.Mentions[0].ID == session.State.User.ID) {
                    go CCTV(message.Message)

                    switch {
                    case regexp.MustCompile("^REFRESH CHAT SESSION$").Match([]byte(msg)):
                        // Refresh cleverbot session
                        CleverbotRefreshSession(channel.ID)
                        discordSession.ChannelMessageSend(channel.ID, ":cyclone: Refreshed!")
                        return

                    case regexp.MustCompile("^SET PREFIX (.){0,10}$").Match([]byte(msg)):
                        // Set new prefix
                        err := SetPrefixForServer(channel.GuildID, strings.Replace(msg, "SET PREFIX ", "", 1))

                        if err != nil {
                            SendError(channel.ID, err)
                        } else {
                            discordSession.ChannelMessageSend(channel.ID, ":white_check_mark: Saved!")
                        }
                        return

                    default:
                        // Send to cleverbot
                        WhileTypingIn(channel.ID, func() {
                            CleverbotSend(channel.ID, msg)
                        })
                        return
                    }
                }
            }

            // Only continue if a prefix is set
            prefix, err := GetPrefixForServer(channel.GuildID)
            if err == nil {
                // Check if the message is prefixed for us
                if (strings.HasPrefix(message.Content, prefix)) {
                    // Check if a module matches that command
                    plugins.CallBotPlugin(parts[0], message.Message, &discordSession)
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