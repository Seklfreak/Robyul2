package utils

import (
    "bytes"
    "errors"
    "github.com/Jeffail/gabs"
    "github.com/bwmarrin/discordgo"
    "github.com/sn0w/Karen/helpers"
    "github.com/ugjka/cleverbot-go"
    "io"
    "net/http"
    "strconv"
    "encoding/base64"
)

// Callback aliases a func
type Callback func()

var (
    // config Saves the bot-config
    config *gabs.Container

    // cleverbotSessions stores all cleverbot connections
    cleverbotSessions map[string]*cleverbot.Session
)

// LoadConfig loads the config from $path into $config
func LoadConfig(path string) {
    json, err := gabs.ParseJSONFile(path)

    if err != nil {
        panic(err)
    }

    config = json
}

// GetConfig is a config getter
func GetConfig() *gabs.Container {
    return config
}

// CleverbotSend sends a message to cleverbot and responds with it's answer.
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

// CleverbotRefreshSession refreshes the cleverbot session for said channel
func CleverbotRefreshSession(channel string) {
    cleverbotSessions[channel] = cleverbot.New()
}

// GetPrefixForServer gets the prefix for $guild
func GetPrefixForServer(guild string) (string, error) {
    settings, err := GuildSettingsGet(guild)
    if err != nil {
        return "", err
    }

    return settings.Prefix, nil
}

// SetPrefixForServer sets the prefix for $guild to $prefix
func SetPrefixForServer(guild string, prefix string) error {
    settings, err := GuildSettingsGet(guild)
    if err != nil {
        return err
    }

    settings.Prefix = prefix

    return GuildSettingsSet(guild, settings)
}

// NetGet executes a GET request to url with the Karen/Discord-Bot user-agent
func NetGet(url string) []byte {
    return NetGetUA(url, "Karen/Discord-Bot")
}

// NetGetUA performs a GET request with a custom user-agent
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

// GetJSON sends a GET request to $url, parses it and returns the JSON
func GetJSON(url string) *gabs.Container {
    // Parse json
    json, err := gabs.ParseJSON(NetGet(url))
    helpers.Relax(err)

    return json
}

// RequireAdmin only calls $cb if the author is an admin or has MANAGE_SERVER permission
func RequireAdmin(session *discordgo.Session, msg *discordgo.Message, cb Callback) {
    channel, e := session.Channel(msg.ChannelID)
    if e != nil {
        helpers.SendError(session, msg, errors.New("Cannot verify permissions"))
        return
    }

    guild, e := session.Guild(channel.GuildID)
    if e != nil {
        helpers.SendError(session, msg, errors.New("Cannot verify permissions"))
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

// BtoA is a polyfill for javascript's window#btoa()
func BtoA(s string) string {
    b64 := base64.URLEncoding.WithPadding(base64.NoPadding)
    src := []byte(s)
    buf := make([]byte, b64.EncodedLen(len(src)))
    b64.Encode(buf, src)

    return string(buf)
}
