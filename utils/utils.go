package utils

import (
    "bytes"
    "errors"
    "fmt"
    "github.com/Jeffail/gabs"
    "github.com/bwmarrin/discordgo"
    "github.com/getsentry/raven-go"
    "github.com/sn0w/Karen/helpers"
    "github.com/ugjka/cleverbot-go"
    "io"
    "net/http"
    "strconv"
)

// Defines what a callback is
type Callback func()

var (
    // Saves the config
    config *gabs.Container

    // Array of cleverbot sessions
    cleverbotSessions map[string]*cleverbot.Session
)

// Loads the config from $path into $config
func LoadConfig(path string) {
    json, err := gabs.ParseJSONFile(path)

    if err != nil {
        panic(err)
    }

    config = json
}

// Config getter
func GetConfig() *gabs.Container {
    return config
}

// Sends a message to cleverbot. Responds with it's answer.
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

// Refreshes the cleverbot session for said channel
func CleverbotRefreshSession(channel string) {
    cleverbotSessions[channel] = cleverbot.New()
}

// Gets the prefix for $guild
func GetPrefixForServer(guild string) (string, error) {
    settings, err := GuildSettingsGet(guild)
    if err != nil {
        return "", err
    }

    return settings.Prefix, nil
}

// Sets the prefix for $guild to $prefix
func SetPrefixForServer(guild string, prefix string) error {
    settings, err := GuildSettingsGet(guild)
    if err != nil {
        return err
    }

    settings.Prefix = prefix

    return GuildSettingsSet(guild, settings)
}

// Takes an error and sends it to discord and sentry.io
func SendError(session *discordgo.Session, msg *discordgo.Message, err interface{}) {
    session.ChannelMessageSend(
        msg.ChannelID,
        "Error :frowning:\n0xFADED#3237 has been notified.\n```\n" +
            fmt.Sprintf("%#v", err) +
            "\n```\nhttp://i.imgur.com/FcV2n4X.jpg",
    )

    raven.SetUserContext(&raven.User{
        ID:       msg.ID,
        Username: msg.Author.Username + "#" + msg.Author.Discriminator,
    })
    raven.CaptureError(errors.New(fmt.Sprintf("%#v", err)), map[string]string{
        "ChannelID":       msg.ChannelID,
        "Content":         msg.Content,
        "Timestamp":       string(msg.Timestamp),
        "TTS":             strconv.FormatBool(msg.Tts),
        "MentionEveryone": strconv.FormatBool(msg.MentionEveryone),
        "IsBot":           strconv.FormatBool(msg.Author.Bot),
    })
}

func NetGet(url string) []byte {
    return NetGetUA(url, "Karen/Discord-Bot")
}

func NetGetUA(url string, useragent string) []byte {
    // Allocate client
    client := &http.Client{}

    // Prepare request
    request, err := http.NewRequest("GET", url, nil)
    if err != nil {
        panic(err)
    }

    // Set custom UA
    request.Header.Set("User-Agent", useragent)

    // Do request
    response, err := client.Do(request)
    helpers.Relax(err)

    // Only continue if code was 200
    if response.StatusCode != 200 {
        panic(errors.New("Expected status 200; Got " + strconv.Itoa(response.StatusCode)))
    } else {
        // Read body
        defer response.Body.Close()

        buf := bytes.NewBuffer(nil)
        _, err := io.Copy(buf, response.Body)
        helpers.Relax(err)

        return buf.Bytes()
    }
}

// Sends a GET request to $url, parses it and returns the JSON
func GetJSON(url string) *gabs.Container {
    // Parse json
    json, err := gabs.ParseJSON(NetGet(url))
    helpers.Relax(err)

    return json
}

// Only calls $cb if the author is an admin or has MANAGE_SERVER permission
func RequireAdmin(session *discordgo.Session, msg *discordgo.Message, cb Callback) {
    channel, e := session.Channel(msg.ChannelID)
    if e != nil {
        SendError(session, msg, errors.New("Cannot verify permissions"))
        return
    }

    guild, e := session.Guild(channel.GuildID)
    if e != nil {
        SendError(session, msg, errors.New("Cannot verify permissions"))
        return
    }

    if msg.Author.ID == guild.OwnerID {
        cb()
        return
    }

    // Check if role may manage server
    for _, role := range guild.Roles {
        if role.Permissions & 8 == 8 {
            cb()
            return
        }
    }

    session.ChannelMessageSend(msg.ChannelID, "You are not an admin :frowning:")
}
