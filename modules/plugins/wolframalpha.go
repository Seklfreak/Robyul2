package plugins

import (
	"strings"

	"net/url"

	"bytes"

	"github.com/Krognol/go-wolfram"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
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

func (m *WolframAlpha) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermWolframAlpha) {
		return
	}

	var err error
	var res string
	var imageSearch bool
	if strings.HasPrefix(content, "image ") || strings.HasPrefix(content, "img ") {
		content = strings.TrimLeft(content, "image ")
		content = strings.TrimLeft(content, "img ")
		content = strings.TrimSpace(content)

		imageSearch = true
	}

	if content == "" || content == "img" {
		return
	}

	quitChannel := helpers.StartTypingLoop(msg.ChannelID)
	defer func() { quitChannel <- 0 }()

	wolframClient := &wolfram.Client{AppID: helpers.GetConfig().Path("wolframalpha.appid").Data().(string)}

	if !imageSearch {
		metrics.WolframAlphaRequests.Add(1)
		res, err = wolframClient.GetShortAnswerQuery(content, wolfram.Metric, 10)
		helpers.Relax(err)

		if res == "No short answer available" {
			imageSearch = true
		}
	}

	if imageSearch {
		urlValues := url.Values{}
		//urlValues.Add("foreground", "white")
		//urlValues.Add("background", "35393E")
		urlValues.Add("layout", "labelbar")
		urlValues.Add("timeout", "30")

		metrics.WolframAlphaRequests.Add(1)
		image, _, err := wolframClient.GetSimpleQuery(content, urlValues)
		helpers.Relax(err)

		buf := new(bytes.Buffer)
		buf.ReadFrom(image)
		if strings.Contains(buf.String(), "Wolfram|Alpha did not understand your input") {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.wolframalpha.error"))
			return
		}

		quitChannel <- 0
		_, err = helpers.SendComplex(
			msg.ChannelID, &discordgo.MessageSend{
				Files: []*discordgo.File{
					{
						Name:   "wolframalpha-robyul.png",
						Reader: bytes.NewReader(buf.Bytes()),
					},
				},
			})
		helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
		return
	}

	if res == "" || res == "Wolfram|Alpha did not understand your input" {
		quitChannel <- 0
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.wolframalpha.error"))
		return
	}

	quitChannel <- 0
	_, err = helpers.SendMessage(msg.ChannelID, res)
	helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
}
