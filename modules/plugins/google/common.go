package google

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

func logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "google")
}

func linkResultEmbed(searchResult linkResult) (embed *discordgo.MessageEmbed) {
	embed = &discordgo.MessageEmbed{}

	embed.Footer = &discordgo.MessageEmbedFooter{
		Text:    helpers.GetText("plugins.google.embed-footer"),
		IconURL: helpers.GetText("plugins.google.embed-footer-imageurl"),
	}
	embed.Title = searchResult.Title
	embed.URL = searchResult.Link
	embed.Description = searchResult.Text

	return embed
}

func imageResultEmbed(searchResult imageResult) (embed *discordgo.MessageEmbed) {
	embed = &discordgo.MessageEmbed{}

	embed.Footer = &discordgo.MessageEmbedFooter{
		Text:    helpers.GetText("plugins.google.embed-footer"),
		IconURL: helpers.GetText("plugins.google.embed-footer-imageurl"),
	}
	embed.Title = searchResult.Title
	embed.URL = searchResult.Link
	embed.Image = &discordgo.MessageEmbedImage{
		URL: searchResult.URL,
	}

	return embed
}
