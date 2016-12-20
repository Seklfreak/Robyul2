package plugins

import (
    "github.com/bwmarrin/discordgo"
    "fmt"
    "os"
    "strings"
    "io/ioutil"
    "regexp"
    "os/exec"
    "github.com/sn0w/Karen/helpers"
    rethink "gopkg.in/gorethink/gorethink.v3"

    "github.com/sn0w/Karen/utils"
)

type Music struct{}

type Playlist struct {
    GuildID   string
    ChannelID string
    Songs     []Song
}

type Song struct {
    ID          string `gorethink:"id,omitempty"`
    AddedBy     string `gorethink:"addedBy"`
    Title       string `gorethink:"title"`
    Description string `gorethink:"description"`
    FullTitle   string `gorethink:"full_title"`
    URL         string `gorethink:"webpage_url"`
    Duration    int    `gorethink:"duration"`
    Processed   bool `gorethink:"processed"`
    Path        string `gorethink:"path"`
}

var musicPluginEnabled = false

func (m Music) Commands() []string {
    return []string{
        "join",
        "leave",
        "play",
        "stop",
        "skip",
        "clear",
        "add",
        "list",
        "playing",
        "np",
    }
}

func (m Music) Init(session *discordgo.Session) {
    foundYTD, foundFFPROBE, foundFFMPEG := false, false, false

    fmt.Println("=> Checking for youtube-dl, ffmpeg and ffprobe...")
    for _, path := range strings.Split(os.Getenv("PATH"), ":") {
        files, _ := ioutil.ReadDir(path)

        for _, file := range files {
            switch {
            case regexp.MustCompile(`youtube-dl.*`).Match([]byte(file.Name())):
                foundYTD = true
                break

            case regexp.MustCompile(`ffprobe.*`).Match([]byte(file.Name())):
                foundFFPROBE = true
                break

            case regexp.MustCompile(`ffmpeg.*`).Match([]byte(file.Name())):
                foundFFMPEG = true
                break
            }
        }
    }

    if (foundYTD && foundFFPROBE && foundFFMPEG) {
        musicPluginEnabled = true
        fmt.Println("=> Found. Music enabled!")
    } else {
        fmt.Println("=> Not Found. Music disabled!")
    }
}

func (m Music) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    if !musicPluginEnabled {
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
            message, merr := session.ChannelMessageSend(channel.ID, ":arrows_counterclockwise: Joining...")

            _, err := session.ChannelVoiceJoin(guild.ID, vc.ID, false, false)
            if err != nil {
                panic(err)
            }

            if merr == nil {
                session.ChannelMessageEdit(channel.ID, message.ID, "Joined! :slight_smile:")
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
        voiceConnection.Disconnect()
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
        content = strings.Trim(content, " ")

        // Resolve the url through YTDL.
        ytdl := exec.Command("youtube-dl", "-g", content)
        yerr := ytdl.Run()

        // If youtube-dl exits with 0 the link is valid
        if yerr != nil {
            session.ChannelMessageSend(channel.ID, "That looks like an invalid or unspported download link :frowning:")
            return
        }

        // check if the link has been cached
        cursor, err := rethink.Table("music").Filter(map[string]interface{}{"url":content}).Run(utils.GetDB())
        defer cursor.Close()
        helpers.Relax(err)

        var matches []Song
        err = cursor.All(&matches)
        if err == rethink.ErrEmptyResult {
            // First song ever O-O
            // Ignore error
        } else if err != nil {
            // Error. Should report that.
            helpers.Relax(err)
            return
        }

        for _, match := range matches {
            // Check if url is present
            if match.URL == content {
                // Check if the match was processed
                if match.Processed {
                    session.ChannelMessageSend(channel.ID, "Added from cache! :smiley:")
                    return
                } else {
                    session.ChannelMessageSend(channel.ID, "This song is:ok_hand:\n You can see the live queue at <http://music.meetkaren.xyz/>")
                }
            }
        }

        // add to queue otherwise
        _, e := rethink.Table("music").Insert(Song{
            Processed: false,
            URL: content,
        }).RunWrite(utils.GetDB())
        helpers.Relax(e)

        session.ChannelMessageSend(channel.ID, "Added to queue :ok_hand:\n You can see the live queue at <http://music.meetkaren.xyz/>")
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
