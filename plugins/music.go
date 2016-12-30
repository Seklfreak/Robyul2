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
    Logger "github.com/sn0w/Karen/logger"
    "strconv"
    "encoding/binary"
    "io"
)

// Define callbacks
type Callback func()

// Define control messages
type controlMessage int

const (
    Skip controlMessage = iota
    Pause
    Resume
)

// A connection to one guild's channel
type GuildConnection struct {
    // Controller channel for Skip/Pause/Resume
    controller chan controlMessage

    // Closer channel for Stop commands
    closer     chan struct{}

    // Slice of waiting songs
    playlist   []Song

    // Slice of waiting but unprocessed songs
    queue      []Song

    // Whether this is playing music or not
    playing    bool
}

// Helper to generate a guild connection
func (gc *GuildConnection) Alloc() *GuildConnection {
    gc.controller = make(chan controlMessage)
    gc.closer = make(chan struct{})
    gc.playlist = []Song{}
    gc.queue = []Song{}
    gc.playing = false
    return gc
}

// Define a song
type Song struct {
    ID        string `gorethink:"id,omitempty"`
    AddedBy   string `gorethink:"added_by"`
    Title     string `gorethink:"title"`
    URL       string `gorethink:"url"`
    Duration  int    `gorethink:"duration"`
    Processed bool   `gorethink:"processed"`
    Path      string `gorethink:"path"`
}

// Plugin class
type Music struct {
    guildConnections map[string]*GuildConnection
    enabled          bool
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
        "mdev",
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

        // Allocate connections map
        m.guildConnections = make(map[string]*GuildConnection)

        // Start loop that processes videos in background
        go m.processorLoop()
    } else {
        fmt.Println("=> Not Found. Music disabled!")
    }
}

func (m *Music) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    // Only continue if enabled
    if !m.enabled {
        return
    }

    // Store channel ref
    channel, err := session.Channel(msg.ChannelID)
    helpers.Relax(err)

    // Store guild ref
    guild, err := session.Guild(channel.GuildID)
    helpers.Relax(err)

    // Store voice channel ref
    vc := m.resolveVoiceChannel(msg.Author, guild, session)

    // Store voice connection ref
    var voiceConnection *discordgo.VoiceConnection

    // Store fingerprint (guild:channel)
    var fingerprint string

    // Check if the user is connected to voice at all
    if vc == nil {
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
        // We are connected \o/
        // Save the ref for easier access
        voiceConnection = session.VoiceConnections[guild.ID]

        // Generate fingerprint
        fingerprint = voiceConnection.GuildID + ":" + voiceConnection.ChannelID

        // Allocate guildConnection if not present
        if m.guildConnections[fingerprint] == nil {
            m.guildConnections[fingerprint] = (&GuildConnection{}).Alloc()
        }
    }

    // Store pointers for easier access
    queue := &m.guildConnections[fingerprint].queue
    playlist := &m.guildConnections[fingerprint].playlist

    if command == "mdev" {
        session.ChannelMessageSend(
            channel.ID,
            fmt.Sprintf("```\n%#v\n```", m.guildConnections[fingerprint]),
        )
        return
    }

    // Check what the user wants from us
    switch command {
    case "leave":
        session.ChannelMessageSend(channel.ID, "OK, bye :frowning:")
        voiceConnection.Disconnect()
        break

    case "play":
        if len(*playlist) == 0 {
            if len(*queue) > 0 {
                session.ChannelMessageSend(channel.ID, "Please wait until your downloads are finished :wink:")
                return
            }

            session.ChannelMessageSend(channel.ID, "You should add some music first :thinking:")
            return
        }
        go m.startPlayer(fingerprint, voiceConnection, msg, session)
        break

    case "stop":
        session.ChannelMessageSend(channel.ID, ":stop_button: Track stopped")
        close(m.guildConnections[fingerprint].closer)
        break

    case "skip":
        session.ChannelMessageSend(channel.ID, ":track_next: Loading next track")
        m.guildConnections[fingerprint].controller <- Skip
        break

    case "clear":
        session.ChannelMessageSend(channel.ID, ":asterisk::stop_button: Playlist deleted. Music stopped.")
        close(m.guildConnections[fingerprint].closer)
        *playlist = []Song{}
        break

    case "playing", "np":
        session.ChannelMessageSend(
            channel.ID,
            ":musical_note: Currently playing `" + (*playlist)[0].Title + "`",
        )
        break

    case "list":
        if len(*playlist) == 0 {
            session.ChannelMessageSend(channel.ID, "Playlist is empty ¯\\_(ツ)_/¯")
            return
        }

        msg := ":musical_note: Playlist\n\n"
        msg += "Currently Playing: `" + (*playlist)[0].Title + "`\n"

        songs := [][]string{}
        for i, song := range *playlist {
            if i == 0 {
                continue
            }

            songs = append(songs, []string{strconv.Itoa(i) + ".", song.Title})
        }

        msg += helpers.DrawTable([]string{
            "#", "Title",
        }, songs)

        session.ChannelMessageSend(channel.ID, msg)
        break

    case "random":
        session.ChannelMessageSend(channel.ID, ":x: Not yet implemented")
        break

    case "add":
        session.ChannelTyping(channel.ID)
        content = strings.TrimSpace(content)

        // Check if the link has been cached
        cursor, err := rethink.Table("music").Filter(map[string]interface{}{"url":content}).Run(utils.GetDB())
        helpers.Relax(err)
        defer cursor.Close()

        var match Song
        err = cursor.One(&match)

        // Check if the song does not yet exists in the DB
        if err == rethink.ErrEmptyResult {
            // Link was not downloaded yet

            // Resolve the url through YTDL.
            ytdl := exec.Command("youtube-dl", "-J", content)
            yout, yerr := ytdl.Output()

            // If youtube-dl exits with 0 the link is valid
            if yerr != nil {
                session.ChannelMessageSend(channel.ID, "That looks like an invalid or unspported download link :frowning:")
                return
            }

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
            if match.Duration > int((65 * time.Minute).Seconds()) {
                session.ChannelMessageSend(channel.ID, "Whoa that's a big video!\nPlease use something shorter :neutral_face:")
                return
            }

            // Add to db
            _, e := rethink.Table("music").Insert(match).RunWrite(utils.GetDB())
            helpers.Relax(e)

            // Add to queue
            *queue = append(*queue, match)

            // Inform users
            session.ChannelMessageSend(
                channel.ID,
                "Added to queue. Music should start soon.\nLive progress at: <https://meetkaren.xyz/music>",
            )
            go m.waitForSong(channel.ID, fingerprint, match, msg, session)
            return
        } else if err != nil {
            // Unknown error. Should report that.
            helpers.Relax(err)
            return
        }

        // Song present in DB
        // Check if the match was already processed
        if match.Processed {
            // Was processed. Add to playlist.
            *playlist = append(*playlist, match)
            session.ChannelMessageSend(channel.ID, "Added from cache! :ok_hand:")
            return
        }

        // Not yet processed. Check if it's in the queue
        songPresent := false
        for _, song := range *queue {
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

        *queue = append(*queue, match)
        session.ChannelMessageSend(
            channel.ID,
            "Song was added to your queue but did not finish downloading yet. Wait a bit :wink:\nLive progress at: <https://meetkaren.xyz/music>",
        )
        go m.waitForSong(channel.ID, fingerprint, match, msg, session)
        break
    }
}

