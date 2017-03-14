package plugins

import (
    "bufio"
    "encoding/binary"
    "git.lukas.moe/sn0w/Karen/cache"
    "git.lukas.moe/sn0w/Karen/helpers"
    "git.lukas.moe/sn0w/Karen/logger"
    "git.lukas.moe/sn0w/radio-b"
    "github.com/bwmarrin/discordgo"
    "github.com/getsentry/raven-go"
    "github.com/gorilla/websocket"
    "io"
    "net/url"
    "os/exec"
    "strings"
    "sync"
    "time"
)

// ---------------------------------------------------------------------------------------------------------------------
// Helper structs for managing and closing voice connections
// ---------------------------------------------------------------------------------------------------------------------
var RadioChan *radio.Radio

var RadioCurrentMeta RadioMeta

type RadioMetaContainer struct {
    SongId      float64 `json:"song_id,omitempty"`
    ArtistName  string  `json:"artist_name"`
    SongName    string  `json:"song_name"`
    AnimeName   string  `json:"anime_name,omitempty"`
    RequestedBy string  `json:"requested_by,omitempty"`
    Listeners   float64 `json:"listeners,omitempty"`
}

type RadioMeta struct {
    RadioMetaContainer

    Last       RadioMetaContainer `json:"last,omitempty"`
    SecondLast RadioMetaContainer `json:"second_last,omitempty"`
}

type RadioGuildConnection struct {
    sync.RWMutex

    // Closer channel for stop commands
    closer chan struct{}

    // Marks this guild as streaming music
    streaming bool
}

func (r *RadioGuildConnection) Alloc() *RadioGuildConnection {
    r.Lock()
    r.streaming = false
    r.closer = make(chan struct{})
    r.Unlock()

    return r
}

func (r *RadioGuildConnection) Close() {
    r.Lock()
    close(r.closer)
    r.streaming = false
    r.Unlock()
}

// ---------------------------------------------------------------------------------------------------------------------
// Actual plugin implementation
// ---------------------------------------------------------------------------------------------------------------------
type ListenDotMoe struct {
    connections map[string]*RadioGuildConnection
}

func (l *ListenDotMoe) Commands() []string {
    return []string{
        "moe",
        "lm",
    }
}

func (l *ListenDotMoe) Init(session *discordgo.Session) {
    l.connections = make(map[string]*RadioGuildConnection)

    go l.streamer()
    go l.tracklistWorker()
}

func (l *ListenDotMoe) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    // Sanitize subcommand
    content = strings.TrimSpace(content)

    // Store channel ref
    channel, err := cache.Channel(msg.ChannelID)
    helpers.Relax(err)

    // Only continue if the voice is available
    if !helpers.VoiceIsFreeOrOccupiedBy(channel.GuildID, "listen.moe") {
        helpers.VoiceSendStatus(channel.ID, channel.GuildID, session)
        return
    }

    // Store guild ref
    guild, err := session.Guild(channel.GuildID)
    helpers.Relax(err)

    // Store voice channel ref
    vc := l.resolveVoiceChannel(msg.Author, guild, session)

    // Store voice connection ref (deferred)
    var voiceConnection *discordgo.VoiceConnection

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
        if content == "join" || content == "j" {
            if helpers.IsAdmin(msg) {
                helpers.VoiceOccupy(guild.ID, "listen.moe")

                message, merr := session.ChannelMessageSend(channel.ID, ":arrows_counterclockwise: Joining...")

                voiceConnection, err = session.ChannelVoiceJoin(guild.ID, vc.ID, false, false)
                if err != nil {
                    helpers.VoiceFree(guild.ID)
                    helpers.Relax(err)
                }

                if merr == nil {
                    session.ChannelMessageEdit(channel.ID, message.ID, "Joined!\nThe radio should start playing shortly c:")

                    l.connections[guild.ID] = (&RadioGuildConnection{}).Alloc()

                    go l.pipeStream(guild.ID, session)
                    return
                }

                helpers.Relax(merr)
            } else {
                // @todo: remove this else and use returns instead.
                session.ChannelMessageSend(msg.ChannelID, helpers.GetText("admin.no_permission"))
            }
        } else {
            session.ChannelMessageSend(channel.ID, "You should join the channel I'm in or make me join yours before telling me to do stuff :thinking:")
        }

        return
    }

    // We are present.
    voiceConnection = session.VoiceConnections[vc.GuildID]

    // Check for other commands
    switch content {
    case "leave", "l":
        if helpers.IsAdmin(msg) {
            voiceConnection.Disconnect()

            l.connections[guild.ID].Close()
            delete(l.connections, guild.ID)

            helpers.VoiceFree(guild.ID)

            session.ChannelMessageSend(channel.ID, "OK, bye :frowning:")
            return
        }

        session.ChannelMessageSend(msg.ChannelID, helpers.GetText("admin.no_permission"))
        break

    case "playing", "np", "song", "title":
        fields := make([]*discordgo.MessageEmbedField, 1)
        fields[0] = &discordgo.MessageEmbedField{
            Name:   "Now Playing",
            Value:  RadioCurrentMeta.ArtistName + " " + RadioCurrentMeta.SongName,
            Inline: false,
        }

        if RadioCurrentMeta.AnimeName != "" {
            fields = append(fields, &discordgo.MessageEmbedField{
                Name: "Anime", Value: RadioCurrentMeta.AnimeName, Inline: false,
            })
        }

        if RadioCurrentMeta.RequestedBy != "" {
            fields = append(fields, &discordgo.MessageEmbedField{
                Name: "Requested by", Value: "[" + RadioCurrentMeta.RequestedBy + "](https://forum.listen.moe/u/" + RadioCurrentMeta.RequestedBy + ")", Inline: false,
            })
        }

        session.ChannelMessageSendEmbed(msg.ChannelID, &discordgo.MessageEmbed{
            Color: 0xEC1A55,
            Thumbnail: &discordgo.MessageEmbedThumbnail{
                URL: "http://i.imgur.com/Jp8N7YG.jpg",
            },
            Fields: fields,
            Footer: &discordgo.MessageEmbedFooter{
                Text: "powered by listen.moe (ﾉ◕ヮ◕)ﾉ*:･ﾟ✧",
            },
        })
        break
    }
}

