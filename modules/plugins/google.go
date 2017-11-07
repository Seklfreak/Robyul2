package plugins

import (
	"fmt"
	"net/url"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

type Google struct{}

func (g *Google) Commands() []string {
	return []string{
		"google",
		"goog",
	}
}

func (g *Google) Init(session *discordgo.Session) {

}

func (g *Google) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	helpers.SendMessage(msg.ChannelID, fmt.Sprintf(
		"<https://lmgtfy.com/?q=%s>",
		url.QueryEscape(content),
	))
}
