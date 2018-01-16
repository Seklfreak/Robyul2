package plugins

import (
	"strings"

	"time"

	"fmt"

	"github.com/Jeffail/gabs"
	"github.com/Seklfreak/Robyul2/helpers"
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

				targetEmbed := targetMessage.Embeds[0]
				var embedCode string

				if len(targetMessage.Content) > 0 {
					embedCode += "ptext=" + m.cleanEmbedValue(targetMessage.Content) + " | "
				}
				if targetEmbed.Author != nil && targetEmbed.Author.Name != "" {
					if targetEmbed.Author.URL == "" && targetEmbed.Author.IconURL == "" {
						embedCode += "author=" + m.cleanEmbedValue(targetEmbed.Author.Name) + " | "
					} else {
						embedCode += "author=name=" + m.cleanEmbedValue(targetEmbed.Author.Name)
						if targetEmbed.Author.URL != "" {
							embedCode += " url=" + m.cleanEmbedValue(targetEmbed.Author.URL)
						}
						if targetEmbed.Author.IconURL != "" {
							embedCode += " icon=" + m.cleanEmbedValue(targetEmbed.Author.IconURL)
						}
						embedCode += " | "
					}
				}
				if targetEmbed.Title != "" {
					embedCode += "title=" + m.cleanEmbedValue(targetEmbed.Title) + " | "
				}
				if targetEmbed.Description != "" {
					embedCode += "description=" + m.cleanEmbedValue(targetEmbed.Description) + " | "
				}
				if targetEmbed.Thumbnail != nil && targetEmbed.Thumbnail.URL != "" {
					embedCode += "thumbnail=" + m.cleanEmbedValue(targetEmbed.Thumbnail.URL) + " | "
				}
				if targetEmbed.Image != nil && targetEmbed.Image.URL != "" {
					embedCode += "image=" + m.cleanEmbedValue(targetEmbed.Image.URL) + " | "
				}
				if targetEmbed.Fields != nil && len(targetEmbed.Fields) > 0 {
					for _, targetField := range targetEmbed.Fields {
						if targetField.Inline {
							embedCode += "field=name=" + m.cleanEmbedValue(targetField.Name) +
								" value=" + m.cleanEmbedValue(targetField.Value) + " | "
						} else {
							embedCode += "field=name=" + m.cleanEmbedValue(targetField.Name) +
								" value=" + m.cleanEmbedValue(targetField.Value) + " inline=no | "
						}
					}
				}
				if targetEmbed.Footer != nil && targetEmbed.Footer.Text != "" {
					if targetEmbed.Footer.IconURL == "" {
						embedCode += "footer=" + m.cleanEmbedValue(targetEmbed.Footer.Text)
					} else {
						embedCode += "footer=name=" + m.cleanEmbedValue(targetEmbed.Footer.Text) +
							" icon=" + m.cleanEmbedValue(targetEmbed.Footer.IconURL)
					}
					embedCode += " | "
				}
				if targetEmbed.Color > 0 {
					embedCode += "color=#" + helpers.GetHexFromDiscordColor(targetEmbed.Color) + " | "
				}

				embedCode = strings.TrimSuffix(embedCode, " | ")

				_, err = helpers.SendMessageBoxed(msg.ChannelID, embedCode)
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)

				return
			}
		}

		if len(args) < 3 {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
			return
		}

		// Code ported from https://github.com/appu1232/Discord-Selfbot/blob/master/cogs/misc.py#L146
		// Reference https://github.com/Seklfreak/Robyul-Web/blob/master/src/RobyulWebBundle/Resources/public/js/main.js#L724
		var ptext, title, description, image, thumbnail, color, footer, author string
		var timestamp time.Time

		embedValues := strings.Split(embedText, "|")
		for _, embedValue := range embedValues {
			embedValue = strings.TrimSpace(embedValue)
			if strings.HasPrefix(embedValue, "ptext=") {
				ptext = strings.TrimSpace(embedValue[6:])
			} else if strings.HasPrefix(embedValue, "title=") {
				title = strings.TrimSpace(embedValue[6:])
			} else if strings.HasPrefix(embedValue, "description=") {
				description = strings.TrimSpace(embedValue[12:])
			} else if strings.HasPrefix(embedValue, "desc=") {
				description = strings.TrimSpace(embedValue[5:])
			} else if strings.HasPrefix(embedValue, "image=") {
				image = strings.TrimSpace(embedValue[6:])
			} else if strings.HasPrefix(embedValue, "thumbnail=") {
				thumbnail = strings.TrimSpace(embedValue[10:])
			} else if strings.HasPrefix(embedValue, "colour=") {
				color = strings.TrimSpace(embedValue[7:])
			} else if strings.HasPrefix(embedValue, "color=") {
				color = strings.TrimSpace(embedValue[6:])
			} else if strings.HasPrefix(embedValue, "footer=") {
				footer = strings.TrimSpace(embedValue[7:])
			} else if strings.HasPrefix(embedValue, "author=") {
				author = strings.TrimSpace(embedValue[7:])
			} else if strings.HasPrefix(embedValue, "timestamp") {
				timestamp = time.Now()
			} else if description == "" && !strings.HasPrefix(embedValue, "field=") {
				description = embedValue
			}
		}

		if ptext == "" && title == "" && description == "" && image == "" && thumbnail == "" && footer == "" &&
			author == "" && !strings.Contains("field=", embedText) {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
			return
		}

		embed := discordgo.MessageEmbed{
			Title:       title,
			Description: description,
		}
		if !timestamp.IsZero() {
			embed.Timestamp = timestamp.Format(time.RFC3339)
		}
		if color != "" {
			embed.Color = helpers.GetDiscordColorFromHex(color)
		}

		var fieldValues []string
		var field, fieldName, fieldValue string
		var fieldInline bool
		for _, embedValue := range embedValues {
			embedValue = strings.TrimSpace(embedValue)
			if strings.HasPrefix(embedValue, "field=") {
				fieldInline = true
				field = strings.TrimSpace(strings.TrimPrefix(embedValue, "field="))
				fieldValues = strings.SplitN(field, "value=", 2)
				if len(fieldValues) >= 2 {
					fieldName = fieldValues[0]
					fieldValue = fieldValues[1]
				} else if len(fieldValues) >= 1 {
					fieldName = fieldValues[0]
					fieldValue = ""
				}
				if strings.Contains(fieldValue, "inline=") {
					fieldValues = strings.SplitN(fieldValue, "inline=", 2)
					if len(fieldValues) >= 2 {
						fieldValue = fieldValues[0]
						if strings.Contains(strings.ToLower(fieldValues[1]), "false") ||
							strings.Contains(strings.ToLower(fieldValues[1]), "no") {
							fieldInline = false
						}
					} else if len(fieldValues) >= 1 {
						fieldValue = fieldValues[0]
					}
				}
				fieldName = strings.TrimSpace(strings.TrimPrefix(fieldName, "name="))
				fieldValue = strings.TrimSpace(fieldValue)
				embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
					Name:   fieldName,
					Value:  fieldValue,
					Inline: fieldInline,
				})
			}
		}
		var authorValues, iconValues []string
		if author != "" {
			if strings.Contains(author, "icon=") {
				authorValues = strings.SplitN(author, "icon=", 2)
				if len(authorValues) >= 2 {
					if strings.Contains(authorValues[1], "url=") {
						iconValues = strings.SplitN(authorValues[1], "url=", 2)
						if len(iconValues) >= 2 {
							embed.Author = &discordgo.MessageEmbedAuthor{
								Name:    strings.TrimSpace(authorValues[0][5:]),
								IconURL: strings.TrimSpace(iconValues[0]),
								URL:     strings.TrimSpace(iconValues[1]),
							}
						}
					} else {
						embed.Author = &discordgo.MessageEmbedAuthor{
							Name:    strings.TrimSpace(authorValues[0][5:]),
							IconURL: strings.TrimSpace(authorValues[1]),
						}
					}
				}
			} else {
				if strings.Contains(author, "url=") {
					authorValues = strings.SplitN(author, "url=", 2)
					if len(iconValues) >= 2 {
						embed.Author = &discordgo.MessageEmbedAuthor{
							Name: strings.TrimSpace(authorValues[0][5:]),
							URL:  strings.TrimSpace(authorValues[1]),
						}
					}
				} else {
					embed.Author = &discordgo.MessageEmbedAuthor{
						Name: strings.TrimSpace(author),
					}
				}
			}
		}
		if image != "" {
			embed.Image = &discordgo.MessageEmbedImage{
				URL: image,
			}
		}
		if thumbnail != "" {
			embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
				URL: thumbnail,
			}
		}
		var footerValues []string
		if footer != "" {
			if strings.Contains(footer, "icon=") {
				footerValues = strings.SplitN(footer, "icon=", 2)
				if len(footerValues) >= 2 {
					embed.Footer = &discordgo.MessageEmbedFooter{
						Text:    strings.TrimSpace(footerValues[0])[5:],
						IconURL: strings.TrimSpace(footerValues[1]),
					}
				}
			} else {
				embed.Footer = &discordgo.MessageEmbedFooter{
					Text: strings.TrimSpace(footer),
				}
			}
		}

		if targetMessage == nil {
			newMessages, err := helpers.SendComplex(targetChannel.ID, &discordgo.MessageSend{
				Content: ptext,
				Embed:   &embed,
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
							} else {
								fmt.Println(err.Error())
							}
						}
					} else {
						fmt.Println(err.Error())
					}
				}
			}
			helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)

			if len(newMessages) > 0 {
				session.MessageReactionAdd(msg.ChannelID, msg.ID, "👌")
			}
		} else {
			editMessage, err := helpers.EditComplex(&discordgo.MessageEdit{
				Content: &ptext,
				Embed:   &embed,
				ID:      targetMessage.ID,
				Channel: targetChannel.ID,
			})
			helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)

			if editMessage != nil {
				session.MessageReactionAdd(msg.ChannelID, msg.ID, "👌")
			}
		}
	})
}

func (m *EmbedPost) cleanEmbedValue(input string) (output string) {
	return strings.Replace(input, "|", "-", -1)
}
