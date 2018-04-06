package plugins

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
)

type Nuke struct{}

func (n *Nuke) Commands() []string {
	return []string{
		"nuke",
	}
}

func (n *Nuke) Init(session *discordgo.Session) {
	splitChooseRegex = regexp.MustCompile(`'.*?'|".*?"|\S+`)
}

func (n *Nuke) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermNuke) {
		return
	}

	args := strings.Fields(content)
	if len(args) >= 1 {
		switch args[0] {
		case "user": // [p]nuke user <user id/mention> "<reason>"
			if !helpers.IsNukeMod(msg.Author.ID) {
				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.nuke.no-nukemod-permissions"))
				helpers.Relax(err)
				return
			} else {
				session.ChannelTyping(msg.ChannelID)

				safeArgs := splitChooseRegex.FindAllString(content, -1)
				if len(safeArgs) < 3 {
					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
					return
				} else {
					var err error
					var targetUser *discordgo.User
					targetUser, err = helpers.GetUserFromMention(strings.Trim(safeArgs[1], "\""))
					if err != nil {
						if err, ok := err.(*discordgo.RESTError); ok && err.Message.Code == discordgo.ErrCodeUnknownUser {
							_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.nuke.user-not-found"))
							helpers.Relax(err)
							return
						} else {
							helpers.Relax(err)
							return
						}
					}

					reason := strings.TrimSpace(strings.Replace(content, strings.Join(args[:2], " "), "", 1))

					if helpers.ConfirmEmbed(msg.ChannelID, msg.Author, helpers.GetTextF("plugins.nuke.nuke-confirm",
						targetUser.Username, targetUser.ID, targetUser.ID, reason), "✅", "🚫") == true {
						_, err = helpers.MDbInsert(
							models.NukelogTable,
							models.NukelogEntry{
								UserID:   targetUser.ID,
								UserName: targetUser.Username + "#" + targetUser.Discriminator,
								NukerID:  msg.Author.ID,
								Reason:   strings.TrimSpace(reason),
								NukedAt:  time.Now(),
							},
						)
						helpers.Relax(err)

						_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.nuke.nuke-saved-in-db"))
						helpers.Relax(err)

						bannedOnN := 0

						reasonText := fmt.Sprintf("Nuke Ban | Issued by: %s#%s (#%s) | Delete Days: %d | Reason: %s",
							msg.Author.Username, msg.Author.Discriminator, msg.Author.ID, 1, strings.TrimSpace(reason))

						for _, targetGuild := range session.State.Guilds {
							targetGuildSettings := helpers.GuildSettingsGetCached(targetGuild.ID)
							fmt.Println("checking server: ", targetGuild.Name)
							if targetGuildSettings.NukeIsParticipating == true {
								err = session.GuildBanCreateWithReason(targetGuild.ID, targetUser.ID, reasonText, 1)
								if err != nil {
									if err, ok := err.(*discordgo.RESTError); ok {
										helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.nuke.ban-error",
											targetGuild.Name, targetGuild.ID, err.Message.Message))
										if targetGuildSettings.NukeLogChannel != "" {
											helpers.SendMessage(targetGuildSettings.NukeLogChannel,
												helpers.GetTextF("plugins.nuke.onserver-banned-error",
													targetUser.Username, targetUser.ID,
													err.Message.Message,
													msg.Author.Username, msg.Author.ID,
													reason))
										}
									} else {
										helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.nuke.ban-error",
											targetGuild.Name, targetGuild.ID, err.Error()))
										if targetGuildSettings.NukeLogChannel != "" {
											helpers.SendMessage(targetGuildSettings.NukeLogChannel,
												helpers.GetTextF("plugins.nuke.onserver-banned-error",
													targetUser.Username, targetUser.ID,
													err.Error(),
													msg.Author.Username, msg.Author.ID,
													reason))
										}
									}
								} else {
									helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.nuke.banned-on-server",
										targetGuild.Name, targetGuild.ID))
									if targetGuildSettings.NukeLogChannel != "" {
										helpers.SendMessage(targetGuildSettings.NukeLogChannel,
											helpers.GetTextF("plugins.nuke.onserver-banned-success",
												targetUser.Username, targetUser.ID,
												msg.Author.Username, msg.Author.ID,
												reason))
									}
									bannedOnN += 1
								}
							}
						}

						_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.nuke.nuke-completed", bannedOnN))
						helpers.Relax(err)
					}
				}
			}
			return
		case "participate": // [p]nuke participate [<log channel>]
			helpers.RequireAdmin(msg, func() {
				session.ChannelTyping(msg.ChannelID)

				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)

				settings := helpers.GuildSettingsGetCached(channel.GuildID)

				if len(args) >= 2 {
					targetChannel, err := helpers.GetChannelFromMention(msg, args[1])
					if err != nil {
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
						return
					}

					nukeModMentions := make([]string, 0)
					for _, nukeMod := range helpers.NukeMods {
						nukeModMentions = append(nukeModMentions, "<@"+nukeMod+">")
					}

					if helpers.ConfirmEmbed(msg.ChannelID, msg.Author, helpers.GetTextF("plugins.nuke.participation-confirm", strings.Join(nukeModMentions, ", "), helpers.GetPrefixForServer(channel.GuildID)), "✅", "🚫") == true {
						settings.NukeIsParticipating = true
						previousChannel := settings.NukeLogChannel
						settings.NukeLogChannel = targetChannel.ID
						err = helpers.GuildSettingsSet(channel.GuildID, settings)
						helpers.Relax(err)

						changes := make([]models.ElasticEventlogChange, 0)
						if previousChannel != "" {
							changes = []models.ElasticEventlogChange{
								{
									Key:      "nuke_participate_channel",
									OldValue: previousChannel,
									NewValue: settings.NukeLogChannel,
									Type:     models.EventlogTargetTypeChannel,
								},
							}
						}

						changes = append(changes, models.ElasticEventlogChange{
							Key:      "nuke_participating",
							OldValue: helpers.StoreBoolAsString(false),
							NewValue: helpers.StoreBoolAsString(true),
						})

						_, err = helpers.EventlogLog(time.Now(), channel.GuildID, channel.GuildID,
							models.EventlogTargetTypeGuild, msg.Author.ID,
							models.EventlogTypeRobyulNukeParticipate, "",
							changes,
							[]models.ElasticEventlogOption{
								{
									Key:   "nuke_participate_channel",
									Value: targetChannel.ID,
									Type:  models.EventlogTargetTypeChannel,
								},
							}, false)
						helpers.RelaxLog(err)

						_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.nuke.participation-enabled"))
						helpers.Relax(err)
						// TODO: ask to ban people already nuked?
					}
				} else {
					settings.NukeIsParticipating = false
					err = helpers.GuildSettingsSet(channel.GuildID, settings)
					helpers.Relax(err)

					_, err = helpers.EventlogLog(time.Now(), channel.GuildID, channel.GuildID,
						models.EventlogTargetTypeGuild, msg.Author.ID,
						models.EventlogTypeRobyulNukeParticipate, "",
						[]models.ElasticEventlogChange{
							{
								Key:      "nuke_participating",
								OldValue: helpers.StoreBoolAsString(true),
								NewValue: helpers.StoreBoolAsString(false),
							},
						},
						nil, false)
					helpers.RelaxLog(err)

					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.nuke.participation-disabled"))
					helpers.Relax(err)
				}
			})
			return
		case "log": // [p]nuke log
			helpers.RequireMod(msg, func() {
				session.ChannelTyping(msg.ChannelID)

				var entryBucket []models.NukelogEntry
				err := helpers.MDbIter(helpers.MdbCollection(models.NukelogTable).Find(nil).Sort("nukedat")).All(&entryBucket)
				helpers.Relax(err)

				logMessage := "__**Nuke Log:**__\n"
				for _, logEntry := range entryBucket {
					logMessage += fmt.Sprintf("`%s` `#%s` Reason `%s` at `%s UTC`\n",
						logEntry.UserName, logEntry.UserID, logEntry.Reason, logEntry.NukedAt.Format(time.ANSIC))
				}
				logMessage += "_All Usernames are from the time they got nuked._"

				_, err = helpers.SendMessage(msg.ChannelID, logMessage)
				helpers.Relax(err)
			})
			return
		case "apply":
			helpers.RequireAdmin(msg, func() {
				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)

				// bot is allowed to ban?
				botPermissions := helpers.GetMemberPermissions(channel.GuildID, session.State.User.ID)
				if botPermissions&discordgo.PermissionBanMembers != discordgo.PermissionBanMembers &&
					botPermissions&discordgo.PermissionAdministrator != discordgo.PermissionAdministrator {
					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.nuke.apply-bot-not-allowed"))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}
				// user is allowed to ban?
				userPermissions := helpers.GetMemberPermissions(channel.GuildID, msg.Author.ID)
				if userPermissions&discordgo.PermissionBanMembers != discordgo.PermissionBanMembers &&
					userPermissions&discordgo.PermissionAdministrator != discordgo.PermissionAdministrator {
					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.nuke.apply-user-not-allowed"))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}

				if helpers.ConfirmEmbed(msg.ChannelID, msg.Author, helpers.GetTextF("plugins.nuke.apply-confirm",
					helpers.GetPrefixForServer(channel.GuildID)), "✅", "🚫") {
					// gather bans
					guildBans, err := cache.GetSession().GuildBans(channel.GuildID)
					helpers.Relax(err)

					// gather nuked users
					var entryBucket []models.NukelogEntry
					err = helpers.MDbIter(helpers.MdbCollection(models.NukelogTable).Find(nil).Sort("nukedat")).All(&entryBucket)
					helpers.Relax(err)

					// ban users
					var nuked int
				NextNukedUser:
					for _, nukeEntry := range entryBucket {
						nukedByUser, err := helpers.GetUser(nukeEntry.NukerID)
						if err != nil {
							nukedByUser = new(discordgo.User)
							nukedByUser.Username = "N/A"
							nukedByUser.Discriminator = "N/A"
						}
						nukedUser, err := helpers.GetUser(nukeEntry.UserID)
						if err != nil {
							nukedUser = new(discordgo.User)
							nukedUser.Username = "N/A"
							nukedUser.Discriminator = "N/A"
						}

						// already banned?
						for _, guildBan := range guildBans {
							if guildBan.User.ID == nukeEntry.UserID {
								helpers.SendMessage(msg.ChannelID,
									fmt.Sprintf(":white_check_mark: User `%s#%s` (`#%s`) is already banned",
										nukedUser.Username, nukedUser.Discriminator, nukeEntry.UserID))
								continue NextNukedUser
							}
						}

						reasonText := fmt.Sprintf("Nuke Apply Ban | Issued by: %s#%s (#%s) | Delete Days: %d | Reason: %s",
							nukedByUser.Username, nukedByUser.Discriminator, nukeEntry.UserID, 0, nukeEntry.Reason)

						err = cache.GetSession().GuildBanCreateWithReason(channel.GuildID, nukeEntry.UserID, reasonText, 0)
						helpers.Relax(err)

						helpers.SendMessage(msg.ChannelID,
							fmt.Sprintf(":white_check_mark: Banned User `%s#%s` (`#%s`): `%s`",
								nukedUser.Username, nukedUser.Discriminator, nukeEntry.UserID, nukeEntry.Reason))
						nuked++
					}

					_, err = helpers.SendMessage(msg.ChannelID, fmt.Sprintf("banned **%d** nuked users", nuked))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}
				return
			})
			return
		}
	}
}
