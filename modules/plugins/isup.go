package plugins

import (
	"strings"

	"net/url"

	"time"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

type Isup struct{}

func (iu *Isup) Commands() []string {
	return []string{
		"isup",
	}
}

func (iu *Isup) Init(session *discordgo.Session) {
}

func (iu *Isup) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermIsup) {
		return
	}

	args := strings.Fields(content)

	if len(args) < 1 {
		helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
		return
	}

	quitChannel := helpers.StartTypingLoop(msg.ChannelID)
	defer func() { quitChannel <- 0 }()

	text := helpers.GetText("plugins.isup.isnotup")
	if iu.isup(args[0]) {
		text = helpers.GetText("plugins.isup.isup")
	}
	text += "\n" + helpers.GetText("plugins.isup.credits")

	quitChannel <- 0
	_, err := helpers.SendMessage(msg.ChannelID, text)
	helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
	return
}

func (iu *Isup) isup(link string) (isup bool) {
	resultBytes, err := helpers.NetGetUAWithErrorAndTimeout(
		"http://www.isup.me/"+url.QueryEscape(link), helpers.DEFAULT_UA, time.Duration(60*time.Second))
	helpers.Relax(err)

	if strings.Contains(string(resultBytes), "It's just you") {
		return true
	}
	return false
}
