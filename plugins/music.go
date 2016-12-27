package plugins

import (
    "fmt"
    "github.com/Jeffail/gabs"
    "github.com/bwmarrin/discordgo"
    "github.com/sn0w/Karen/helpers"
    "github.com/sn0w/Karen/utils"
    rethink "gopkg.in/gorethink/gorethink.v3"
    "io/ioutil"
    "os"
    "os/exec"
    "regexp"
    "strings"
    "time"
)

type Music struct {
    // Maps refs to a song slice.
    // guild:voice_id -> []Song
    Playlist map[string][]Song
}

type Song struct {
    ID          string `gorethink:"id,omitempty"`
    AddedBy     string `gorethink:"added_by"`
    Title       string `gorethink:"title"`
    Description string `gorethink:"description"`
    URL         string `gorethink:"webpage_url"`
    Duration    int    `gorethink:"duration"`
    Processed   bool   `gorethink:"processed"`
    Path        string `gorethink:"path"`
}

var musicPluginEnabled = false

func (m Music) Commands() []string {
    return []string{
        "join",
        "leave",
        "play",
        "pause",
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

    if foundYTD && foundFFPROBE && foundFFMPEG {
        musicPluginEnabled = true
        fmt.Println("=> Found. Music enabled!")

        // Start loop that processes videos in background
        go processorLoop()
    } else {
        fmt.Println("=> Not Found. Music disabled!")
    }
}

func (m Music) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    if !musicPluginEnabled {
        return
    }

    // Channel ref
    channel, err := session.Channel(msg.ChannelID)
    helpers.Relax(err)

    // Guild ref
    guild, err := session.Guild(channel.GuildID)
    helpers.Relax(err)

    // Voice channel ref
    vc := resolveVoiceChannel(msg.Author, guild, session)

    // Voice connection ref
    var voiceConnection *discordgo.VoiceConnection

    // Check if the user is connected to voice at all
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
            helpers.Relax(err)

            if merr == nil {
                session.ChannelMessageEdit(channel.ID, message.ID, "Joined! :smiley:")
            }

            helpers.Relax(merr)
        } else {
            session.ChannelMessageSend(channel.ID, "You should join the channel I'm in or make me join yours before telling me to do stuff :thinking:")
        }

        return
    } else {
        // We are \o/
        // Save the ref for easier access
        voiceConnection = session.VoiceConnections[guild.ID]

        // Allocate playlist if not present
        id := voiceConnection.GuildID + ":" + voiceConnection.ChannelID
        if m.Playlist[id] == nil {
            m.Playlist[id] = make([]Song, 0)
        }
    }

    // Check what the user wants from us
    switch command {
    case "leave":
        session.ChannelMessageSend(channel.ID, "OK, bye :frowning:")
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
        content = strings.TrimSpace(content)

        // Resolve the url through YTDL.
        ytdl := exec.Command("youtube-dl", "-J", content)
        yout, yerr := ytdl.CombinedOutput()

        // If youtube-dl exits with 0 the link is valid
        if yerr != nil {
            session.ChannelMessageSend(channel.ID, "That looks like an invalid or unspported download link :frowning:")
            return
        }

        // Check if the link has been cached
        cursor, err := rethink.Table("music").Filter(map[string]interface{}{"url": strings.TrimSpace(content)}).Run(utils.GetDB())
        helpers.Relax(err)
        defer cursor.Close()

        var match Song
        err = cursor.One(&match)
        if err == rethink.ErrEmptyResult {
            // Link was not downloaded yet
            // Parse info JSON and allocate song object
            json, err := gabs.ParseJSON(yout)
            helpers.Relax(err)

            match = Song{
                AddedBy:     msg.Author.ID,
                Processed:   false,
                Title:       json.Path("title").Data().(string),
                Description: json.Path("description").Data().(string),
                Duration:    int(json.Path("duration").Data().(float64)),
                URL:         content,
            }

            // Check if the video is not too long
            if match.Duration > int((15 * time.Minute).Seconds()) {
                session.ChannelMessageSend(msg.ChannelID, "Whoa that's a big video!\nPlease use something shorter :neutral_face:")
                return
            }

            // Add to db
            _, e := rethink.Table("music").Insert(match).RunWrite(utils.GetDB())
            helpers.Relax(e)
        } else if err != nil {
            // Unknown error. Should report that.
            helpers.Relax(err)
            return
        }

        // Check if the match was already processed
        if match.Processed {
            // Was processed. Add from cache.
            session.ChannelMessageSend(channel.ID, "Added from cache! :smiley:")
            return
        }

        // Not yet processed. Let the users know.
        session.ChannelMessageSend(channel.ID, "Added to queue :ok_hand:\n You can see the live queue at <http://music.meetkaren.xyz/>")
        break
    }
}

func resolveVoiceChannel(user *discordgo.User, guild *discordgo.Guild, session *discordgo.Session) *discordgo.Channel {
    for _, vs := range guild.VoiceStates {
        if vs.UserID == user.ID {
            channel, err := session.Channel(vs.ChannelID)
            helpers.Relax(err)

            return channel
        }
    }

    return nil
}
