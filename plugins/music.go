package plugins

import (
    "encoding/binary"
    "fmt"
    "github.com/Jeffail/gabs"
    "github.com/bwmarrin/discordgo"
    "git.lukas.moe/sn0w/Karen/helpers"
    Logger "git.lukas.moe/sn0w/Karen/logger"
    rethink "gopkg.in/gorethink/gorethink.v3"
    "io"
    "io/ioutil"
    "math/rand"
    "os"
    "os/exec"
    "regexp"
    "strconv"
    "strings"
    "time"
    "bufio"
)

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
    gc.CreateChannels()

    gc.playlist = []Song{}
    gc.queue = []Song{}
    gc.playing = false

    return gc
}

func (gc *GuildConnection) CloseChannels() {
    close(gc.closer)
    close(gc.controller)
}

func (gc *GuildConnection) CreateChannels() {
    gc.closer = make(chan struct{})
    gc.controller = make(chan controlMessage)
}

func (gc *GuildConnection) RecreateChannels() {
    gc.CloseChannels()
    gc.CreateChannels()
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
        "next",
        "playing",
        "np",

        "add",
        "list",
        "playlist",
        "random",
        "rand",
        "search",
        "find",

        "mdev",
    }
}

func (m *Music) Init(session *discordgo.Session) {
    m.enabled = false
    foundYTD, foundFFPROBE, foundFFMPEG := false, false, false

    Logger.PLUGIN.L("music", "Checking for youtube-dl, ffmpeg and ffprobe...")
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
        Logger.PLUGIN.L("music", "Found. Music enabled!")

        // Allocate connections map
        m.guildConnections = make(map[string]*GuildConnection)

        // Start loop that processes videos in background
        Logger.PLUGIN.L("music", "Starting async processor loop")
        go m.processorLoop()

        // Start janitor that removes files which are not tracked in the DB
        Logger.PLUGIN.L("music", "Starting async janitor")
        go m.janitor()
    } else {
        Logger.PLUGIN.L("music", "Not Found. Music disabled!")
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

    // Store voice connection ref (deferred)
    var voiceConnection *discordgo.VoiceConnection

    // Store fingerprint (guild:channel) (deferred)
    var fingerprint string

    // Check if the user is connected to voice at all
    if vc == nil {
        session.ChannelMessageSend(channel.ID, "You're either not in the voice-chat or I can't see you :neutral_face:")
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

                // Make guild connection
                fingerprint = guild.ID + ":" + session.VoiceConnections[guild.ID].ChannelID
                if m.guildConnections[fingerprint] == nil {
                    m.guildConnections[fingerprint] = (&GuildConnection{}).Alloc()
                }

                // Start auto-leaver
                go m.autoLeave(guild.ID, channel.ID, session, m.guildConnections[fingerprint].closer)
            }

            helpers.Relax(merr)
        } else {
            session.ChannelMessageSend(channel.ID, "You should join the channel I'm in or make me join yours before telling me to do stuff :thinking:")
        }

        return
    }

    // We are connected \o/
    // Save the ref for easier access
    voiceConnection = session.VoiceConnections[guild.ID]

    // Generate fingerprint
    fingerprint = voiceConnection.GuildID + ":" + voiceConnection.ChannelID

    // Store gc pointer for easier access
    queue := &m.guildConnections[fingerprint].queue
    playlist := &m.guildConnections[fingerprint].playlist

    // Dev command that shows the guild connection
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
        voiceConnection.Disconnect()
        m.guildConnections[fingerprint].CloseChannels()
        delete(m.guildConnections, fingerprint)
        session.ChannelMessageSend(channel.ID, "OK, bye :frowning:")
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

        if m.guildConnections[fingerprint].playing {
            m.guildConnections[fingerprint].controller <- Resume
        } else {
            go m.startPlayer(fingerprint, voiceConnection, msg, session)
        }
        break

    case "pause":
        m.guildConnections[fingerprint].controller <- Pause
        break

    case "stop":
        m.guildConnections[fingerprint].RecreateChannels()
        m.guildConnections[fingerprint].playing = false
        *playlist = []Song{}

        session.ChannelMessageSend(channel.ID, ":stop_button: Playback stopped. (Playlist is now empty)")
        break

    case "skip", "next":
        m.guildConnections[fingerprint].controller <- Skip
        break

    case "playing", "np":
        if !m.guildConnections[fingerprint].playing {
            session.ChannelMessageSend(
                channel.ID,
                "Why do you ask me? You didn't even start playback yet!",
            )
            return
        }

        if len(*playlist) == 0 {
            session.ChannelMessageSend(
                channel.ID,
                "Seems like you cleared the Playlist. \n I don't know what's playing :neutral_face:",
            )
            return
        }

        session.ChannelMessageSend(
            channel.ID,
            ":musical_note: Currently playing `" + (*playlist)[0].Title + "`",
        )
        break

    case "list", "playlist":
        if len(*playlist) == 0 {
            session.ChannelMessageSend(channel.ID, "Playlist is empty ¯\\_(ツ)_/¯")
            return
        }

        msg := ":musical_note: Playlist\n\n"

        songs := [][]string{}
        for i, song := range *playlist {
            num := strconv.Itoa(i + 1) + "."

            if i == 0 && m.guildConnections[fingerprint].playing {
                num = "[>] " + num
            }

            songs = append(songs, []string{
                num,
                helpers.SecondsToDuration(song.Duration),
                song.Title,
            })
        }

        msg += helpers.DrawTable([]string{
            "#", "Duration", "Title",
        }, songs)

        session.ChannelMessageSend(channel.ID, msg)
        break

    case "random", "rand":
        cursor, err := rethink.Table("music").Filter(map[string]interface{}{"processed": true}).Run(helpers.GetDB())
        helpers.Relax(err)
        defer cursor.Close()

        var matches []Song
        err = cursor.All(&matches)
        helpers.Relax(err)

        match := matches[rand.Intn(len(matches))]
        *playlist = append(*playlist, match)

        session.ChannelMessageSend(channel.ID, ":ballot_box_with_check: Added `" + match.Title + "`")
        break

    case "add":
        session.ChannelTyping(channel.ID)
        content = strings.TrimSpace(content)

        // Trim masquerade if present
        contentRune := []rune(content)
        if contentRune[0] == '<' && contentRune[len(contentRune) - 1] == '>' {
            content = string(contentRune[1:len(contentRune) - 1])
        }

        // Check if the url is a playlist
        if strings.Contains(content, "list") ||
            strings.Contains(content, "/set") ||
            strings.Contains(content, "/mix") {
            session.ChannelMessageSend(channel.ID, "Sorry but playlists are not supported :neutral_face:")
            return
        }

        // Check if the link has been cached
        cursor, err := rethink.Table("music").Filter(map[string]interface{}{"url": content}).Run(helpers.GetDB())
        helpers.Relax(err)
        defer cursor.Close()

        var match Song
        err = cursor.One(&match)

        // Check if the song does not yet exists in the DB
        if err == rethink.ErrEmptyResult {
            // Link was not downloaded yet

            // Resolve the url through YTDL.
            ytdl := exec.Command("youtube-dl", "-J", "--flat-playlist", content)
            yout, yerr := ytdl.Output()

            // If youtube-dl exits with 0 the link is valid
            if yerr != nil {
                session.ChannelMessageSend(channel.ID, "That looks like an invalid or unspported download link :frowning:")
                return
            }

            // Parse info JSON and allocate song object
            json, err := gabs.ParseJSON(yout)
            helpers.Relax(err)

            // Exit if the link is a live stream
            // (archived live streams are allowed though)
            if json.ExistsP("is_live") && json.Path("is_live").Data().(bool) {
                session.ChannelMessageSend(channel.ID, "Livestreams are not supported :neutral_face:")
                return
            }

            // Exit if the link is a playlist
            if json.ExistsP("_type") && json.Path("_type").Data() != nil {
                switch json.Path("_type").Data().(string) {
                case "playlist":
                    session.ChannelMessageSend(channel.ID, "Sorry but playlists are not supported :neutral_face:")
                    return
                }
            }

            match = Song{
                AddedBy:   msg.Author.ID,
                Processed: false,
                Title:     json.Path("title").Data().(string),
                Duration:  int(json.Path("duration").Data().(float64)),
                URL:       content,
            }

            // Check if the video is not too long
            // Bot owners may bypass this
            if !helpers.IsBotAdmin(msg.Author.ID) && match.Duration > int((65 * time.Minute).Seconds()) {
                session.ChannelMessageSend(channel.ID, "Whoa `" + match.Title + "` is a big video!\nPlease use something shorter :neutral_face:")
                return
            }

            // Add to db
            _, e := rethink.Table("music").Insert(match).RunWrite(helpers.GetDB())
            helpers.Relax(e)

            // Add to queue
            *queue = append(*queue, match)

            // Inform users
            session.ChannelMessageSend(
                channel.ID,
                "`" + match.Title + "` was added to your download-queue.\nLive progress at: <https://meetkaren.xyz/music>",
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
                "`" + match.Title + "` is already in your download-queue.\nLive progress at: <https://meetkaren.xyz/music>",
            )
            return
        }

        *queue = append(*queue, match)
        session.ChannelMessageSend(
            channel.ID,
            "`" + match.Title + "` was added to your download-queue.\nLive progress at: <https://meetkaren.xyz/music>",
        )
        go m.waitForSong(channel.ID, fingerprint, match, msg, session)
        break
    case "search", "find":
        if len(content) < 4 {
            session.ChannelMessageSend(
                channel.ID,
                "Kinda short search term :thinking:\nTry something longer!",
            )
            return
        }

        term := "(?i).*" + strings.Join(strings.Split(content, " "), ".*") + ".*"

        // @formatter:off
		cursor, err := rethink.
			Table("music").
			Filter(map[string]interface{}{"processed": true}).
			Filter(rethink.Row.Field("title").Match(term)).
			Run(helpers.GetDB())
		// @formatter:on

        helpers.Relax(err)
        defer cursor.Close()

        var results []Song
        err = cursor.All(&results)
        if err == rethink.ErrEmptyResult || len(results) == 0 {
            session.ChannelMessageSend(
                channel.ID,
                "No results :frowning:",
            )
            return
        }
        helpers.Relax(err)

        headers := []string{"Title", "Link"}
        rows := make([][]string, len(results))
        for i, result := range results {
            rows[i] = []string{result.Title, result.URL}
        }

        session.ChannelMessageSend(channel.ID, ":mag: Search results:\n" + helpers.DrawTable(headers, rows))
        break
    }
}

