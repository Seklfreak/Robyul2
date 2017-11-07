package plugins

import (
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

type Support struct{}

func (s *Support) Commands() []string {
	return []string{
		"support",
		"discord",
	}
}

func (s *Support) Init(session *discordgo.Session) {

}

func (s *Support) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	helpers.SendMessage(msg.ChannelID, "Here you go <:googlesmile:317031693951434752> \n https://discord.gg/wNPejct")
}
