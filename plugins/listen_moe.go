package plugins

import (
    "time"
    "sync"
    "github.com/sn0w/discordgo"
    "git.lukas.moe/sn0w/Karen/channels"
    "git.lukas.moe/sn0w/Karen/logger"
    "git.lukas.moe/sn0w/Karen/helpers"
    "strings"
    "os/exec"
    "bufio"
    "encoding/binary"
    "io"
)

// ---------------------------------------------------------------------------------------------------------------------
// Helper structs for managing and closing voice connections
// ---------------------------------------------------------------------------------------------------------------------
var ListenDotMoeChan channels.Receiver

type RadioGuildConnection struct {
    sync.RWMutex

    // Closer channel for stop commands
    closer    chan struct{}

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
    }
}

func (l *ListenDotMoe) Init(session *discordgo.Session) {
    l.connections = make(map[string]*RadioGuildConnection)

    go l.streamer()
}

func (l *ListenDotMoe) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    // Sanitize subcommand
    content = strings.TrimSpace(content)

    // Store channel ref
    channel, err := session.Channel(msg.ChannelID)
    helpers.Relax(err)

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
        if content == "join" {
            lock := helpers.VoiceOccupy(guild.ID, "listen.moe")
            helpers.RelaxAssertEqual(lock, true, nil)

            message, merr := session.ChannelMessageSend(channel.ID, ":arrows_counterclockwise: Joining...")

            voiceConnection, err = session.ChannelVoiceJoin(guild.ID, vc.ID, false, false)
            helpers.Relax(err)

            if merr == nil {
                session.ChannelMessageEdit(channel.ID, message.ID, "Joined!\nThe radio should start playing shortly c:")

                l.connections[guild.ID] = (&RadioGuildConnection{}).Alloc()

                go l.pipeStream(guild.ID, session)
                return
            }

            helpers.Relax(merr)
        } else {
            session.ChannelMessageSend(channel.ID, "You should join the channel I'm in or make me join yours before telling me to do stuff :thinking:")
        }

        return
    }

    // We are present.
    // Check for other commands
    switch content {
    case "leave", "l":
        voiceConnection.Close()

        l.connections[guild.ID].Lock()
        l.connections[guild.ID].Close()
        delete(l.connections, guild.ID)

        session.ChannelMessageSend(channel.ID, "OK, bye :frowning:")
        break

    case "playing", "np", "song", "title":
        session.ChannelMessageSend(channel.ID, "Not yet implemented")
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

func (l *ListenDotMoe) streamer() {
    logger.VERBOSE.L("listen_moe.go", "Allocating channels")
    broadcastChannel := channels.NewBroadcaster()
    ListenDotMoeChan = broadcastChannel.Listen()

    logger.VERBOSE.L("listen_moe.go", "Piping subprocesses")

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

    logger.VERBOSE.L("listen_moe.go", "Running subprocesses")

    // Run ffmpeg
    err = ffmpeg.Start()
    helpers.Relax(err)

    // Run ropus
    err = ropus.Start()
    helpers.Relax(err)

    // Stream ropus to buffer
    robuf := bufio.NewReaderSize(rout, 16384)

    // Stream ropus output to discord
    var opusLength int16

    logger.VERBOSE.L("listen_moe.go", "Streaming")
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
        broadcastChannel.Write(opus)
    }

    logger.VERBOSE.L("listen_moe.go", "Stream died")
}

func (l *ListenDotMoe) pipeStream(guildID string, session *discordgo.Session) {
    vc := session.VoiceConnections[guildID]

    // Start eventloop
    for {
        // Exit if the closer channel dies
        select {
        case <-l.connections[guildID].closer:
            return
        default:
        }

        // Do nothing until voice is ready
        if !vc.Ready {
            time.Sleep(1 * time.Second)
            continue
        }

        // Send a frame to discord
        vc.OpusSend <- ListenDotMoeChan.Read()
    }
}
