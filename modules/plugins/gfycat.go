package plugins

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Jeffail/gabs"
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
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
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermGfycat) {
		return
	}

	session.ChannelTyping(msg.ChannelID)

	if len(content) <= 0 && len(msg.Attachments) <= 0 {
		_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		helpers.Relax(err)
		return
	}

	args := strings.Fields(content)
	sourceUrl := content
	cutArgJson := ""
	var duration, start string
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

	httpClient := &http.Client{
		Timeout: time.Duration(10 * time.Second),
	}

	postGfycatEndpoint := fmt.Sprintf(gfycatApiBaseUrl, "gfycats")
	postData, err := gabs.ParseJSON([]byte(fmt.Sprintf(
		`{"private": true,
    "fetchUrl": "%s"%s}`,
		sourceUrl, cutArgJson,
	)))
	if err != nil {
		if strings.Contains(err.Error(), "unexpected end of JSON input") {
			helpers.SendMessage(msg.ChannelID, fmt.Sprintf("<@%s> ", msg.Author.ID)+helpers.GetTextF("bot.errors.general", "Gfycat Error")+"\nPlease check the link or try again later.")
			cache.GetLogger().WithField("module", "gfycat").Errorf("Gfycat Error: %s", err.Error())
			return
		}
	}
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

	helpers.SendMessage(msg.ChannelID, "Your gfycat is processing, this may take a while. <a:ablobsleep:394026914290991116>")
	session.ChannelTyping(msg.ChannelID)

	if jsonResult.ExistsP("isOk") == false || jsonResult.Path("isOk").Data().(bool) == false {
		errorMessage := ""
		if jsonResult.ExistsP("errorMessage") && jsonResult.Path("errorMessage").Data().(string) != "" {
			parsedErrorMessage, err := gabs.ParseJSON([]byte(jsonResult.Path("errorMessage").Data().(string))) // weird gfycat api is weird?
			helpers.Relax(err)
			if parsedErrorMessage.ExistsP("description") {
				errorMessage = parsedErrorMessage.Path("description").Data().(string)
			}
		}
		if errorMessage == "" {
			_, err = helpers.SendMessage(msg.ChannelID, fmt.Sprintf("<@%s> ", msg.Author.ID)+helpers.GetTextF("bot.errors.general", "Gfycat Error")+"\nPlease check the link or try again later.")
			cache.GetLogger().WithField("module", "gfycat").Errorf("Gfycat Error: %s", jsonResult.String())
		} else {
			_, err = helpers.SendMessage(msg.ChannelID, fmt.Sprintf("<@%s> ", msg.Author.ID)+fmt.Sprintf("Error: `%s`.", errorMessage))
		}
		helpers.Relax(err)
		return
	}

	gfyName := jsonResult.Path("gfyname").Data().(string)
CheckGfycatStatusLoop:
	for {
		statusGfycatEndpoint := fmt.Sprintf(gfycatApiBaseUrl, fmt.Sprintf("gfycats/fetch/status/%s", gfyName))
		rawResult, err := helpers.NetGetUAWithError(statusGfycatEndpoint, helpers.DEFAULT_UA)
		if err != nil {
			if strings.Contains(err.Error(), "Expected status 200; Got 504") {
				_, err := helpers.SendMessage(msg.ChannelID, fmt.Sprintf("<@%s> ", msg.Author.ID)+helpers.GetTextF("bot.errors.general", "Gfycat Status Error")+"\nPlease check the link or try again later.")
				helpers.Relax(err)
				return
			}
		}
		result, err := gabs.ParseJSON(rawResult)
		if err != nil {
			if strings.Contains(err.Error(), "unexpected end of JSON input") {
				_, err := helpers.SendMessage(msg.ChannelID, fmt.Sprintf("<@%s> ", msg.Author.ID)+helpers.GetTextF("bot.errors.general", "Gfycat Parsing Error")+"\nPlease check the link or try again later.")
				helpers.Relax(err)
				return
			}
		}
		helpers.Relax(err)

		taskData, _ := result.Path("task").Data().(string)

		switch taskData {
		case "encoding":
			time.Sleep(5 * time.Second)
			session.ChannelTyping(msg.ChannelID)
			continue CheckGfycatStatusLoop
		case "complete":
			gfyName = result.Path("gfyname").Data().(string)
			break CheckGfycatStatusLoop
		default:
			cache.GetLogger().WithField("module", "gfycat").Errorf("Gfycat Status Error: %s (ID: %s)", result.String(), gfyName)
			_, err := helpers.SendMessage(msg.ChannelID, fmt.Sprintf("<@%s> ", msg.Author.ID)+helpers.GetTextF("bot.errors.general", "Gfycat Status Error")+"\nPlease check the link or try again later.")
			helpers.Relax(err)
			return
		}
	}

	gfycatUrl := fmt.Sprintf(gfycatFriendlyUrl, gfyName)

	_, err = helpers.SendMessage(msg.ChannelID, fmt.Sprintf("<@%s> Your gfycat is done: %s .", msg.Author.ID, gfycatUrl))
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
	httpClient := &http.Client{
		Timeout: time.Duration(10 * time.Second),
	}
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
