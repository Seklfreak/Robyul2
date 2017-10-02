package plugins

import (
	"fmt"
	"strings"

	"time"

	"github.com/RichardKnop/machinery/v1/tasks"
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/olebedev/when"
	"github.com/olebedev/when/rules/common"
	"github.com/olebedev/when/rules/en"
)

type AutoRoles struct {
	parser *when.Parser
}

func (a *AutoRoles) Commands() []string {
	return []string{
		"autorole",
		"autoroles",
	}
}

func (a *AutoRoles) Init(session *discordgo.Session) {
	a.parser = when.New(nil)
	a.parser.Add(en.All...)
	a.parser.Add(common.All...)
}

func (a *AutoRoles) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	args := strings.Fields(content)
	if len(args) >= 1 {
		switch args[0] {
		case "add":
			session.ChannelTyping(msg.ChannelID)
			helpers.RequireAdmin(msg, func() {
				if len(args) < 2 {
					_, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					helpers.Relax(err)
					return
				}
				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)

				var delay time.Duration
				if len(args) >= 5 {
					timeText := strings.TrimSpace(strings.Replace(content, strings.Join(args[:len(args)-3], " "), "", 1))
					timeText = strings.Replace(timeText, "after", "in", 1)
					fmt.Println("timeText:", timeText)
					now := time.Now()
					r, err := a.parser.Parse(timeText, now)
					if err == nil && r != nil {
						delay = r.Time.Sub(now)
					}
				}
				fmt.Println("delay:", delay.String())

				serverRoles, err := session.GuildRoles(channel.GuildID)
				if err != nil {
					if errD := err.(*discordgo.RESTError); errD != nil {
						if errD.Message.Code == 50013 {
							_, err = session.ChannelMessageSend(msg.ChannelID, "Please give me the `Manage Roles` permission to use this feature.")
							helpers.Relax(err)
							return
						} else {
							helpers.Relax(err)
						}
					} else {
						helpers.Relax(err)
					}
				}

				roleNameToMatch := strings.TrimSpace(strings.Replace(content, strings.Join(args[:1], " "), "", 1))
				if delay > 0 {
					roleNameToMatch = strings.TrimSpace(strings.Replace(roleNameToMatch, strings.Join(args[len(args)-3:], " "), "", 1))
				}
				fmt.Println("roleNameToMatch:", roleNameToMatch)

				var targetRole *discordgo.Role
				for _, role := range serverRoles {
					if strings.ToLower(role.Name) == strings.ToLower(roleNameToMatch) || role.ID == roleNameToMatch {
						targetRole = role
					}
				}
				if targetRole == nil || targetRole.ID == "" {
					_, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					helpers.Relax(err)
					return
				}

				settings := helpers.GuildSettingsGetCached(channel.GuildID)

				for _, role := range settings.AutoRoleIDs {
					if role == targetRole.ID {
						_, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.autorole.role-add-error-duplicate"))
						helpers.Relax(err)
						return
					}
				}
				for _, delayedRole := range settings.DelayedAutoRoles {
					if delayedRole.RoleID == targetRole.ID {
						_, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.autorole.role-add-error-duplicate"))
						helpers.Relax(err)
						return
					}
				}

				var successText string
				if delay <= 0 {
					settings.AutoRoleIDs = append(settings.AutoRoleIDs, targetRole.ID)
					successText = helpers.GetTextF("plugins.autorole.role-add-success", targetRole.Name)
				} else {
					settings.DelayedAutoRoles = append(settings.DelayedAutoRoles, models.DelayedAutoRole{
						RoleID: targetRole.ID,
						Delay:  delay,
					})
					successText = helpers.GetTextF("plugins.autorole.delayed-role-add-success", targetRole.Name, delay.String())
				}

				err = helpers.GuildSettingsSet(channel.GuildID, settings)
				helpers.Relax(err)

				_, err = session.ChannelMessageSend(msg.ChannelID, successText)
				helpers.Relax(err)
				return
			})
			return
		case "list":
			session.ChannelTyping(msg.ChannelID)
			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)
			settings := helpers.GuildSettingsGetCached(channel.GuildID)

			if len(settings.AutoRoleIDs) <= 0 {
				_, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.autorole.role-list-none"))
				helpers.Relax(err)
				return
			}

			result := "AutoRoles on this server: "

			for _, roleID := range settings.AutoRoleIDs {
				role, err := session.State.Role(channel.GuildID, roleID)
				if err == nil {
					result += fmt.Sprintf("`%s (#%s)`\n", role.Name, role.ID)
				} else {
					result += fmt.Sprintf("`N/A (#%s)`\n", roleID)
				}
			}

			for _, delayedRole := range settings.DelayedAutoRoles {
				role, err := session.State.Role(channel.GuildID, delayedRole.RoleID)
				if err == nil {
					result += fmt.Sprintf("`%s (#%s)` after %s\n", role.Name, role.ID, delayedRole.Delay.String())
				} else {
					result += fmt.Sprintf("`N/A (#%s)` after %s\n", delayedRole.RoleID, delayedRole.Delay.String())
				}
			}

			result += fmt.Sprintf("(%d role(s))", len(settings.AutoRoleIDs))

			_, err = session.ChannelMessageSend(msg.ChannelID, result)
			helpers.Relax(err)
			return
		case "delete", "remove":
			session.ChannelTyping(msg.ChannelID)
			helpers.RequireAdmin(msg, func() {
				if len(args) < 2 {
					_, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					helpers.Relax(err)
					return
				}
				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)

				serverRoles, err := session.GuildRoles(channel.GuildID)
				if err != nil {
					if errD := err.(*discordgo.RESTError); errD != nil {
						if errD.Message.Code == 50013 {
							_, err = session.ChannelMessageSend(msg.ChannelID, "Please give me the `Manage Roles` permission to use this feature.")
							helpers.Relax(err)
							return
						} else {
							helpers.Relax(err)
						}
					} else {
						helpers.Relax(err)
					}
				}

				roleNameToMatch := strings.TrimSpace(strings.Replace(content, strings.Join(args[:1], " "), "", 1))

				var targetRole *discordgo.Role
				for _, role := range serverRoles {
					if strings.ToLower(role.Name) == strings.ToLower(roleNameToMatch) || role.ID == roleNameToMatch {
						targetRole = role
					}
				}
				if targetRole == nil || targetRole.ID == "" {
					_, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					helpers.Relax(err)
					return
				}

				settings := helpers.GuildSettingsGetCached(channel.GuildID)

				roleWasInList := false
				newRoleIDs := make([]string, 0)
				newDelayedRoles := make([]models.DelayedAutoRole, 0)

				for _, role := range settings.AutoRoleIDs {
					if role == targetRole.ID {
						roleWasInList = true
					} else {
						newRoleIDs = append(newRoleIDs, role)
					}
				}

				if !roleWasInList {
					for _, delayedRole := range settings.DelayedAutoRoles {
						if delayedRole.RoleID == targetRole.ID {
							roleWasInList = true
						} else {
							newDelayedRoles = append(newDelayedRoles, delayedRole)
						}
					}
				} else {
					newDelayedRoles = settings.DelayedAutoRoles
				}

				if !roleWasInList {
					_, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.autorole.role-remove-error-not-found"))
					helpers.Relax(err)
					return
				}

				settings.AutoRoleIDs = newRoleIDs
				settings.DelayedAutoRoles = newDelayedRoles

				err = helpers.GuildSettingsSet(channel.GuildID, settings)
				helpers.Relax(err)

				_, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.autorole.role-remove-success"))
				helpers.Relax(err)
				return
			})
			return
		case "apply":
			session.ChannelTyping(msg.ChannelID)
			helpers.RequireAdmin(msg, func() {
				if len(args) < 2 {
					_, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					helpers.Relax(err)
					return
				}
				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)

				serverRoles, err := session.GuildRoles(channel.GuildID)
				if err != nil {
					if errD := err.(*discordgo.RESTError); errD != nil {
						if errD.Message.Code == 50013 {
							_, err = session.ChannelMessageSend(msg.ChannelID, "Please give me the `Manage Roles` permission to use this feature.")
							helpers.Relax(err)
							return
						} else {
							helpers.Relax(err)
						}
					} else {
						helpers.Relax(err)
					}
				}

				roleNameToMatch := strings.TrimSpace(strings.Replace(content, strings.Join(args[:1], " "), "", 1))

				var targetRole *discordgo.Role
				for _, role := range serverRoles {
					if strings.ToLower(role.Name) == strings.ToLower(roleNameToMatch) || role.ID == roleNameToMatch {
						targetRole = role
					}
				}
				if targetRole == nil || targetRole.ID == "" {
					_, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					helpers.Relax(err)
					return
				}

				users := make([]string, 0)
				for _, botGuild := range session.State.Guilds {
					if botGuild.ID == channel.GuildID {
						for _, member := range botGuild.Members {
							users = append(users, member.User.ID)
						}
					}
				}

				if helpers.ConfirmEmbed(msg.ChannelID, msg.Author, helpers.GetTextF("plugins.autorole.apply-confirm",
					targetRole.Name, targetRole.ID, len(users)), "✅", "🚫") {
					_, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.autorole.apply-started"))
					helpers.Relax(err)

					addedSuccess := 0
					addedError := 0

					for _, userID := range users {
						err := session.GuildMemberRoleAdd(channel.GuildID, userID, targetRole.ID)
						if err != nil {
							addedError += 1
						} else {
							addedSuccess += 1
						}
					}

					_, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.autorole.apply-done",
						msg.Author.ID, addedSuccess, addedError))
					helpers.Relax(err)
					return
				}
				return
			})
			return
		}
	}
}

