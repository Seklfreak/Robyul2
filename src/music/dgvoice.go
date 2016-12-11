/**
 * This was mostly taken from github.com/bwmarrin/dgvoice.git
 * and slightly modified. Thanks for the hard work!
 */
package music

import (
    "bufio"
    "encoding/binary"
    "fmt"
    "io"
    "os/exec"
    "strconv"
    "sync"

    "github.com/bwmarrin/discordgo"
    "github.com/layeh/gopus"
)

const (
    // 1 for mono, 2 for stereo
    channels int = 2

    // audio sampling rate
    frameRate int = 48000

    // uint16 size of each audio frame
    frameSize int = 960

    // max size of opus data
    maxBytes int = (frameSize * 2) * 2
)

var (
    speakers map[uint32]*gopus.Decoder
    opusEncoder *gopus.Encoder
    run *exec.Cmd
    sendpcm bool
    recvpcm bool
    recv chan *discordgo.Packet
    send chan []int16
    mu sync.Mutex
)

// SendPCM will receive on the provied channel encode
// received PCM data into Opus then send that to Discordgo
func SendPCM(v *discordgo.VoiceConnection, pcm <-chan []int16) {
    // make sure this only runs one instance at a time.
    mu.Lock()

    if sendpcm || pcm == nil {
        mu.Unlock()
        return
    }

    sendpcm = true
    mu.Unlock()

    defer func() {
        sendpcm = false
    }()

    var err error

    opusEncoder, err = gopus.NewEncoder(frameRate, channels, gopus.Audio)

    if err != nil {
        fmt.Println("NewEncoder Error:", err)
        return
    }

    for {
        // read pcm from chan, exit if channel is closed.
        recv, ok := <-pcm
        if !ok {
            fmt.Println("PCM Channel closed.")
            return
        }

        // try encoding pcm frame with Opus
        opus, err := opusEncoder.Encode(recv, frameSize, maxBytes)
        if err != nil {
            fmt.Println("Encoding Error:", err)
            return
        }

        if v.Ready == false || v.OpusSend == nil {
            fmt.Printf("Discordgo not ready for opus packets. %+v : %+v", v.Ready, v.OpusSend)
            return
        }

        // send encoded opus data to the sendOpus channel
        v.OpusSend <- opus
    }
}

// ReceivePCM will receive on the the Discordgo OpusRecv channel and decode
// the opus audio into PCM then send it on the provided channel.
func ReceivePCM(v *discordgo.VoiceConnection, c chan *discordgo.Packet) {
    // make sure this only runs one instance at a time.
    mu.Lock()
    if recvpcm || c == nil {
        mu.Unlock()
        return
    }

    recvpcm = true
    mu.Unlock()

    defer func() {
        sendpcm = false
    }()

    var err error

    for {
        if v.Ready == false || v.OpusRecv == nil {
            fmt.Printf("Discordgo not ready to receive opus packets. %+v : %+v", v.Ready, v.OpusRecv)
            return
        }

        p, ok := <-v.OpusRecv
        if !ok {
            return
        }

        if speakers == nil {
            speakers = make(map[uint32]*gopus.Decoder)
        }

        _, ok = speakers[p.SSRC]
        if !ok {
            speakers[p.SSRC], err = gopus.NewDecoder(48000, 2)
            if err != nil {
                fmt.Println("error creating opus decoder:", err)
                continue
            }
        }

        p.PCM, err = speakers[p.SSRC].Decode(p.Opus, 960, false)
        if err != nil {
            fmt.Println("Error decoding opus data: ", err)
            continue
        }

        c <- p
    }
}

// PlayAudioFile will play the given filename to the already connected
// Discord voice server/channel.  voice websocket and udp socket
// must already be setup before this will work.
func PlayAudioFile(v *discordgo.VoiceConnection, filename string) {
    // Create a shell command "object" to run.
    run = exec.Command("ffmpeg", "-i", filename, "-f", "s16le", "-ar", strconv.Itoa(frameRate), "-ac", strconv.Itoa(channels), "pipe:1")
    ffmpegout, err := run.StdoutPipe()
    if err != nil {
        fmt.Println("StdoutPipe Error:", err)
        return
    }

    ffmpegbuf := bufio.NewReaderSize(ffmpegout, 16384)

    // Starts the ffmpeg command
    err = run.Start()
    if err != nil {
        fmt.Println("RunStart Error:", err)
        return
    }

    // Send "speaking" packet over the voice websocket
    v.Speaking(true)

    // Send not "speaking" packet over the websocket when we finish
    defer v.Speaking(false)

    // will actually only spawn one instance, a bit hacky.
    if send == nil {
        send = make(chan []int16, 2)
    }
    go SendPCM(v, send)

    for {
        // read data from ffmpeg stdout
        audiobuf := make([]int16, frameSize * channels)
        err = binary.Read(ffmpegbuf, binary.LittleEndian, &audiobuf)
        if err == io.EOF || err == io.ErrUnexpectedEOF {
            return
        }
        if err != nil {
            fmt.Println("error reading from ffmpeg stdout :", err)
            return
        }

        // Send received PCM to the sendPCM channel
        send <- audiobuf
    }
}

// KillPlayer forces the player to stop by killing the ffmpeg cmd process
// this method may be removed later in favor of using chans or bools to
// request a stop.
func KillPlayer() {
    run.Process.Kill()
}