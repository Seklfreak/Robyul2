package plugins

import (
    "github.com/Seklfreak/Robyul2/helpers"
    "github.com/bwmarrin/discordgo"
    "github.com/Jeffail/gabs"
    "fmt"
    "net/http"
    "strings"
    "bytes"
    "io"
    "github.com/Seklfreak/Robyul2/logger"
    "time"
)

const (
    gfycatApiBaseUrl  = "https://api.gfycat.com/v1/%s"
    gfycatFriendlyUrl = "https://gfycat.com/%s"
)

type Gfycat struct{}

func (m *Gfycat) Commands() []string {
    return []string{
        "gfycat",
        "gfy",
    }
}

func (m *Gfycat) Init(session *discordgo.Session) {

}

func (m *Gfycat) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) { // [p]gfy [<link>] or attachment [<start in seconds> <duration in seconds>]
    session.ChannelTyping(msg.ChannelID)

    if len(content) <= 0 && len(msg.Attachments) <= 0 {
        _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
        helpers.Relax(err)
        return
    }

    args := strings.Fields(content)
    sourceUrl := content
    cutArgJson := ""
    duration := ""
    start := ""
    if len(args) >= 3 || (len(args) >= 2 && len(msg.Attachments) > 0) {
        duration = args[len(args)-1]
        start = args[len(args)-2]
        cutArgJson = fmt.Sprintf(`,
        "cut": {
        "duration": "%s",
        "start": "%s"
        }`, duration, start)
        sourceUrl = strings.Join(args[0:len(args)-2], " ")
    }

    if len(msg.Attachments) > 0 {
        sourceUrl = msg.Attachments[0].URL
    }

    accessToken := m.getAccessToken()

    httpClient = &http.Client{}

    postGfycatEndpoint := fmt.Sprintf(gfycatApiBaseUrl, "gfycats")
    postData, err := gabs.ParseJSON([]byte(fmt.Sprintf(
        `{"private": true,
    "fetchUrl": "%s"%s}`,
        sourceUrl, cutArgJson,
    )))
    helpers.Relax(err)
    request, err := http.NewRequest("POST", postGfycatEndpoint, strings.NewReader(postData.String()))
    request.Header.Add("user-agent", helpers.DEFAULT_UA)
    request.Header.Add("content-type", "application/json")
    request.Header.Add("Authorization", accessToken)
    helpers.Relax(err)
    response, err := httpClient.Do(request)
    helpers.Relax(err)
    defer response.Body.Close()
    buf := bytes.NewBuffer(nil)
    _, err = io.Copy(buf, response.Body)
    helpers.Relax(err)
    jsonResult, err := gabs.ParseJSON(buf.Bytes())
    helpers.Relax(err)

    session.ChannelMessageSend(msg.ChannelID, "Your gfycat is processing, this may take a while. <:blobsleeping:317047101534109696>")
    session.ChannelTyping(msg.ChannelID)

    if jsonResult.ExistsP("isOk") == false || jsonResult.Path("isOk").Data().(bool) == false {
        errorMessage := ""
        fmt.Println(jsonResult.Path("errorMessage").Data().(string))
        if jsonResult.ExistsP("errorMessage") && jsonResult.Path("errorMessage").Data().(string) != "" {
            parsedErrorMessage, err := gabs.ParseJSON([]byte(jsonResult.Path("errorMessage").Data().(string))) // weird gfycat api is weird?
            helpers.Relax(err)
            if parsedErrorMessage.ExistsP("description") {
                errorMessage = parsedErrorMessage.Path("description").Data().(string)
            }
        }
        if errorMessage == "" {
            _, err = session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("<@%s> ", msg.Author.ID)+helpers.GetTextF("bot.errors.general", "Gfycat Error")+"\nPlease check the link or try again later.")
            logger.ERROR.L("gfycat", fmt.Sprintf("Gfycat Error: %s", jsonResult.String()))
        } else {
            _, err = session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("<@%s> ", msg.Author.ID)+fmt.Sprintf("Error: `%s`.", errorMessage))
        }
        helpers.Relax(err)
        return
    }

    gfyName := jsonResult.Path("gfyname").Data().(string)
CheckGfycatStatusLoop:
    for {
        statusGfycatEndpoint := fmt.Sprintf(gfycatApiBaseUrl, fmt.Sprintf("gfycats/fetch/status/%s", gfyName))
        result := helpers.GetJSON(statusGfycatEndpoint)

        switch result.Path("task").Data().(string) {
        case "encoding":
            time.Sleep(5 * time.Second)
            session.ChannelTyping(msg.ChannelID)
            continue CheckGfycatStatusLoop
        case "complete":
            gfyName = result.Path("gfyname").Data().(string)
            break CheckGfycatStatusLoop
        default:
            logger.ERROR.L("gfycat", fmt.Sprintf("Gfycat Status Error: %s", result.String()))
            _, err := session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("<@%s> ", msg.Author.ID)+helpers.GetTextF("bot.errors.general", "Gfycat Status Error")+"\nPlease check the link or try again later.")
            helpers.Relax(err)
            return
        }
    }

    gfycatUrl := fmt.Sprintf(gfycatFriendlyUrl, gfyName)

    _, err = session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("<@%s> Your gfycat is done: %s .", msg.Author.ID, gfycatUrl))
    helpers.Relax(err)
}

func (m *Gfycat) getAccessToken() string {
    getTokenEndpoint := fmt.Sprintf(gfycatApiBaseUrl, "oauth/token")
    postData, err := gabs.ParseJSON([]byte(fmt.Sprintf(
        `{"grant_type": "client_credentials",
    "client_id": "%s",
    "client_secret": "%s"}`,
        helpers.GetConfig().Path("gfycat.client_id").Data().(string),
        helpers.GetConfig().Path("gfycat.client_secret").Data().(string),
    )))
    helpers.Relax(err)
    httpClient = &http.Client{}
    request, err := http.NewRequest("POST", getTokenEndpoint, strings.NewReader(postData.String()))
    request.Header.Add("user-agent", helpers.DEFAULT_UA)
    helpers.Relax(err)
    response, err := httpClient.Do(request)
    helpers.Relax(err)
    defer response.Body.Close()
    buf := bytes.NewBuffer(nil)
    _, err = io.Copy(buf, response.Body)
    helpers.Relax(err)
    jsonResult, err := gabs.ParseJSON(buf.Bytes())
    helpers.Relax(err)

    tokenType := jsonResult.Path("token_type").Data().(string)
    accessToken := jsonResult.Path("access_token").Data().(string)

    return strings.Title(tokenType) + " " + accessToken
}
