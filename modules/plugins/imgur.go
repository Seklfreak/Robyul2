package plugins

import (
	"strings"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/shardmanager"
	"github.com/bwmarrin/discordgo"
)

type Imgur struct{}

func (i *Imgur) Commands() []string {
	return []string{
		"imgur",
	}
}

func (i *Imgur) Init(session *shardmanager.Manager) {

}

func (i *Imgur) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermImgur) {
		return
	}

	session.ChannelTyping(msg.ChannelID)

	args := strings.Split(content, " ")

	var sourceUrl string
	if len(args) >= 1 {
		sourceUrl = args[0]
	}
	if len(msg.Attachments) >= 1 {
		sourceUrl = msg.Attachments[0].URL
	}

	if sourceUrl == "" {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}

	sourceData, err := helpers.NetGetUAWithError(sourceUrl, helpers.DEFAULT_UA)
	if err != nil {
		if strings.Contains(err.Error(), "unsupported protocol scheme") {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
			return
		}
	}
	helpers.Relax(err)

	newLink, err := helpers.UploadImage(sourceData)
	if err != nil {
		if strings.Contains(err.Error(), "Invalid URL") {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
			return
		}
	}
	helpers.Relax(err)

	_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.imgur.success", newLink))
	helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
}
