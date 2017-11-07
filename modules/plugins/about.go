package plugins

import (
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

type About struct{}

func (a *About) Commands() []string {
	return []string{
		"about",
		"info",
	}
}

func (a *About) Init(session *discordgo.Session) {

}

func (a *About) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	m := "**Hey! I'm Robyul.**\nI'm built using Go, open-source and a fork of Shiro, formerly called Karen, which you can find here: <https://github.com/SubliminalHQ/shiro>.\nYou can find out more about me here: <https://robyul.chat/>.\nSuggestions and discussions are always welcome on the Discord for me: <https://discord.gg/s5qZvUV>."

	helpers.SendMessage(msg.ChannelID, m)
}
