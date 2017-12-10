package plugins

import (
	"strings"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	"github.com/domainr/whois"
)

type Whois struct{}

func (w *Whois) Commands() []string {
	return []string{
		"whois",
	}
}

func (w *Whois) Init(session *discordgo.Session) {
}

func (w *Whois) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	session.ChannelTyping(msg.ChannelID)

	args := strings.Fields(content)

	if len(args) < 1 {
		helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
		return
	}

	request, err := whois.NewRequest(args[0])
	if err != nil {
		if strings.Contains(err.Error(), "no public zone found for") {
			helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
			return
		}
	}
	helpers.Relax(err)

	response, err := whois.DefaultClient.Fetch(request)
	helpers.Relax(err)

	_, err = helpers.SendMessageBoxed(msg.ChannelID, string(response.Body))
	helpers.Relax(err)
}
