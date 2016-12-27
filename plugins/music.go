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

type Callback func()

type Music struct {
    playlist map[string][]Song
    queue    map[string][]Song
    enabled  bool
}

type Song struct {
    ID        string `gorethink:"id,omitempty"`
    AddedBy   string `gorethink:"added_by"`
    Title     string `gorethink:"title"`
    URL       string `gorethink:"url"`
    Duration  int    `gorethink:"duration"`
    Processed bool   `gorethink:"processed"`
    Path      string `gorethink:"path"`
}

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
        "mdbg",
    }
}

func (m *Music) Init(session *discordgo.Session) {
    m.enabled = false
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
        m.enabled = true
        fmt.Println("=> Found. Music enabled!")

        // Allocate maps
        m.playlist = make(map[string][]Song)
        m.queue = make(map[string][]Song)

        // Start loop that processes videos in background
        go processorLoop()
    } else {
        fmt.Println("=> Not Found. Music disabled!")
    }
}

func (m *Music) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    if !m.enabled {
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

    // Fingerprint (guild:channel)
    var fingerprint string

    if command == "mdbg" {
        session.ChannelMessageSend(
            channel.ID,
            fmt.Sprintf("Queue:\n```\n%#v\n```", m.queue),
        )
        session.ChannelMessageSend(
            channel.ID,
            fmt.Sprintf("Playlist:\n```\n%#v\n```", m.playlist),
        )
        return
    }

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

        // Generate fingerprint
        fingerprint = voiceConnection.GuildID + ":" + voiceConnection.ChannelID

        // Allocate playlist if not present
        if m.playlist[fingerprint] == nil {
            m.playlist[fingerprint] = make([]Song, 0)
        }

        // Allocate queue if not present
        if m.queue[fingerprint] == nil {
            m.queue[fingerprint] = make([]Song, 0)
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
        session.ChannelTyping(channel.ID)

        content = strings.TrimSpace(content)

        // Resolve the url through YTDL.
        ytdl := exec.Command("youtube-dl", "-J", content)
        yout, yerr := ytdl.Output()

        // If youtube-dl exits with 0 the link is valid
        if yerr != nil {
            session.ChannelMessageSend(channel.ID, "That looks like an invalid or unspported download link :frowning:")
            return
        }

        // Check if the link has been cached
        cursor, err := rethink.Table("music").Filter(map[string]interface{}{"url":content}).Run(utils.GetDB())
        helpers.Relax(err)
        defer cursor.Close()

        var match Song
        err = cursor.One(&match)

        // Check if the song does not yet exists in the DB
        if err == rethink.ErrEmptyResult {
            // Link was not downloaded yet
            // Parse info JSON and allocate song object
            json, err := gabs.ParseJSON(yout)
            helpers.Relax(err)

            match = Song{
                AddedBy:     msg.Author.ID,
                Processed:   false,
                Title:       json.Path("title").Data().(string),
                Duration:    int(json.Path("duration").Data().(float64)),
                URL:         content,
            }

            // Check if the video is not too long
            if match.Duration > int((15 * time.Minute).Seconds()) {
                session.ChannelMessageSend(channel.ID, "Whoa that's a big video!\nPlease use something shorter :neutral_face:")
                return
            }

            // Add to db
            _, e := rethink.Table("music").Insert(match).RunWrite(utils.GetDB())
            helpers.Relax(e)

            // Add to queue
            m.queue[fingerprint] = append(m.queue[fingerprint], match)

            // Inform users
            session.ChannelMessageSend(
                channel.ID,
                "Added to queue. Music should start soon.\nLive progress at: <https://meetkaren.xyz/music>",
            )
            go m.waitForSong(channel.ID, fingerprint, match, session)
            return
        } else if err != nil {
            // Unknown error. Should report that.
            helpers.Relax(err)
            return
        }

        // Song present in DB
        // Check if the match was already processed
        if match.Processed {
            // Was processed. Add from cache.
            session.ChannelMessageSend(channel.ID, "Added from cache! :ok_hand:")
            return
        }

        // Not yet processed. Check if it's in the queue
        songPresent := false
        for _, song := range m.queue[fingerprint] {
            if match.URL == song.URL {
                songPresent = true
                break
            }
        }

        if songPresent {
            session.ChannelMessageSend(
                channel.ID,
                "That song is already in your queue.\nLive progress at: <https://meetkaren.xyz/music>",
            )
            return
        }

        m.queue[fingerprint] = append(m.queue[fingerprint], match)
        session.ChannelMessageSend(
            channel.ID,
            "Song was added to your queue but did not finish downloading yet. Wait a bit :wink:\nLive progress at: <https://meetkaren.xyz/music>",
        )
        go m.waitForSong(channel.ID, fingerprint, match, session)
        break
    }
}

// Waits until the song is ready.
func (m *Music) waitForSong(channel string, fingerprint string, match Song, session *discordgo.Session) {
    for {
        time.Sleep(10 * time.Second)

        cursor, err := rethink.Table("music").Filter(map[string]interface{}{"url":match.URL}).Run(utils.GetDB())
        helpers.Relax(err)

        var res Song
        err = cursor.One(&res)
        helpers.Relax(err)
        cursor.Close()

        if res.Processed {
            session.ChannelMessageSend(channel, "`" + match.Title + "` finished downloading :smiley:")
            break
        }
    }
}

// Resolves a voice channel relative to a user id
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

// Endless coroutine that checks for new songs and spawns youtube-dl as needed
func processorLoop() {
    // Define vars once and override later as needed
    var err error
    var cursor *rethink.Cursor

    for {
        // Sleep before next iteration
        time.Sleep(10 * time.Second)
        continue

        // Get unprocessed items
        cursor, err = rethink.Table("music").Filter(map[string]interface{}{"processed":false}).Run(utils.GetDB())
        helpers.Relax(err)

        // Get items
        var songs []Song
        err = cursor.All(&songs)
        cursor.Close()

        if err == rethink.ErrEmptyResult {
            continue
        }

        // Loop through items
        for _, song := range songs {
            ytdl := exec.Command(
                "youtube-dl",
                "--abort-on-error",
                "--no-color",
                "--no-playlist",
                "--max-filesize",
                "512m",
                "-f",
                "[height<=480][abr<=192][ext=mp4]",
                "-o",
                ".%(ext)s",
                song.URL,
            )
            _, _ = ytdl.CombinedOutput()
        }

        // Update db
        _, err = rethink.Table("music").Insert(songs).RunWrite(utils.GetDB())
        helpers.Relax(err)
    }
}