// Waits until the song is ready.
func (m *Music) waitForSong(channel string, fingerprint string, match Song, msg *discordgo.Message, session *discordgo.Session) {
    defer helpers.RecoverDiscord(session, msg)

    queue := &m.guildConnections[fingerprint].queue
    playlist := &m.guildConnections[fingerprint].playlist

    for {
        time.Sleep(1 * time.Second)

        cursor, err := rethink.Table("music").Filter(map[string]interface{}{"url":match.URL}).Run(utils.GetDB())
        helpers.Relax(err)

        var res Song
        err = cursor.One(&res)
        helpers.Relax(err)
        cursor.Close()

        if res.Processed {
            // Remove from queue
            for idx, song := range *queue {
                if song.URL == match.URL {
                    *queue = append((*queue)[:idx], (*queue)[idx + 1:]...)
                }
            }

            // Add to playlist
            *playlist = append(*playlist, res)

            session.ChannelMessageSend(channel, "`" + res.Title + "` finished downloading :smiley:")
            break
        }
    }
}

// Resolves a voice channel relative to a user id
func (m *Music) resolveVoiceChannel(user *discordgo.User, guild *discordgo.Guild, session *discordgo.Session) *discordgo.Channel {
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
func (m *Music) processorLoop() {
    defer helpers.Recover()

    // Define vars once and override later as needed
    var err error
    var cursor *rethink.Cursor

    for {
        // Sleep before next iteration
        time.Sleep(5 * time.Second)

        // Get unprocessed items
        cursor, err = rethink.Table("music").Filter(map[string]interface{}{"processed":false}).Run(utils.GetDB())
        helpers.Relax(err)

        // Get items
        var songs []Song
        err = cursor.All(&songs)
        helpers.Relax(err)
        cursor.Close()

        if err == rethink.ErrEmptyResult || len(songs) == 0 {
            continue
        }

        Logger.INF("[MUSIC] Found " + strconv.Itoa(len(songs)) + " unprocessed items!")

        // Loop through items
        for _, song := range songs {
            start := time.Now().Unix()

            name := utils.BtoA(song.URL)

            Logger.INF("[MUSIC] Downloading " + song.URL + " as " + name)

            // Download with youtube-dl
            ytdl := exec.Command(
                "youtube-dl",
                "--abort-on-error",
                "--no-color",
                "--no-playlist",
                "--max-filesize", "1024m",
                "-f", "bestaudio/best[height<=720][fps<=30]/best[height<=720]/[abr<=192]",
                "-x",
                "--audio-format", "wav",
                "--audio-quality", "0",
                "-o", name + ".%(ext)s",
                "--exec", "mv {} /srv/karen-data",
                song.URL,
            )
            ytdl.Stdout = os.Stdout
            ytdl.Stderr = os.Stderr
            helpers.Relax(ytdl.Start())
            helpers.Relax(ytdl.Wait())

            // WAV => RAW OPUS using DCA
            opusFile, err := os.Create("/srv/karen-data/" + name + ".opus")
            helpers.Relax(err)

            Logger.INF("[MUSIC] WAV => OPUS | " + name)
            dca := exec.Command("dca", "-raw", "-i", "/srv/karen-data/" + name + ".wav")
            dca.Stderr = os.Stderr
            dca.Stdout = opusFile
            helpers.Relax(dca.Start())
            helpers.Relax(dca.Wait())

            // Cleanup
            helpers.Relax(os.Remove("/srv/karen-data/" + name + ".wav"))

            // Mark as processed
            song.Processed = true
            song.Path = "/srv/karen-data/" + name + ".opus"

            // Update db
            _, err = rethink.Table("music").
                Filter(map[string]interface{}{"id":song.ID}).
                Update(song).
                RunWrite(utils.GetDB())
            helpers.Relax(err)

            end := time.Now().Unix()
            Logger.INF("[MUSIC] Conversion took " + strconv.Itoa(int(end - start)) + " seconds. | File: " + name)
        }
    }
}

func (m *Music) startPlayer(fingerprint string, vc *discordgo.VoiceConnection, msg *discordgo.Message, session *discordgo.Session) {
    defer helpers.RecoverDiscord(session, msg)

    // Ignore call if already playing
    if m.guildConnections[fingerprint].playing {
        return
    }

    // Get pointer to closer and controller via guildConnection
    closer := &m.guildConnections[fingerprint].closer
    controller := &m.guildConnections[fingerprint].controller
    playlist := &m.guildConnections[fingerprint].playlist

    // Start eventloop
    for {
        // Exit if the closer channel closes
        select {
        case <-(*closer):
            return
        default:
        }

        // Do nothing until voice is ready and songs are queued
        if !vc.Ready || len(*playlist) == 0 {
            time.Sleep(1 * time.Second)
            continue
        }

        // Mark guild as playing
        m.guildConnections[fingerprint].playing = true

        // Announce track
        session.ChannelMessageSend(msg.ChannelID, ":arrow_forward: Now playing `" + (*playlist)[0].Title + "`")

        // Send data to discord
        // Blocks until the song is done
        m.play(vc, *closer, *controller, (*playlist)[0])

        // Remove song from playlist
        *playlist = append((*playlist)[:0], (*playlist)[1:]...)
    }
}

func (m *Music) play(vc *discordgo.VoiceConnection, closer <-chan struct{}, controller <-chan controlMessage, song Song) {
    // Mark as speaking
    vc.Speaking(true)

    // Mark as not speaking as soon as we're done
    defer vc.Speaking(false)

    // Read file
    file, err := os.Open(song.Path)
    helpers.Relax(err)
    defer file.Close()

    // Allocate opus header buffer
    var opusLength int16

    // Start eventloop
    for {
        // Exit if the closer channel closes
        select {
        case <-closer:
            return
        default:
        }

        // Listen for commands from controller
        select {
        case ctl := <-controller:
            switch ctl {
            case Skip:
                return
            case Pause:
                // Wait until the controller asks to Skip or Resume
                wait := true
                for {
                    ctl := <-controller
                    switch ctl {
                    case Skip:
                        return
                    case Resume:
                        wait = false
                    }

                    if !wait {
                        break
                    }
                }
            default:
            }
        default:
        }

        // Read opus frame length
        err = binary.Read(file, binary.LittleEndian, &opusLength)
        if err == io.EOF || err == io.ErrUnexpectedEOF {
            return
        }
        helpers.Relax(err)

        // Read audio data
        opus := make([]byte, opusLength)
        err = binary.Read(file, binary.LittleEndian, &opus)
        helpers.Relax(err)

        // Send to discord
        vc.OpusSend <- opus
    }
}
