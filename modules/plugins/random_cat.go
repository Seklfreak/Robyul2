package plugins

import (
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

type RandomCat struct{}

func (rc RandomCat) Commands() []string {
	return []string{
		"cat",
	}
}

func (rc RandomCat) Init(session *discordgo.Session) {

}

func (rc RandomCat) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	const ENDPOINT = "http://random.cat/meow"

	session.ChannelTyping(msg.ChannelID)

	json := helpers.GetJSON(ENDPOINT)
	session.ChannelMessageSend(
		msg.ChannelID,
		"MEOW! :smiley_cat:\n"+json.Path("file").Data().(string),
	)
}
