package plugins

import (
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/shardmanager"
	"github.com/bwmarrin/discordgo"
)

type M8ball struct{}

func (m8 *M8ball) Commands() []string {
	return []string{
		"8ball",
		"8",
	}
}

func (m8 *M8ball) Init(session *shardmanager.Manager) {
}

func (m8 *M8ball) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePerm8ball) {
		return
	}

	if len(content) < 3 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.8ball.ask_a_question"))
		return
	}

	_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.8ball"))
	helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
}
