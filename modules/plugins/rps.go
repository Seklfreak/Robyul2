package plugins

import (
	"regexp"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/shardmanager"
	"github.com/bwmarrin/discordgo"
)

type RPS struct{}

func (r *RPS) Commands() []string {
	return []string{
		"rps",
	}
}

func (r *RPS) Init(session *shardmanager.Manager) {

}

func (r *RPS) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermGames) {
		return
	}

	switch {
	case regexp.MustCompile("(?i)(rock|stone)").MatchString(content):
		helpers.SendMessage(msg.ChannelID, "I've chosen :newspaper:\nMy paper wraps your stone.\nI win <a:ablobsmile:393869335312990209>")
		return

	case regexp.MustCompile("(?i)paper").MatchString(content):
		helpers.SendMessage(msg.ChannelID, "I've chosen :scissors:\nMy scissors cuts your paper!\nI win <a:ablobsmile:393869335312990209>")
		return

	case regexp.MustCompile("(?i)scissors").MatchString(content):
		helpers.SendMessage(msg.ChannelID, "I've chosen :white_large_square:\nMy stone breaks your scissors.\nI win <a:ablobsmile:393869335312990209>")
		return
	}

	helpers.SendMessage(msg.ChannelID, "That's an odd or invalid choice for RPS <:blobneutral:317029459720929281>")
}
