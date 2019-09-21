package mod

import (
	"fmt"
	"strings"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

// kickHander [p]kick <User> [<Reason>], checks for IsMod and Kick Permissions
func kickHander(msg *discordgo.Message, content string, confirmation bool) {
	if !helpers.IsMod(msg) {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("mod.no_permission"))
		return
	}

	args := strings.Fields(content)
	if len(args) < 1 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
		return
	}

	var usersToKick []*discordgo.User

	var offset int
	for _, arg := range args {
		targetUser, err := helpers.GetUserFromMention(arg)
		if err != nil {
			break
		}

		usersToKick = append(usersToKick, targetUser)

		offset++

		// up to 10 users at once
		if offset >= 10 {
			break
		}
	}

	usersToKick = helpers.UniqueUsers(usersToKick)

	if len(usersToKick) <= 0 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}

	// Bot can kick?
	var botCanKick bool
	guild, err := helpers.GetGuild(msg.GuildID)
	helpers.Relax(err)
	guildMemberBot, err := helpers.GetGuildMember(guild.ID, cache.GetSession().SessionForGuildS(msg.GuildID).State.User.ID)
	helpers.Relax(err)
	for _, role := range guild.Roles {
		for _, userRole := range guildMemberBot.Roles {
			if userRole == role.ID &&
				(role.Permissions&discordgo.PermissionKickMembers == discordgo.PermissionKickMembers ||
					role.Permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator) {
				botCanKick = true
			}
		}
	}

	if !botCanKick {
		_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.mod.bot-disallowed"))
		helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
		return
	}

	// User can kick?
	var userCanKick bool
	guildMemberUser, err := helpers.GetGuildMember(guild.ID, msg.Author.ID)
	helpers.Relax(err)
	for _, role := range guild.Roles {
		for _, userRole := range guildMemberUser.Roles {
			if userRole == role.ID &&
				(role.Permissions&discordgo.PermissionKickMembers == discordgo.PermissionKickMembers ||
					role.Permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator) {
				userCanKick = true
			}
		}
	}
	if msg.Author.ID == guild.OwnerID {
		userCanKick = true
	}

	if !userCanKick {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.disallowed"))
		return
	}

	// Get Reason
	reasonText := fmt.Sprintf(
		"Issued by: %s#%s (#%s) | Reason: ",
		msg.Author.Username, msg.Author.Discriminator, msg.Author.ID,
	)

	if len(args) >= offset+1 {
		reasonText += strings.TrimSpace(strings.Replace(content, strings.Join(args[:offset], " "), "", 1))
	}

	if strings.HasSuffix(reasonText, "Reason: ") {
		reasonText += "None given"
	}

	// Kick user, if confirmed
	var usersToKickText string
	for _, userToKick := range usersToKick {
		usersToKickText += "`" + userToKick.String() + "`, "
	}
	usersToKickText = strings.TrimRight(usersToKickText, ", ")

	if !confirmation ||
		helpers.ConfirmEmbed(
			msg.GuildID, msg.ChannelID, msg.Author,
			helpers.GetTextF(
				"plugins.mod.confirm-kick",
				usersToKickText,
				reasonText,
			), "✅", "🚫") {
		for _, userToKick := range usersToKick {
			err = cache.GetSession().SessionForGuildS(msg.GuildID).GuildMemberDeleteWithReason(guild.ID, userToKick.ID, reasonText)
			if err != nil {
				if err, ok := err.(*discordgo.RESTError); ok && err.Message != nil {
					if err.Message.Code == 0 {
						_, err := helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.mod.user-kicked-failed-too-low"))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					} else {
						helpers.Relax(err)
						return
					}
				} else {
					helpers.Relax(err)
					return
				}
			}
			cache.GetLogger().WithField("module", "mod").Info(fmt.Sprintf(
				"Kicked User %s (#%s) on Guild %s (#%s) by %s (#%s)",
				userToKick.Username, userToKick.ID, guild.Name, guild.ID, msg.Author.Username, msg.Author.ID,
			))
			_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.mod.user-kicked-success", userToKick.Username, userToKick.ID))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
		}
	}
}
