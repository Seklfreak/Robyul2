package plugins

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/bwmarrin/discordgo"
)

type WolframAlpha struct{}

const (
	wolframBaseUrl       string = "http://api.wolframalpha.com/v2/query?units=metric&output=json&appid=%s&input=%s"
	wolframFriendlyUrl   string = "http://www.wolframalpha.com/input/?i=%s"
	wolframalphaHexColor string = "#ff8737"
)

func (m *WolframAlpha) Commands() []string {
	return []string{
		"wolfram",
		"w",
	}
}

func (m *WolframAlpha) Init(session *discordgo.Session) {

}

func (m *WolframAlpha) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	defer func() {
		err := recover()

		if err != nil {
			session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.wolframalpha.error"))
			return
		}
	}()
	session.ChannelTyping(msg.ChannelID)

	encodedQuery := url.QueryEscape(content)
	queryUrl := fmt.Sprintf(wolframBaseUrl, helpers.GetConfig().Path("wolframalpha.appid").Data().(string), encodedQuery)

	result := helpers.GetJSON(queryUrl)

	podResultItems, err := result.Path("queryresult.pods").Children()
	helpers.Relax(err)

	resultEmbed := &discordgo.MessageEmbed{
		Title:  helpers.GetTextF("plugins.wolframalpha.result-embed-title", content),
		URL:    fmt.Sprintf(wolframFriendlyUrl, encodedQuery),
		Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.wolframalpha.embed-footer")},
		Fields: []*discordgo.MessageEmbedField{},
		Color:  helpers.GetDiscordColorFromHex(wolframalphaHexColor),
	}

	for _, podResult := range podResultItems {
		isPrimary, ok := podResult.Path("primary").Data().(bool)
		if ok == false || isPrimary == false {
			continue
		}

		titleText := podResult.Path("title").Data().(string)
		valueText := ""
		subPodResultItems, err := podResult.Path("subpods").Children()
		helpers.Relax(err)
		for _, subPodResult := range subPodResultItems {
			for _, line := range strings.Split(subPodResult.Path("plaintext").Data().(string), "|") {
				if line != "" {
					valueText += strings.TrimSpace(line) + "; "
				}
			}
		}
		if valueText != "" {
			resultEmbed.Fields = append(resultEmbed.Fields, &discordgo.MessageEmbedField{
				Name:   titleText,
				Value:  valueText,
				Inline: false,
			})
		}
	}

	session.ChannelMessageSendEmbed(msg.ChannelID, resultEmbed)
	metrics.WolframAlphaRequests.Add(1)
}
