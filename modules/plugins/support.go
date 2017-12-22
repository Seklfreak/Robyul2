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
	helpers.SendMessage(msg.ChannelID, "Here you go <a:ablobsmile:393869335312990209> \n https://discord.gg/wNPejct")
}
