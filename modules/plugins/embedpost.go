package plugins

import (
	"strings"

	"time"

	"github.com/Jeffail/gabs"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
)

type EmbedPost struct{}

func (m *EmbedPost) Commands() []string {
	return []string{
		"embedpost",
		"embed",
		"edit-embed",
		"embed-edit",
		"get-embed",
		"embed-get",
	}
}

func (m *EmbedPost) Init(session *discordgo.Session) {

}

func (m *EmbedPost) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermEmbedPost) {
		return
	}

	helpers.RequireMod(msg, func() {
		args := strings.Split(content, " ")

		if len(args) < 2 {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
			return
		}

		var targetMessage *discordgo.Message
		targetChannel, err := helpers.GetChannelFromMention(msg, args[0])
		if err != nil {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
			return
		}

		embedText := strings.TrimSpace(strings.Replace(content, args[0], "", 1))

		if command == "edit-embed" || command == "embed-edit" || command == "get-embed" || command == "embed-get" {
			if len(args) < 2 {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
				return
			}

			messageID := args[1]
			embedText = strings.TrimSpace(strings.Replace(
				strings.Replace(content, args[0], "", 1), args[1], "", 1))

			targetMessage, err = session.ChannelMessage(targetChannel.ID, messageID)
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok {
					if errD.Message.Code == discordgo.ErrCodeUnknownMessage || strings.Contains(err.Error(), "is not snowflake") {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
						return
					} else {
						helpers.Relax(err)
					}
				} else {
					helpers.Relax(err)
				}
			}

			if command == "get-embed" || command == "embed-get" {
				if targetMessage.Embeds == nil || len(targetMessage.Embeds) <= 0 {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					return
				}

				_, err = helpers.SendMessageBoxed(msg.ChannelID, helpers.GetEmbedCode(targetMessage))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)

				return
			}
		}

		if len(args) < 3 {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
			return
		}

		ptext, embed, err := helpers.ParseEmbedCode(embedText)
		if err != nil {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
			return
		}

		if targetMessage == nil {
			newMessages, err := helpers.SendComplex(targetChannel.ID, &discordgo.MessageSend{
				Content: ptext,
				Embed:   embed,
			})
			if err != nil && strings.Contains(err.Error(), "HTTP 400 Bad Request") {
				if errD, ok := err.(*discordgo.RESTError); ok {
					container, err := gabs.ParseJSON(errD.ResponseBody)
					if err == nil {
						if container.Exists("embed") {
							children, err := container.Path("embed").Children()
							if err == nil {
								errorMessage := "Please check the following field(s) of your embed: "
								for _, entry := range children {
									errorMessage += strings.Trim(entry.String(), "\"") + ", "
								}
								errorMessage = strings.TrimSuffix(errorMessage, ", ")
								helpers.SendMessage(msg.ChannelID, errorMessage)
								return
							}
						}
					}
				}
			}
			helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)

			if len(newMessages) > 0 {
				session.MessageReactionAdd(msg.ChannelID, msg.ID, "👌")

				newMessageIDs := make([]string, 0)
				for _, newMessage := range newMessages {
					newMessageIDs = append(newMessageIDs, newMessage.ID)
				}
				_, err = helpers.EventlogLog(time.Now(), targetChannel.GuildID, strings.Join(newMessageIDs, ","),
					models.EventlogTargetTypeMessage, msg.Author.ID,
					models.EventlogTypeRobyulPostCreate, "",
					nil,
					[]models.ElasticEventlogOption{
						{
							Key:   "post_channelid",
							Value: targetChannel.ID,
							Type:  models.EventlogTargetTypeChannel,
						},
						{
							Key:   "post_embedcode",
							Value: embedText,
						},
					}, false)
				helpers.RelaxLog(err)
			}
		} else {
			editMessage, err := helpers.EditComplex(&discordgo.MessageEdit{
				Content: &ptext,
				Embed:   embed,
				ID:      targetMessage.ID,
				Channel: targetChannel.ID,
			})
			helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)

			if editMessage != nil {
				session.MessageReactionAdd(msg.ChannelID, msg.ID, "👌")

				_, err = helpers.EventlogLog(time.Now(), targetChannel.GuildID, editMessage.ID,
					models.EventlogTargetTypeMessage, msg.Author.ID,
					models.EventlogTypeRobyulPostUpdate, "",
					[]models.ElasticEventlogChange{
						{
							Key:      "post_embedcode",
							OldValue: helpers.GetEmbedCode(targetMessage),
							NewValue: embedText,
						},
					},
					[]models.ElasticEventlogOption{
						{
							Key:   "post_channelid",
							Value: targetChannel.ID,
							Type:  models.EventlogTargetTypeChannel,
						},
					}, false)
				helpers.RelaxLog(err)
			}
		}
	})
}