func (a *AutoRoles) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {

}

func (a *AutoRoles) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {
	go func() {
		defer helpers.Recover()

		settings := helpers.GuildSettingsGetCached(member.GuildID)
		for _, roleID := range settings.AutoRoleIDs {
			err := AutoroleApply(member.GuildID, member.User.ID, roleID)
			helpers.RelaxLog(err)
		}
		for _, delayedAutorole := range settings.DelayedAutoRoles {
			signature := AutoroleApplySignature(member.GuildID, member.User.ID, delayedAutorole.RoleID)
			applyAt := time.Now().Add(delayedAutorole.Delay)
			signature.ETA = &applyAt

			_, err := cache.GetMachineryServer().SendTask(signature)
			helpers.Relax(err)
		}
	}()
}

func AutoroleApply(guildID string, userID string, roleID string) (err error) {
	err = cache.GetSession().GuildMemberRoleAdd(guildID, userID, roleID)
	if err != nil {
		if errD, ok := err.(*discordgo.RESTError); ok {
			if errD.Message.Code != discordgo.ErrCodeMissingPermissions &&
				errD.Message.Code != discordgo.ErrCodeMissingAccess &&
				errD.Message.Code != discordgo.ErrCodeUnknownRole {
				return
			}
		} else {
			return
		}
	}
	err = nil
	return
}
func AutoroleApplySignature(guildID string, userID string, roleID string) (signature *tasks.Signature) {
	signature = &tasks.Signature{
		Name: "apply_autorole",
		Args: []tasks.Arg{
			{
				Type:  "string",
				Value: guildID,
			},
			{
				Type:  "string",
				Value: userID,
			},
			{
				Type:  "string",
				Value: roleID,
			},
		},
	}
	signature.RetryCount = 3
	signature.OnError = []*tasks.Signature{{Name: "log_error"}}
	return signature
}

func (a *AutoRoles) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {

}

func (a *AutoRoles) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {

}

func (a *AutoRoles) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {

}

func (a *AutoRoles) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {

}

func (a *AutoRoles) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}
func (a *AutoRoles) OnMessageDelete(msg *discordgo.MessageDelete, session *discordgo.Session) {

}
