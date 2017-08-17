package plugins

import (
    "github.com/Seklfreak/Robyul2/helpers"
    "github.com/bwmarrin/discordgo"
    "fmt"
    "time"
    "net/url"
    "strings"
    "net/http"
    "bytes"
    "io"
    "github.com/Jeffail/gabs"
)

const (
    streamableApiBaseUrl  = "https://api.streamable.com/%s"
)

type Streamable struct{}

func (s *Streamable) Commands() []string {
    return []string{
        "streamable",
    }
}

func (s *Streamable) Init(session *discordgo.Session) {

}

func (s *Streamable) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) { // [p]streamable [<link>] or attachment
    var err error

    session.ChannelTyping(msg.ChannelID)

    if len(content) <= 0 && len(msg.Attachments) <= 0 {
        _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
        helpers.Relax(err)
        return
    }

    sourceUrl := content
    if len(msg.Attachments) > 0 {
        sourceUrl = msg.Attachments[0].URL
    }

    createStreamableEndpoint := fmt.Sprintf(streamableApiBaseUrl, fmt.Sprintf("import?url=%s", url.QueryEscape(sourceUrl)))
    request, err := http.NewRequest("GET", createStreamableEndpoint, nil)
    request.Header.Add("user-agent", helpers.DEFAULT_UA)
    request.SetBasicAuth(helpers.GetConfig().Path("streamable.username").Data().(string),
        helpers.GetConfig().Path("streamable.password").Data().(string))
    helpers.Relax(err)
    response, err := httpClient.Do(request)
    helpers.Relax(err)
    defer response.Body.Close()
    buf := bytes.NewBuffer(nil)
    _, err = io.Copy(buf, response.Body)
    helpers.Relax(err)

    jsonResult, err := gabs.ParseJSON(buf.Bytes())

    if err != nil || jsonResult.ExistsP("status") == false || jsonResult.Path("status").Data().(float64) >= 3 {
        _, err = session.ChannelMessageSend(msg.ChannelID,
            fmt.Sprintf("<@%s> Something went wrong while creating your streamable. <:blobscream:317043778823389184>",
                msg.Author.ID))
        helpers.Relax(err)
        return
    }

    session.ChannelMessageSend(msg.ChannelID, "Your streamable is processing, this may take a while. <:blobsleeping:317047101534109696>")
    session.ChannelTyping(msg.ChannelID)

    streamableShortcode := jsonResult.Path("shortcode").Data().(string)
    streamableUrl := ""
CheckStreamableStatusLoop:
    for {
        statusStreamableEndpoint := fmt.Sprintf(streamableApiBaseUrl, fmt.Sprintf("videos/%s", streamableShortcode))
        result := helpers.GetJSON(statusStreamableEndpoint)

        switch result.Path("status").Data().(float64) {
        case 0:
        case 1:
            time.Sleep(5 * time.Second)
            session.ChannelTyping(msg.ChannelID)
            continue CheckStreamableStatusLoop
        case 2:
            streamableUrl = result.Path("url").Data().(string)
            if !strings.Contains(streamableUrl, "://") {
                streamableUrl = "https://" + streamableUrl
            }
            break CheckStreamableStatusLoop
        default:
            _, err = session.ChannelMessageSend(msg.ChannelID,
                fmt.Sprintf("<@%s> Something went wrong while creating your streamable. <:blobscream:317043778823389184>",
                    msg.Author.ID))
            helpers.Relax(err)
            return
            return
        }
    }

    _, err = session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("<@%s> Your streamable is done: %s .", msg.Author.ID, streamableUrl))
    helpers.Relax(err)
}