// ---------------------------------------------------------------------------------------------------------------------
// Helper functions for managing voice connections
// ---------------------------------------------------------------------------------------------------------------------

// Resolves a voice channel relative to a user id
func (l *ListenDotMoe) resolveVoiceChannel(user *discordgo.User, guild *discordgo.Guild, session *discordgo.Session) *discordgo.Channel {
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

// ---------------------------------------------------------------------------------------------------------------------
// Helper functions for reading and piping listen.moe's stream to multiple targets at once
// ---------------------------------------------------------------------------------------------------------------------

// @todo:
// remove FFMPEG and ROPUS dependency.
// should be doable by correctly waiting until the ICECast headers are sent and then piping the raw bytes.
// until we reach this point the streamer routine consumes about 2% CPU power. All-The-Time!!!

func (l *ListenDotMoe) streamer() {
    for {
        logger.PLUGIN.L("listen_moe", "Allocating channels")
        RadioChan = radio.NewRadio()

        logger.PLUGIN.L("listen_moe", "Piping subprocesses")

        // Read stream with ffmpeg and turn it into PCM
        ffmpeg := exec.Command(
            "ffmpeg",
            "-i", "http://listen.moe:9999/stream",
            "-f", "s16le",
            "pipe:1",
        )
        ffout, err := ffmpeg.StdoutPipe()
        helpers.Relax(err)

        // Pipe FFMPEG to ropus to convert it to .ro format
        ropus := exec.Command("ropus")
        ropus.Stdin = ffout

        rout, err := ropus.StdoutPipe()
        helpers.Relax(err)

        // Run ffmpeg
        logger.PLUGIN.L("listen_moe", "Running FFMPEG")
        err = ffmpeg.Start()
        helpers.Relax(err)

        // Run ropus
        logger.PLUGIN.L("listen_moe", "Running ROPUS")
        err = ropus.Start()
        helpers.Relax(err)

        // Stream ropus to buffer
        robuf := bufio.NewReaderSize(rout, 16384)

        // Stream ropus output to discord
        var opusLength int16

        logger.PLUGIN.L("listen_moe", "Streaming :3")
        for {
            // Read opus frame length
            err = binary.Read(robuf, binary.LittleEndian, &opusLength)
            if err == io.EOF || err == io.ErrUnexpectedEOF {
                break
            }
            helpers.Relax(err)

            // Read audio data
            opus := make([]byte, opusLength)
            err = binary.Read(robuf, binary.LittleEndian, &opus)
            if err == io.EOF || err == io.ErrUnexpectedEOF {
                break
            }
            helpers.Relax(err)

            // Send to discord
            RadioChan.Broadcast(opus)
        }

        logger.ERROR.L("listen_moe", "Stream died :(")
        logger.PLUGIN.L("listen_moe", "Waiting for ffmpeg and ropus to die")
        ffmpeg.Wait()
        ropus.Wait()

        logger.PLUGIN.L("listen_moe", "Telling coroutines the bad news...")
        RadioChan.Alienate()

        logger.PLUGIN.L("listen_moe", "Will reconnect in 2 seconds")
        time.Sleep(2 * time.Second)
    }
}

func (l *ListenDotMoe) pipeStream(guildID string, session *discordgo.Session) {
    audioChan, id := RadioChan.Listen()
    vc := session.VoiceConnections[guildID]

    vc.Speaking(true)

    // Start eventloop
    for {
        connection, ok := l.connections[guildID]
        if !ok {
            break
        }

        // Exit if the closer channel dies
        select {
        case <-connection.closer:
            return
        default:
        }

        // Do nothing until voice is ready
        if !vc.Ready {
            time.Sleep(1 * time.Second)
            continue
        }

        // Send a frame to discord
        vc.OpusSend <- <-audioChan
    }

    vc.Speaking(false)

    RadioChan.Stop(id)
}

// ---------------------------------------------------------------------------------------------------------------------
// Helper functions for interacting with listen.moe's api
// ---------------------------------------------------------------------------------------------------------------------
func (l *ListenDotMoe) tracklistWorker() {
    for {
        c, _, err := websocket.DefaultDialer.Dial((&url.URL{
            Scheme: "wss",
            Host:   "listen.moe",
            Path:   "/api/v2/socket",
        }).String(), nil)

        if err != nil {
            raven.CaptureError(err, map[string]string{})
            time.Sleep(3 * time.Second)
            continue
        }

        c.WriteJSON(map[string]string{"token": helpers.GetConfig().Path("listen_moe").Data().(string)})
        helpers.Relax(err)

        for {
            time.Sleep(5 * time.Second)
            err := c.ReadJSON(&RadioCurrentMeta)

            if err == io.ErrUnexpectedEOF {
                continue
            }

            if err != nil {
                break
            }
        }

        logger.WARNING.L("listen_moe", "Connection to wss://listen.moe lost. Reconnecting!")
        c.Close()

        time.Sleep(5 * time.Second)
    }
}
