package plugins

import (
	"fmt"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

type Avatar struct{}

func (a *Avatar) Commands() []string {
	return []string{
		"avatar",
	}
}

func (a *Avatar) Init(session *discordgo.Session) {

}

func (a *Avatar) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	mentionCount := len(msg.Mentions)

	if mentionCount == 0 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.mentions.too-few"))
		return
	}

	if mentionCount > 1 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.mentions.too-many"))
		return
	}

	helpers.SendMessage(msg.ChannelID, "Here you go <a:ablobsmile:393869335312990209> \n "+fmt.Sprintf(
		"https://cdn.discordapp.com/avatars/%s/%s.jpg",
		msg.Mentions[0].ID,
		msg.Mentions[0].Avatar,
	))
}
