package plugins

import (
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	"net/url"
	"regexp"
	"strings"
)

type Weather struct{}

func (w *Weather) Commands() []string {
	return []string{
		"weather",
		"wttr",
	}
}

func (w *Weather) Init(session *discordgo.Session) {

}

func (w *Weather) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	session.ChannelTyping(msg.ChannelID)

	if content == "" {
		session.ChannelMessageSend(msg.ChannelID, "You should pass a city :thinking:")
		return
	}

	text := string(helpers.NetGetUA("http://wttr.in/"+url.QueryEscape(content), "curl/7.51.0"))
	if text == "ERROR" {
		session.ChannelMessageSend(msg.ChannelID, "Couldn't find that city :frowning:")
		return
	}

	lines := strings.Split(text, "\n")

	session.ChannelMessageSend(
		msg.ChannelID,
		"```\n"+
			regexp.MustCompile("\\[.*?m").ReplaceAllString(strings.Join(lines[0:7], "\n"), "")+
			"\n```",
	)
}
