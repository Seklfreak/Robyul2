package utils

import (
    "github.com/Jeffail/gabs"
    "github.com/ugjka/cleverbot-go"
    "github.com/bwmarrin/discordgo"
    "fmt"
    "bytes"
    "io"
    "net/http"
    "strconv"
    "errors"
    "github.com/getsentry/raven-go"
)

type Callback func()

var (
    config *gabs.Container
    cleverbotSessions map[string]*cleverbot.Session
)

func LoadConfig(path string) {
    json, err := gabs.ParseJSONFile(path)

    if err != nil {
        panic(err)
    }

    config = json
}

func GetConfig() *gabs.Container {
    return config
}

func CleverbotSend(session *discordgo.Session, channel string, message string) {
    var msg string

    if cleverbotSessions[channel] == nil {
        if len(cleverbotSessions) == 0 {
            cleverbotSessions = make(map[string]*cleverbot.Session)
        }

        cleverbotSessions[channel] = cleverbot.New()
    }

    response, err := cleverbotSessions[channel].Ask(message)
    if err != nil {
        msg = "Error :frowning:\n```\n" + err.Error() + "\n```"
    } else {
        msg = response
    }

    session.ChannelMessageSend(channel, msg)
}

func CleverbotRefreshSession(channel string) {
    cleverbotSessions[channel] = cleverbot.New()
}

func CCTV(session *discordgo.Session, message *discordgo.Message) {
    var (
        channelName string = "?"
        channelID string = "?"
        serverName string = "?"
        serverID string = "?"
    )

    channel, err := session.Channel(message.ChannelID)
    if err == nil {
        channelName = channel.Name
        channelID = channel.ID

        server, err := session.Guild(channel.GuildID)
        if err == nil {
            serverName = server.Name
            serverID = server.ID
        }
    }

    template := `
At:      %s
Origin:  #%s (%s) in %s (%s)
Author:  %s#%s (%s)
Message:
%s
`

    msg :=
        "```\n" +
            fmt.Sprintf(
                template,
                message.Timestamp,
                channelName,
                channelID,
                serverName,
                serverID,
                message.Author.Username,
                message.Author.Discriminator,
                message.Author.ID,
                message.Content,
            ) +
            "\n```"

    session.ChannelMessageSend(
        config.Path("cctv").Data().(string),
        msg,
    )
}

func GetPrefixForServer(guild string) (string, error) {
    return GuildSettingGet(guild, "prefix")
}

func SetPrefixForServer(guild string, prefix string) error {
    return GuildSettingSet(guild, "prefix", prefix)
}

func SendError(session *discordgo.Session, msg *discordgo.Message, err interface{}) {
    session.ChannelMessageSend(
        msg.ChannelID,
        "Error :frowning:\n```\n" +
            fmt.Sprintf("%#v", err) +
            "\n```\nhttp://i.imgur.com/FcV2n4X.jpg",
    )

    raven.SetUserContext(&raven.User{
        ID: msg.ID,
        Username: msg.Author.Username + "#" + msg.Author.Discriminator,
    })
    raven.CaptureError(errors.New(fmt.Sprintf("%#v", err)), map[string]string{
        "ChannelID": msg.ChannelID,
        "Content": msg.Content,
        "Timestamp": msg.Timestamp,
        "TTS": strconv.FormatBool(msg.Tts),
        "MentionEveryone": strconv.FormatBool(msg.MentionEveryone),
        "IsBot": strconv.FormatBool(msg.Author.Bot),
    })
}

func WhileTypingIn(session *discordgo.Session, channel string, cb Callback) {
    session.ChannelTyping(channel)
    cb()
}

func GetJSON(url string) *gabs.Container {
    // Send request
    response, err := http.Get(url)
    if err != nil {
        panic(err)
    }

    // Only continue if code was 200
    if response.StatusCode != 200 {
        panic(errors.New("Expected status 200; Got " + strconv.Itoa(response.StatusCode)))
    } else {
        // Read body
        defer response.Body.Close()

        buf := bytes.NewBuffer(nil)
        _, err := io.Copy(buf, response.Body)
        if err != nil {
            panic(err)
        }

        // Parse json
        json, err := gabs.ParseJSON(buf.Bytes())
        if err != nil {
            panic(err)
        }

        return json
    }
}