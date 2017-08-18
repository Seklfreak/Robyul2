package plugins

import (
	"strings"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

// WhoIs command
type WhoIs struct{}

// Commands for WhoIs
func (w *WhoIs) Commands() []string {
	return []string{
		"whois",
	}
}

// Init func
func (w *WhoIs) Init(s *discordgo.Session) {}

// Action will return info about the first @user
func (w *WhoIs) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	// Check if the msg contains at least 1 mention
	if len(msg.Mentions) == 0 {
		session.ChannelMessageSend(msg.ChannelID, "you need to @mention someone")
		return
	}

	// Get channel info
	channel, err := helpers.GetChannel(msg.ChannelID)
	if err != nil {
		cache.GetLogger().WithField("module", "whois").Error(err.Error())
		return
	}

	// Guild info
	guild, err := helpers.GetGuild(channel.GuildID)
	if err != nil {
		cache.GetLogger().WithField("module", "whois").Error(err.Error())
		return
	}

	// Get the member object for the @user
	target, err := helpers.GetGuildMember(guild.ID, msg.Mentions[0].ID)
	if err != nil {
		cache.GetLogger().WithField("module", "whois").Error(err.Error())
		return
	}

	// The roles name of the @user
	roles := []string{}
	for _, grole := range guild.Roles {
		for _, urole := range target.Roles {
			if urole == grole.ID {
				roles = append(roles, grole.Name)
			}
		}
	}

	joined, _ := time.Parse(time.RFC3339, target.JoinedAt)

	session.ChannelMessageSendEmbed(msg.ChannelID, &discordgo.MessageEmbed{
		Title: "Information about " + target.User.Username + "#" + target.User.Discriminator,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: helpers.GetAvatarUrl(target.User),
		},
		Color: 0x0FADED,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Joined server",
				Value:  joined.Format(time.RFC1123),
				Inline: true,
			},
			{
				Name:   "Joined Discord",
				Value:  helpers.GetTimeFromSnowflake(target.User.ID).Format(time.RFC1123),
				Inline: true,
			},
			{
				Name:   "Avatar link",
				Value:  helpers.GetAvatarUrl(target.User),
				Inline: false,
			},
			{
				Name:   "Roles",
				Value:  strings.Join(roles, ","),
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "UserID: " + target.User.ID,
		},
	})
}
