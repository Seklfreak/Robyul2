package plugins

import (
	"fmt"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/bwmarrin/discordgo"
	"net/url"
)

type WolframAlpha struct{}

const (
	wolframBaseUrl string = "http://api.wolframalpha.com/v1/result?units=metric&appid=%s&i=%s"
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

	queryUrl := fmt.Sprintf(wolframBaseUrl, helpers.GetConfig().Path("wolframalpha.appid").Data().(string), url.QueryEscape(content))

	result := helpers.NetGet(queryUrl)

	session.ChannelMessageSend(msg.ChannelID, string(result))
	metrics.WolframAlphaRequests.Add(1)
}
