package plugins

import (
	"fmt"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/version"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"runtime"
	"strconv"
	"time"
	"strings"
)

type Stats struct{}

func (s *Stats) Commands() []string {
	return []string{
		"stats",
		"serverinfo",
		"userinfo",
	}
}

func (s *Stats) Init(session *discordgo.Session) {

}

func (s *Stats) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	switch command {
	case "stats":
		// Count guilds, channels and users
		users := make(map[string]string)
		channels := 0
		guilds := session.State.Guilds

		for _, guild := range guilds {
			channels += len(guild.Channels)

			lastAfterMemberId := ""
			for {
				members, err := session.GuildMembers(guild.ID, lastAfterMemberId, 1000)
				if len(members) <= 0 {
					break
				}
				lastAfterMemberId = members[len(members)-1].User.ID
				helpers.Relax(err)
				for _, u := range members {
					users[u.User.ID] = u.User.Username
				}
			}
		}

		// Get RAM stats
		var ram runtime.MemStats
		runtime.ReadMemStats(&ram)

		// Get uptime
		bootTime, err := strconv.ParseInt(metrics.Uptime.String(), 10, 64)
		if err != nil {
			bootTime = 0
		}

		uptime := time.Now().Sub(time.Unix(bootTime, 0)).String()

		session.ChannelMessageSendEmbed(msg.ChannelID, &discordgo.MessageEmbed{
			Color: 0x0FADED,
			Thumbnail: &discordgo.MessageEmbedThumbnail{
				URL: fmt.Sprintf(
					"https://cdn.discordapp.com/avatars/%s/%s.jpg",
					session.State.User.ID,
					session.State.User.Avatar,
				),
			},
			Fields: []*discordgo.MessageEmbedField{
				// Build
				{Name: "Build Time", Value: version.BUILD_TIME, Inline: false},
				{Name: "Build System", Value: version.BUILD_USER + "@" + version.BUILD_HOST, Inline: false},

				// System
				{Name: "Bot Uptime", Value: uptime, Inline: true},
				{Name: "Bot Version", Value: version.BOT_VERSION, Inline: true},
				{Name: "GO Version", Value: runtime.Version(), Inline: true},

				// Bot
				{Name: "Used RAM", Value: humanize.Bytes(ram.Alloc) + "/" + humanize.Bytes(ram.Sys), Inline: true},
				{Name: "Collected garbage", Value: humanize.Bytes(ram.TotalAlloc), Inline: true},
				{Name: "Running coroutines", Value: strconv.Itoa(runtime.NumGoroutine()), Inline: true},

				// Discord
				{Name: "Connected servers", Value: strconv.Itoa(len(guilds)), Inline: true},
				{Name: "Watching channels", Value: strconv.Itoa(channels), Inline: true},
				{Name: "Users with access to me", Value: strconv.Itoa(len(users)), Inline: true},

				// Link
				{Name: "Want more stats and awesome graphs?", Value: "Visit my [datadog dashboard](https://p.datadoghq.com/sb/066f13da3-7607f827de)", Inline: false},
			},
		})
	case "serverinfo":
		session.ChannelTyping(msg.ChannelID)
		currentChannel, err := session.Channel(msg.ChannelID)
		helpers.Relax(err)
		guild, err := session.Guild(currentChannel.GuildID)
		helpers.Relax(err)
		users := make(map[string]string)
		lastAfterMemberId := ""
		for {
			members, err := session.GuildMembers(guild.ID, lastAfterMemberId, 1000)
			helpers.Relax(err)
			if len(members) <= 0 {
				break
			}

			lastAfterMemberId = members[len(members)-1].User.ID
			for _, u := range members {
				users[u.User.ID] = u.User.Username
			}
		}

		textChannels := 0
		voiceChannels := 0
		for _, channel := range guild.Channels {
			if channel.Type == "voice" {
				voiceChannels += 1
			} else if channel.Type == "text" {
				textChannels += 1
			}
		}
		online := 0
		for _, presence := range guild.Presences{
			if presence.Status == discordgo.StatusOnline || presence.Status == discordgo.StatusDoNotDisturb || presence.Status == discordgo.StatusIdle {
				online += 1
			}
		}

		createdAtTime := helpers.GetTimeFromSnowflake(guild.ID)

		owner, err := session.User(guild.OwnerID)
		helpers.Relax(err)
		member, err := session.GuildMember(guild.ID, guild.OwnerID)
		helpers.Relax(err)
		ownerText := fmt.Sprintf("%s#%s", owner.Username, owner.Discriminator)
		if member.Nick != "" {
			ownerText = fmt.Sprintf("%s#%s ~ %s", owner.Username, owner.Discriminator, member.Nick)
		}

		emoteText := "None"
		emoteN := 0
		for _, emote := range guild.Emojis {
			if emoteN == 0 {
				emoteText = fmt.Sprintf("`%s`", emote.Name)
			} else {

				emoteText += fmt.Sprintf(", `%s`", emote.Name)
			}
			emoteN += 1
		}
		if emoteText != "None" {
			emoteText += fmt.Sprintf(" (%d in Total)", emoteN)
		}

		serverinfoEmbed := &discordgo.MessageEmbed{
			Color: 0x0FADED,
			Title: guild.Name,
			Description: fmt.Sprintf("Since: %s. That's %s.", createdAtTime.Format(time.ANSIC), helpers.SinceInDaysText(createdAtTime)),
			Footer: &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Server ID: %s", guild.ID)},
			Fields: []*discordgo.MessageEmbedField{
				{Name: "Region", Value: guild.Region, Inline: true},
				{Name: "Users", Value: fmt.Sprintf("%d/%d", online, len(users)), Inline: true},
				{Name: "Text Channels", Value: strconv.Itoa(textChannels), Inline: true},
				{Name: "Voice Channels", Value: strconv.Itoa(voiceChannels), Inline: true},
				{Name: "Roles", Value: strconv.Itoa(len(guild.Roles)), Inline: true},
				{Name: "Owner", Value: ownerText, Inline: true},
				{Name: "Emotes", Value: emoteText, Inline: false},
			},
		}

		if guild.Icon != "" {
			serverinfoEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: fmt.Sprintf("https://cdn.discordapp.com/icons/%s/%s.jpg", guild.ID, guild.Icon) }
		}

		_, err = session.ChannelMessageSendEmbed(msg.ChannelID, serverinfoEmbed)
		helpers.Relax(err)
	case "userinfo":
		session.ChannelTyping(msg.ChannelID)
		targetUser, err := session.User(msg.Author.ID)
		helpers.Relax(err)
		args := strings.Split(content, " ")
		if len(args) >= 1 && args[0] != "" {
			targetUser, err = helpers.GetUserFromMention(args[0])
			helpers.Relax(err)
			if targetUser.ID == "" {
				_, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
				helpers.Relax(err)
			}
		}

		currentChannel, err := session.Channel(msg.ChannelID)
		helpers.Relax(err)
		currentGuild, err := session.Guild(currentChannel.GuildID)
		helpers.Relax(err)
		targetMember, err := session.GuildMember(currentGuild.ID, targetUser.ID)
		helpers.Relax(err)

		status := ""
		game := ""
		gameUrl := ""
		for _, presence := range currentGuild.Presences{
			if presence.User.ID == targetUser.ID {
				status = string(presence.Status)
				switch status {
				case "dnd":
					status = "Do Not Disturb"
				case "idle":
					status = "Away"
				}
				if presence.Game != nil {
					game = presence.Game.Name
					gameUrl = presence.Game.URL
				}
			}
		}
		nick := ""
		if targetMember.Nick != "" {
			nick = targetMember.Nick
		}
		description := fmt.Sprintf("**%s**", status)
		if game != "" {
			description = fmt.Sprintf("**%s** (Playing: **%s**)", status, game)
			if gameUrl != "" {
				description = fmt.Sprintf("**%s** (:mega: Streaming: **%s**)", status, game)
			}
		}
		title := fmt.Sprintf("%s#%s", targetUser.Username, targetUser.Discriminator)
		if nick != "" {
			title = fmt.Sprintf("%s#%s ~ %s", targetUser.Username, targetUser.Discriminator, nick)
		}
		rolesText := "None"
		guildRoles, err := session.GuildRoles(currentGuild.ID)
		helpers.Relax(err)
		isFirst := true
		for _, guildRole := range guildRoles {
			for _, userRole := range targetMember.Roles {
				if guildRole.ID == userRole {
					if isFirst == true {
						rolesText = fmt.Sprintf("%s", guildRole.Name)
					} else {

						rolesText += fmt.Sprintf(", %s", guildRole.Name)
					}
					isFirst = false
				}
			}
		}

		joinedTime := helpers.GetTimeFromSnowflake(targetUser.ID)
		joinedServerTime, err := discordgo.Timestamp(targetMember.JoinedAt).Parse()
		helpers.Relax(err)

		userinfoEmbed := &discordgo.MessageEmbed{
			Color: 0x0FADED,
			Title: title,
			Description: description,
			Footer: &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("User ID: %s", targetUser.ID)},
			Fields: []*discordgo.MessageEmbedField{
				{Name: "Joined Discord on", Value: fmt.Sprintf("%s (%s)", joinedTime.Format(time.ANSIC), helpers.SinceInDaysText(joinedTime)), Inline: true},
				{Name: "Joined this server on", Value: fmt.Sprintf("%s (%s)", joinedServerTime.Format(time.ANSIC), helpers.SinceInDaysText(joinedServerTime)), Inline: true},
				{Name: "Roles", Value: rolesText, Inline: false},
			},
		}

		if helpers.GetAvatarUrl(targetUser) != "" {
			userinfoEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: helpers.GetAvatarUrl(targetUser)}
		}
		if gameUrl != "" {
			userinfoEmbed.URL = gameUrl
		}

		_, err = session.ChannelMessageSendEmbed(msg.ChannelID, userinfoEmbed)
		helpers.Relax(err)
		// TODO: Fix Role Order
		// TODO: Show Member #
	}
}
