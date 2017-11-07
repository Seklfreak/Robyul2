package plugins

import (
	"strings"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

type Headpat struct{}

func (h *Headpat) Commands() []string {
	return []string{
		"headpat",
		"pat",
	}
}

func (h *Headpat) Init(session *discordgo.Session) {

}

func (h *Headpat) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	// Check mentions in the message
	mentionUsers := len(msg.Mentions)

	// Delete spaces from params
	params := strings.TrimSpace(content)

	// Case 1: pat yourself
	if params == "me" || mentionUsers == 1 && (msg.Author.ID == msg.Mentions[0].ID) {
		helpers.SendMessage(msg.ChannelID,
			helpers.GetText("bot.mentions.pat-yourself")+"\n"+"https://media.giphy.com/media/wUArrd4mE3pyU/giphy.gif",
		)
		return
	}

	// Case 2: pat @User#1234
	if mentionUsers == 1 {
		helpers.SendMessage(msg.ChannelID,
			helpers.GetTextF(
				"triggers.headpat.msg",
				msg.Author.ID,
				msg.Mentions[0].ID,
			)+"\n"+helpers.GetText("triggers.headpat.link"),
		)
		return
	}

	// Case 3: pat multiple users
	if msg.MentionEveryone || mentionUsers > 1 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.mentions.pat-group"))
		return
	}

	// Case 4: no params || wrong params
	helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.mentions.who-to-pat"))
}
