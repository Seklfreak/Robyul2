package plugins

import (
    "github.com/sn0w/discordgo"
    "git.lukas.moe/sn0w/Karen/channels"
    "time"
    "net"
    "git.lukas.moe/sn0w/Karen/logger"
    "git.lukas.moe/sn0w/Karen/helpers"
    "fmt"
    "strings"
)

type ListenDotMoe struct{}

var ListenDotMoeChan channels.Receiver

func (l *ListenDotMoe) Commands() []string {
    return []string{
        "moe",
    }
}

func (l *ListenDotMoe) Init(session *discordgo.Session) {
    go streamer()
}

func (l *ListenDotMoe) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    switch strings.TrimSpace(content) {
    case "leave","l":
        break
    case "join","j":
        break
    }
}

func streamer() {
    logger.VERBOSE.L("listen_moe.go", "Allocating channels")
    broadcastChannel := channels.NewBroadcaster()
    ListenDotMoeChan = broadcastChannel.Listen()

    for {
        logger.VERBOSE.L("listen_moe.go", "Connecting")
        tcp, err := net.Dial("tcp", "listen.moe:9999")
        helpers.Relax(err)
        if err != nil {
            time.Sleep(5 * time.Second)
            continue
        }

        fmt.Fprint(tcp, "GET /stream HTTP/1.0\r\n\r\n")

        logger.VERBOSE.L("listen_moe.go", "Stream begins")
        opus := make([]byte, 960)
        for {
            _, err = tcp.Read(opus)
            if err != nil {
                break
            }

            broadcastChannel.Write(opus)
        }
        logger.VERBOSE.L("listen_moe.go", "Stream ended")
    }
}