// Waits until the song is ready and notifies.
func (m *Music) waitForSong(channel string, fingerprint string, match Song, msg *discordgo.Message, session *discordgo.Session) {
    defer helpers.RecoverDiscord(msg)

    queue := &m.guildConnections[fingerprint].queue
    playlist := &m.guildConnections[fingerprint].playlist

    for {
        time.Sleep(1 * time.Second)

        cursor, err := rethink.Table("music").Filter(map[string]interface{}{"url": match.URL}).Run(helpers.GetDB())
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

            session.ChannelMessageSend(channel, ":ballot_box_with_check: `" + res.Title + "` finished downloading :smiley:")
            break
        }
    }
}

// Resolves a voice channel relative to a user id
func (m *Music) resolveVoiceChannel(user *discordgo.User, guild *discordgo.Guild, session *discordgo.Session) *discordgo.Channel {
    for _, vs := range guild.VoiceStates {
        if vs.UserID == user.ID {
            channel, err := session.Channel(vs.ChannelID)
            if err != nil {
                return nil
            }

            return channel
        }
    }

    return nil
}

// processorLoop is a endless coroutine that checks for new songs and spawns youtube-dl as needed
func (m *Music) processorLoop() {
    defer helpers.Recover()

    // Define vars once and override later as needed
    var err error
    var cursor *rethink.Cursor

    for {
        // Sleep before next iteration
        time.Sleep(5 * time.Second)

        // Get unprocessed items
        cursor, err = rethink.Table("music").Filter(map[string]interface{}{"processed": false}).Run(helpers.GetDB())
        helpers.Relax(err)

        // Get song objects
        var songs []Song
        err = cursor.All(&songs)
        helpers.Relax(err)
        cursor.Close()

        // If there are no results skip this iteration
        if err == rethink.ErrEmptyResult || len(songs) == 0 {
            continue
        }

        Logger.INFO.L("music", "Found " + strconv.Itoa(len(songs)) + " unprocessed items!")

        // Loop through songs
        for _, song := range songs {
            start := time.Now().Unix()

            name := helpers.BtoA(song.URL)

            Logger.INFO.L("music", "Downloading " + song.URL + " as " + name)

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

            // WAV => RAW OPUS
            cstart := time.Now().Unix()
            Logger.INFO.L("music", "WAV => ROPUS | " + name)

            // Create file
            opusFile, err := os.Create("/srv/karen-data/" + name + ".ro")
            helpers.Relax(err)
            writer := bufio.NewWriter(opusFile)

            // Read wav
            cat := exec.Command("cat", "/srv/karen-data/" + name + ".wav")

            // Convert wav to raw opus
            ro := exec.Command("ropus")

            // Pipe streams
            r, w := io.Pipe()
            cat.Stdout = w
            ro.Stdin = r
            ro.Stdout = writer

            // Run commands
            helpers.Relax(cat.Start())
            helpers.Relax(ro.Start())

            // Wait until cat loaded the whole file
            helpers.Relax(cat.Wait())
            w.Close()

            // Wait until the file is converted
            helpers.Relax(ro.Wait())
            r.Close()
            opusFile.Close()
            cend := time.Now().Unix()

            // Cleanup
            helpers.Relax(os.Remove("/srv/karen-data/" + name + ".wav"))

            // Mark as processed
            song.Processed = true
            song.Path = "/srv/karen-data/" + name + ".ro"

            // Update db
            _, err = rethink.Table("music").
                Filter(map[string]interface{}{"id": song.ID}).
                Update(song).
                RunWrite(helpers.GetDB())
            helpers.Relax(err)

            end := time.Now().Unix()
            Logger.INFO.L(
                "music",
                "Download took " + strconv.Itoa(int(end - start)) + "s " +
                    "| Conversion took " + strconv.Itoa(int(cend - cstart)) + "s | File: " + name,
            )
        }
    }
}

