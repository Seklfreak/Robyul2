package helpers

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

func GetEmbedCode(targetMessage *discordgo.Message) (embedCode string) {
	if targetMessage == nil {
		return
	}

	if len(targetMessage.Content) > 0 {
		embedCode += "ptext=" + CleanEmbedValue(targetMessage.Content) + " | "
	}

	if targetMessage.Embeds == nil || len(targetMessage.Embeds) <= 0 {
		return
	}

	targetEmbed := targetMessage.Embeds[0]

	if targetEmbed.Author != nil && targetEmbed.Author.Name != "" {
		if targetEmbed.Author.URL == "" && targetEmbed.Author.IconURL == "" {
			embedCode += "author=" + CleanEmbedValue(targetEmbed.Author.Name) + " | "
		} else {
			embedCode += "author=name=" + CleanEmbedValue(targetEmbed.Author.Name)
			if targetEmbed.Author.URL != "" {
				embedCode += " url=" + CleanEmbedValue(targetEmbed.Author.URL)
			}
			if targetEmbed.Author.IconURL != "" {
				embedCode += " icon=" + CleanEmbedValue(targetEmbed.Author.IconURL)
			}
			embedCode += " | "
		}
	}
	if targetEmbed.Title != "" {
		embedCode += "title=" + CleanEmbedValue(targetEmbed.Title) + " | "
	}
	if targetEmbed.Description != "" {
		embedCode += "description=" + CleanEmbedValue(targetEmbed.Description) + " | "
	}
	if targetEmbed.Thumbnail != nil && targetEmbed.Thumbnail.URL != "" {
		embedCode += "thumbnail=" + CleanEmbedValue(targetEmbed.Thumbnail.URL) + " | "
	}
	if targetEmbed.Image != nil && targetEmbed.Image.URL != "" {
		embedCode += "image=" + CleanEmbedValue(targetEmbed.Image.URL) + " | "
	}
	if targetEmbed.Fields != nil && len(targetEmbed.Fields) > 0 {
		for _, targetField := range targetEmbed.Fields {
			if targetField.Inline {
				embedCode += "field=name=" + CleanEmbedValue(targetField.Name) +
					" value=" + CleanEmbedValue(targetField.Value) + " | "
			} else {
				embedCode += "field=name=" + CleanEmbedValue(targetField.Name) +
					" value=" + CleanEmbedValue(targetField.Value) + " inline=no | "
			}
		}
	}
	if targetEmbed.Footer != nil && targetEmbed.Footer.Text != "" {
		if targetEmbed.Footer.IconURL == "" {
			embedCode += "footer=" + CleanEmbedValue(targetEmbed.Footer.Text)
		} else {
			embedCode += "footer=name=" + CleanEmbedValue(targetEmbed.Footer.Text) +
				" icon=" + CleanEmbedValue(targetEmbed.Footer.IconURL)
		}
		embedCode += " | "
	}
	if targetEmbed.Color > 0 {
		embedCode += "color=#" + GetHexFromDiscordColor(targetEmbed.Color) + " | "
	}

	embedCode = strings.TrimSuffix(embedCode, " | ")

	return ReplaceEmojis(embedCode)
}

func CleanEmbedValue(input string) (output string) {
	return strings.Replace(input, "|", "-", -1)
}
