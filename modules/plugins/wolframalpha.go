package plugins

import (
	"strings"

	"net/url"

	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/go-wolfram"
	"github.com/bwmarrin/discordgo"
)

type WolframAlpha struct{}

func (m *WolframAlpha) Commands() []string {
	return []string{
		"wolfram",
		"w",
		"ask",
	}
}

func (m *WolframAlpha) Init(session *discordgo.Session) {

}

func (m *WolframAlpha) TypingLoop(channelID string, quitChannel chan int) {
	for {
		select {
		case <-quitChannel:
			return
		default:
			cache.GetSession().ChannelTyping(channelID)
			time.Sleep(5 * time.Second)
		}
	}
}

func (m *WolframAlpha) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	quitChannel := make(chan int)
	defer func() { quitChannel <- 0 }()

	go m.TypingLoop(msg.ChannelID, quitChannel)

	wolframClient := &wolfram.Client{AppID: helpers.GetConfig().Path("wolframalpha.appid").Data().(string)}

	var res string
	var imageSearch bool
	if strings.HasPrefix(content, "image ") || strings.HasPrefix(content, "img ") {
		content = strings.TrimLeft(content, "image ")
		content = strings.TrimLeft(content, "img ")
		content = strings.TrimSpace(content)

		imageSearch = true
	}

	res, err := wolframClient.GetShortAnswerQuery(content, wolfram.Metric, 10)
	helpers.Relax(err)

	if res == "No short answer available" {
		imageSearch = true
	}

	if imageSearch {
		urlValues := url.Values{}
		urlValues.Add("foreground", "white")
		urlValues.Add("background", "35393E")
		urlValues.Add("layout", "labelbar")
		urlValues.Add("timeout", "30")

		image, _, err := wolframClient.GetSimpleQuery(content, urlValues)
		helpers.Relax(err)

		_, err = helpers.SendComplex(
			msg.ChannelID, &discordgo.MessageSend{
				Files: []*discordgo.File{
					{
						Name:   "wolframalpha-robyul.png",
						Reader: image,
					},
				},
			})
		helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
		return
	}

	if res == "" || res == "Wolfram|Alpha did not understand your input" {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.wolframalpha.error"))
		return
	}

	_, err = helpers.SendMessage(msg.ChannelID, res)
	helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
}
