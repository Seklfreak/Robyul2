package plugins

import (
	"strings"

	"fmt"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

type DiscordMoney struct{}

func (dm *DiscordMoney) Commands() []string {
	return []string{
		"discordmoney",
	}
}

func (dm *DiscordMoney) Init(session *discordgo.Session) {

}

func (dm *DiscordMoney) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	channel, err := helpers.GetChannel(msg.ChannelID)
	helpers.Relax(err)

	guild, err := helpers.GetGuild(channel.GuildID)
	helpers.Relax(err)

	var nitroUsers int
	for _, member := range guild.Members {
		if strings.HasPrefix(member.User.Avatar, "a_") {
			nitroUsers++
		}
	}

	_, err = helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Found %d nitro users. That's **$ %d** per month! :money_with_wings:", nitroUsers, nitroUsers*5))
	helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
}
