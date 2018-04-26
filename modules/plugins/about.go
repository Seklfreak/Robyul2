package plugins

import (
	"strings"

	"fmt"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

type About struct{}

func (a *About) Commands() []string {
	return []string{
		"about",
		"info",
	}
}

func (a *About) Init(session *discordgo.Session) {

}

func (a *About) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	channel, err := helpers.GetChannel(msg.ChannelID)
	helpers.Relax(err)

	embed := &discordgo.MessageEmbed{
		URL:         "https://robyul.chat/",
		Title:       "Hello, I'm Robyul!",
		Description: "I'm built using Go, open-source and a fork of Shiro, formerly called Karen.",
		Color:       0,
		Author: &discordgo.MessageEmbedAuthor{
			URL:     "https://robyul.chat/",
			Name:    "Robyul - The KPop Discord Bot",
			IconURL: session.State.User.AvatarURL("64"),
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   ":earth_asia: Website",
				Value:  "https://robyul.chat/",
				Inline: false,
			},
			{
				Name:   ":tools: Source",
				Value:  "https://github.com/Seklfreak/Robyul2",
				Inline: false,
			},
			{
				Name:   ":speech_left: Support Discord",
				Value:  "https://discord.gg/s5qZvUV",
				Inline: false,
			},
			{
				Name:   ":railway_track: Trello",
				Value:  "https://trello.robyul.chat/",
				Inline: false,
			},
			{
				Name:   ":vertical_traffic_light: Status",
				Value:  "https://status.robyul.chat/",
				Inline: false,
			},
			{
				Name:   ":cop: Team",
				Value:  strings.Replace(helpers.GetStaffUsernamesText(), "or", "and", -1),
				Inline: false,
			},
			{
				Name: ":bulb: Tip",
				Value: fmt.Sprintf("Make suggestions for Robyul: `%ssuggest <your suggestion>`!",
					helpers.GetPrefixForServer(channel.GuildID)),
				Inline: false,
			},
			{
				Name:   ":video_game: Bias Game",
				Value:  "Special thanks to Gailloune who created the original version of this game with his bot Watermelon Queen on the CLC Discord. With his help we were able to add the game to Robyul. Thanks Gai! <:SornHype:428235062925066250>",
				Inline: false,
			},
		},
	}

	helpers.SendEmbed(msg.ChannelID, embed)
}
