package mod

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

// banHandler [p]ban <User> [<Days>] [<Reason>], checks for IsMod and Ban Permissions
func banHandler(msg *discordgo.Message, content string, confirmation bool) {
	if !helpers.IsMod(msg) {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("mod.no_permission"))
		return
	}

	args := strings.Fields(content)
	if len(args) < 1 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
		return
	}

	var usersToBan []*discordgo.User

	var offset int
	for _, arg := range args {
		targetUser, err := helpers.GetUserFromMention(arg)
		if err != nil {
			break
		}

		usersToBan = append(usersToBan, targetUser)

		offset++

		// up to 10 users at once
		if offset >= 10 {
			break
		}
	}

	usersToBan = helpers.UniqueUsers(usersToBan)

	if len(usersToBan) <= 0 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}

	// Days Argument
	days := 0
	var err error

	if len(args) >= offset+1 {
		dayArg := args[offset]

		if regexNumberOnly.MatchString(dayArg) {
			days, err = strconv.Atoi(dayArg)
			if err != nil {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				return
			}
			if days > 7 {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.user-banned-error-too-many-days"))
				return
			}

			offset++
		}
	}

	// Bot can ban?
	var botCanBan bool
	guild, err := helpers.GetGuild(msg.GuildID)
	helpers.Relax(err)
	guildMemberBot, err := helpers.GetGuildMember(guild.ID, cache.GetSession().SessionForGuildS(msg.GuildID).State.User.ID)
	helpers.Relax(err)
	for _, role := range guild.Roles {
		for _, userRole := range guildMemberBot.Roles {
			if userRole == role.ID &&
				(role.Permissions&discordgo.PermissionBanMembers == discordgo.PermissionBanMembers ||
					role.Permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator) {
				botCanBan = true
			}
		}
	}

	if !botCanBan {
		_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.mod.bot-disallowed"))
		helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
		return
	}

	// User can ban?
	var userCanBan bool
	guildMemberUser, err := helpers.GetGuildMember(guild.ID, msg.Author.ID)
	helpers.Relax(err)
	for _, role := range guild.Roles {
		for _, userRole := range guildMemberUser.Roles {
			if userRole == role.ID &&
				(role.Permissions&discordgo.PermissionBanMembers == discordgo.PermissionBanMembers ||
					role.Permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator) {
				userCanBan = true
			}
		}
	}
	if msg.Author.ID == guild.OwnerID {
		userCanBan = true
	}

	if !userCanBan {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.disallowed"))
		return
	}

	// Get Reason
	reasonText := fmt.Sprintf(
		"Issued by: %s#%s (#%s) | Delete Days: %d | Reason: ",
		msg.Author.Username, msg.Author.Discriminator, msg.Author.ID, days,
	)

	if len(args) >= offset+1 {
		reasonText += strings.TrimSpace(strings.Replace(content, strings.Join(args[:offset], " "), "", 1))
	}

	if strings.HasSuffix(reasonText, "Reason: ") {
		reasonText += "None given"
	}

	// Ban user, if confirmed
	var usersToBanText string
	for _, userToBan := range usersToBan {
		usersToBanText += "`" + userToBan.String() + "`, "
	}
	usersToBanText = strings.TrimRight(usersToBanText, ", ")

	if !confirmation ||
		helpers.ConfirmEmbed(
			msg.GuildID, msg.ChannelID, msg.Author,
			helpers.GetTextF(
				"plugins.mod.confirm-ban",
				usersToBanText,
				days,
				reasonText,
			), "✅", "🚫") {
		for _, userToBan := range usersToBan {
			err = cache.GetSession().SessionForGuildS(msg.GuildID).GuildBanCreateWithReason(guild.ID, userToBan.ID, reasonText, days)
			if err != nil {
				if err, ok := err.(*discordgo.RESTError); ok && err.Message != nil {
					if err.Message.Code == 0 {
						_, err := helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.mod.user-banned-failed-too-low"))
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
				"Banned User %s (#%s) on Guild %s (#%s) by %s (#%s)",
				userToBan.Username, userToBan.ID, guild.Name, guild.ID, msg.Author.Username, msg.Author.ID,
			))
			_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.mod.user-banned-success", userToBan.Username, userToBan.ID))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
		}
	}
}
