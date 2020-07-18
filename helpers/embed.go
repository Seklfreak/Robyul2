package helpers

import (
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
)

func IsEmbedCode(embedText string) (isCode bool) {
	if strings.Contains(embedText, "ptext=") {
		return true
	}
	if strings.Contains(embedText, "title=") {
		return true
	}
	if strings.Contains(embedText, "description=") {
		return true
	}
	if strings.Contains(embedText, "desc=") {
		return true
	}
	if strings.Contains(embedText, "thumbnail=") {
		return true
	}
	if strings.Contains(embedText, "title=") {
		return true
	}
	if strings.Contains(embedText, "colour=") {
		return true
	}
	if strings.Contains(embedText, "color=") {
		return true
	}
	if strings.Contains(embedText, "footer=") {
		return true
	}
	if strings.Contains(embedText, "author=") {
		return true
	}
	return false
}

func ParseEmbedCode(embedText string) (ptext string, embed *discordgo.MessageEmbed, err error) {
	// Code ported from https://github.com/appu1232/Discord-Selfbot/blob/master/cogs/misc.py#L146
	// Reference https://github.com/Seklfreak/Robyul-Web/blob/master/src/RobyulWebBundle/Resources/public/js/main.js#L724
	var title, description, image, thumbnail, color, footer, author string
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
		return embedText, nil, errors.New("invalid embed code")
	}

	embed = &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
	}
	if !timestamp.IsZero() {
		embed.Timestamp = timestamp.Format(time.RFC3339)
	}
	if color != "" {
		embed.Color = GetDiscordColorFromHex(color)
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
				if len(authorValues) >= 2 {
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

	return ptext, embed, nil
}
