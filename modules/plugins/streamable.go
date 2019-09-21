package plugins

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"io/ioutil"

	"github.com/Jeffail/gabs"
	"github.com/PuerkitoBio/goquery"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/shardmanager"
	"github.com/bwmarrin/discordgo"
	"github.com/dyatlov/go-oembed/oembed"
)

const (
	streamableApiBaseUrl = "https://api.streamable.com/%s"
)

type Streamable struct{}

func (s *Streamable) Commands() []string {
	return []string{
		"streamable",
	}
}

var (
	oEmbedHandler *oembed.Oembed
)

func (s *Streamable) Init(session *shardmanager.Manager) {
	data, err := ioutil.ReadFile(helpers.GetConfig().Path("assets_folder").Data().(string) + "providers.json")
	helpers.Relax(err)

	oEmbedHandler = oembed.NewOembed()
	err = oEmbedHandler.ParseProviders(bytes.NewReader(data))
	helpers.Relax(err)
}

func (s *Streamable) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) { // [p]streamable [<link>] or attachment
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermStreamable) {
		return
	}

	var err error

	session.ChannelTyping(msg.ChannelID)

	if len(content) <= 0 && len(msg.Attachments) <= 0 {
		_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		helpers.Relax(err)
		return
	}

	sourceUrl := content
	if len(msg.Attachments) > 0 {
		sourceUrl = msg.Attachments[0].URL
	}

	var streamableTitle string

	req, err := http.NewRequest("GET", sourceUrl, nil)
	if err == nil {
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
		resp, _ := httpClient.Do(req)
		if resp != nil && resp.Body != nil {
			defer resp.Body.Close()
		}
		if err == nil {
			// try oEmbed title
			finalURL := resp.Request.URL.String()

			oEmbedResult := oEmbedHandler.FindItem(finalURL)
			if oEmbedResult != nil {
				oEmbedInfo, err := oEmbedResult.FetchOembed(oembed.Options{URL: sourceUrl})
				if err == nil {
					if oEmbedInfo.Status < 300 && oEmbedInfo.Title != "" {
						streamableTitle = oEmbedInfo.Title
					}
				}
			}
			// fallback to html page title
			if streamableTitle == "" {
				doc, err := goquery.NewDocumentFromReader(resp.Body)
				if err == nil {
					streamableTitle = strings.Trim(doc.Find("title").Text(), "\"")
				}
			}
		}
	}

	if streamableTitle == "" {
		streamableTitle = sourceUrl
	} else {
		streamableTitle += "\n" + sourceUrl
	}

	createStreamableEndpoint := fmt.Sprintf(streamableApiBaseUrl, fmt.Sprintf("import?url=%s&title=%s", url.QueryEscape(sourceUrl), url.QueryEscape(streamableTitle)))
	request, err := http.NewRequest("GET", createStreamableEndpoint, nil)
	helpers.Relax(err)
	request.Header.Add("user-agent", helpers.DEFAULT_UA)
	request.SetBasicAuth(helpers.GetConfig().Path("streamable.username").Data().(string),
		helpers.GetConfig().Path("streamable.password").Data().(string))
	response, err := httpClient.Do(request)
	if err != nil {
		if strings.Contains(err.Error(), "Client.Timeout exceeded while awaiting headers") {
			helpers.SendMessage(msg.ChannelID, "I wasn't able to talk to streamable. <a:ablobfrown:394026913292615701>\nPlease try again in a few minutes.")
			return
		}
	}
	helpers.Relax(err)

	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}

	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, response.Body)
	helpers.Relax(err)

	jsonResult, err := gabs.ParseJSON(buf.Bytes())

	if err != nil || jsonResult.ExistsP("status") == false || jsonResult.Path("status").Data().(float64) >= 3 {
		_, err = helpers.SendMessage(msg.ChannelID,
			fmt.Sprintf("<@%s> Something went wrong while creating your streamable. <:blobscream:317043778823389184>",
				msg.Author.ID))
		helpers.Relax(err)
		return
	}

	processMessages, _ := helpers.SendMessage(msg.ChannelID, "Your streamable is processing, this may take a while. <a:ablobsleep:394026914290991116>")

	streamableShortcode := jsonResult.Path("shortcode").Data().(string)
	streamableUrl := ""
CheckStreamableStatusLoop:
	for {
		statusStreamableEndpoint := fmt.Sprintf(streamableApiBaseUrl, fmt.Sprintf("videos/%s", streamableShortcode))
		result, err := gabs.ParseJSON(helpers.NetGet(statusStreamableEndpoint))
		if err != nil {
			if strings.Contains(err.Error(), "Expected status 200; Got 429") {
				_, err = helpers.SendMessage(msg.ChannelID,
					fmt.Sprintf("<@%s> Too many requests, please try again later. <:blobscream:317043778823389184>",
						msg.Author.ID))
				helpers.Relax(err)
			} else {
				helpers.Relax(err)
			}
		}

		switch result.Path("status").Data().(float64) {
		case 0:
		case 1:
			time.Sleep(5 * time.Second)
			continue CheckStreamableStatusLoop
		case 2:
			streamableUrl = result.Path("url").Data().(string)
			if !strings.Contains(streamableUrl, "://") {
				streamableUrl = "https://" + streamableUrl
			}
			break CheckStreamableStatusLoop
		default:
			_, err = helpers.SendMessage(msg.ChannelID,
				fmt.Sprintf("<@%s> Something went wrong while creating your streamable. <:blobscream:317043778823389184>",
					msg.Author.ID))
			helpers.Relax(err)

			if processMessages != nil && len(processMessages) > 0 {
				for _, processMessage := range processMessages {
					session.ChannelMessageDelete(processMessage.ChannelID, processMessage.ID)
				}
			}
			return
		}
	}

	_, err = helpers.SendMessage(msg.ChannelID, fmt.Sprintf("<@%s> Your streamable is done: %s .", msg.Author.ID, streamableUrl))
	helpers.Relax(err)

	if processMessages != nil && len(processMessages) > 0 {
		for _, processMessage := range processMessages {
			session.ChannelMessageDelete(processMessage.ChannelID, processMessage.ID)
		}
	}
}