// startPlayer is a helper to call play()
func (m *Music) startPlayer(fingerprint string, vc *discordgo.VoiceConnection, msg *discordgo.Message, session *discordgo.Session) {
    defer helpers.RecoverDiscord(msg)

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

        // Send data to discord
        // Blocks until the song is done
        m.play(vc, *closer, *controller, (*playlist)[0], msg, session, fingerprint)

        // Remove song from playlist if it's not empty
        if len(*playlist) > 0 {
            *playlist = append((*playlist)[:0], (*playlist)[1:]...)
        }
    }
}

// @formatter:off
// play is responsible for streaming the OPUS data to discord
func (m *Music) play(
    vc *discordgo.VoiceConnection,
    closer <-chan struct{},
    controller <-chan controlMessage,
    song Song,
    msg *discordgo.Message,
    session *discordgo.Session,
    fingerprint string,
) {
    // @formatter:on

    // Announce track
    session.ChannelMessageSend(msg.ChannelID, ":arrow_forward: Now playing `" + song.Title + "`")

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
                wait := true
                iteration := 0
                session.ChannelMessageSend(msg.ChannelID, ":pause_button: Track paused")
                for {
                    // Read from controller channel
                    ctl := <-controller
                    switch ctl {
                    case Skip:
                        return
                    case Resume:
                        wait = false
                    }

                    // If Skip or Resume was received end lock
                    if !wait {
                        break
                    }

                    // Sleep for 0.5s until next check to reduce CPU load
                    iteration++
                    time.Sleep(500 * time.Millisecond)
                }
                session.ChannelMessageSend(msg.ChannelID, ":play_pause: Track resumed")
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
        if err == io.EOF || err == io.ErrUnexpectedEOF {
            return
        }
        helpers.Relax(err)

        // Send to discord
        vc.OpusSend <- opus
    }
}

