package plugins

import (
    "github.com/bwmarrin/discordgo"
    "fmt"
    "os"
    "strings"
    "io/ioutil"
    "regexp"
    "../music"
)

var (
    music_foundYTD = false
    music_foundFFPROBE = false
    music_foundFFMPEG = false
    music_enabled = false
)

type Music struct{}

func (m Music) Name() string {
    return "Music"
}

func (m Music) HelpHidden() bool {
    return true
}

func (m Music) Description() string {
    return `
     Listen to Music :) [EXPERIMENTAL V3 | NOW WITH NATIVE SPEED! \o/]
     For a list of links supported by !add visit https://rg3.github.io/youtube-dl/supportedsites.html
    `
}

func (m Music) Commands() map[string]string {
    return map[string]string{
        "join" : "Join the voice channel you're in right now",
        "leave" : "Leaves the current voice channel",

        "play": "Start playing music",
        "stop": "Stop playing music",
        "skip": "Skip the current track",
        "clear": "Clear the playlist",

        "add": "<random|youtube/soundcloud/... link> Adds a track to the playlist",
        "list": "List the current playlist",

        "playing": "Show the current track",
        "np": "Alias for playing",
    }
}

func (m Music) Init(session *discordgo.Session) {
    fmt.Println("=> Checking for youtube-dl, ffmpeg and ffprobe...")
    for _, path := range strings.Split(os.Getenv("PATH"), ":") {
        files, _ := ioutil.ReadDir(path)

        for _, file := range files {
            switch {
            case regexp.MustCompile(`youtube-dl.*`).Match([]byte(file.Name())):
                music_foundYTD = true
                break

            case regexp.MustCompile(`ffprobe.*`).Match([]byte(file.Name())):
                music_foundFFPROBE = true
                break

            case regexp.MustCompile(`ffmpeg.*`).Match([]byte(file.Name())):
                music_foundFFMPEG = true
                break
            }
        }
    }

    if (music_foundYTD && music_foundFFPROBE && music_foundFFMPEG) {
        music_enabled = true

        fmt.Println("=> Found. Music enabled!")
    } else {
        fmt.Println("=> Not Found. Music disabled!")
    }
}

func (m Music) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    if !music_enabled {
        return
    }

    channel, err := session.Channel(msg.ChannelID)
    if err != nil {
        panic(err)
    }

    guild, err := session.Guild(channel.GuildID)
    if err != nil {
        panic(err)
    }

    // Voice channel ref
    vc := resolveVoiceChannel(msg.Author, guild, session)

    // Voice connection ref
    var voiceConnection *discordgo.VoiceConnection

    // Check if the user is connected at all
    if vc == nil {
        // Nope
        session.ChannelMessageSend(channel.ID, "You have to join a channel first! :neutral_face:")
        return
    }

    // He is connected for sure.
    // The routine would've stopped otherwise
    // Check if we are present in this channel too
    if session.VoiceConnections[guild.ID] == nil || session.VoiceConnections[guild.ID].ChannelID != vc.ID {
        // Nope.
        // Check if the user wanted us to join.
        // Else report the error
        if command == "join" {
            voiceConnection, err := session.ChannelVoiceJoin(guild.ID, channel.ID, false, false)
            if err != nil {
                panic(err)
            }
        } else {
            session.ChannelMessageSend(channel.ID, "You should join the channel I'm in or make me join yours before telling me to do stuff :neutral_face:")
            return
        }
    } else {
        // We are \o/
        voiceConnection = session.VoiceConnections[guild.ID]
    }

    // Check what the user wants from us
    switch command {
    case "leave":
        session.ChannelMessageSend(channel.ID, "OK, bye :wave:")
        voiceConnection.Close()
        break

    case "play":
        break

    case "stop":
        break

    case "skip":
        break

    case "clear":
        break

    case "playing", "np":
        break

    case "list":
        break

    case "random":
        break

    case "add":
        break
    }
}

func resolveVoiceChannel(user *discordgo.User, guild *discordgo.Guild, session *discordgo.Session) *discordgo.Channel {
    for _, vs := range guild.VoiceStates {
        if vs.UserID == user.ID {
            channel, err := session.Channel(vs.ChannelID)
            if err != nil {
                panic(err)
            }

            return channel
        }
    }

    return nil
}