// janitor watches the data dir and deletes files that don't belong there
func (m *Music) janitor() {
    defer helpers.Recover()

    for {
        // Query for songs
        cursor, err := rethink.Table("music").Run(helpers.GetDB())
        helpers.Relax(err)

        // Get items
        var songs []Song
        err = cursor.All(&songs)
        helpers.Relax(err)
        cursor.Close()

        // If there are no songs continue
        if err == rethink.ErrEmptyResult || len(songs) == 0 {
            continue
        }

        // Remove files that have to DB entry
        dir, err := ioutil.ReadDir("/srv/karen-data")
        helpers.Relax(err)

        for _, file := range dir {
            foundFile := false

            for _, song := range songs {
                if strings.Contains("/srv/karen-data/" + file.Name(), helpers.BtoA(song.URL)) {
                    foundFile = true
                    break
                }
            }

            if !foundFile {
                Logger.INFO.L("music", "[JANITOR] Removing " + file.Name())
                err = os.Remove("/srv/karen-data/" + file.Name())
                helpers.Relax(err)
            }
        }

        time.Sleep(30 * time.Second)
    }
}

// autoLeave disconnects from VC if the users leave anf forget to !leave
func (m *Music) autoLeave(guildId string, channelId string, session *discordgo.Session, closer <-chan struct{}) {
    defer helpers.Recover()

    for {
        // Wait for 5 seconds
        time.Sleep(5 * time.Second)

        // Exit if the closer channel closes
        select {
        case <-closer:
            return
        default:
        }

        voiceChannel := session.VoiceConnections[guildId]
        guild, err := session.Guild(guildId)
        if err != nil {
            continue
        }

        fellows := 0
        for _, state := range guild.VoiceStates {
            if state.ChannelID == voiceChannel.ChannelID {
                fellows++
            }
        }

        if fellows == 1 {
            fingerprint := voiceChannel.GuildID + ":" + voiceChannel.ChannelID

            session.ChannelMessageSend(channelId, "Where did everyone go? :frowning:")

            voiceChannel.Disconnect()
            (*m.guildConnections[fingerprint]).CloseChannels()
            delete(m.guildConnections, fingerprint)
            break
        }
    }
}
