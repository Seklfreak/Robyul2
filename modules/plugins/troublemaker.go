package plugins

import (
	"fmt"
	"strings"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/globalsign/mgo/bson"
)

type Troublemaker struct{}

func (t *Troublemaker) Commands() []string {
	return []string{
		"troublemaker",
		"troublemakers",
		"tm",
	}
}

func (t *Troublemaker) Init(session *discordgo.Session) {

}

func (t *Troublemaker) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermTroublemaker) {
		return
	}

	args := strings.Fields(content)
	if len(args) >= 1 {
		switch args[0] {
		case "participate":
			helpers.RequireAdmin(msg, func() {
				session.ChannelTyping(msg.ChannelID)

				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)

				settings := helpers.GuildSettingsGetCached(channel.GuildID)

				if len(args) >= 2 {
					// Set new log channel
					targetChannel, err := helpers.GetChannelFromMention(msg, args[1])
					if err != nil || targetChannel.ID == "" {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
						return
					}

					previousChannel := settings.TroublemakerLogChannel

					settings.TroublemakerIsParticipating = true
					settings.TroublemakerLogChannel = targetChannel.ID
					err = helpers.GuildSettingsSet(channel.GuildID, settings)
					helpers.Relax(err)

					changes := make([]models.ElasticEventlogChange, 0)
					if previousChannel != "" {
						changes = []models.ElasticEventlogChange{
							{
								Key:      "troublemaker_participate_channel",
								OldValue: previousChannel,
								NewValue: settings.TroublemakerLogChannel,
								Type:     models.EventlogTargetTypeChannel,
							},
						}
					}

					changes = append(changes, models.ElasticEventlogChange{
						Key:      "troublemaker_participating",
						OldValue: helpers.StoreBoolAsString(false),
						NewValue: helpers.StoreBoolAsString(true),
					})

					_, err = helpers.EventlogLog(time.Now(), channel.GuildID, channel.GuildID,
						models.EventlogTargetTypeGuild, msg.Author.ID,
						models.EventlogTypeRobyulTroublemakerParticipate, "",
						changes,
						[]models.ElasticEventlogOption{
							{
								Key:   "troublemaker_participate_channel",
								Value: targetChannel.ID,
								Type:  models.EventlogTargetTypeChannel,
							},
						}, false)
					helpers.RelaxLog(err)

					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.troublemaker.participation-enabled"))
					helpers.Relax(err)

					return
				} else {
					// Disable logging
					settings.TroublemakerIsParticipating = false
					err = helpers.GuildSettingsSet(channel.GuildID, settings)
					helpers.Relax(err)

					_, err = helpers.EventlogLog(time.Now(), channel.GuildID, channel.GuildID,
						models.EventlogTargetTypeGuild, msg.Author.ID,
						models.EventlogTypeRobyulTroublemakerParticipate, "",
						[]models.ElasticEventlogChange{
							{
								Key:      "troublemaker_participating",
								OldValue: helpers.StoreBoolAsString(true),
								NewValue: helpers.StoreBoolAsString(false),
							},
						},
						nil, false)
					helpers.RelaxLog(err)

					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.troublemaker.participation-disabled"))
					helpers.Relax(err)

					return
				}
				return
			})
			break
		case "list":
			if !helpers.IsMod(msg) && !helpers.CanInspectExtended(msg) && !helpers.CanInspectBasic(msg) {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("mod.no_permission"))
				return
			}

			session.ChannelTyping(msg.ChannelID)

			if len(args) < 2 {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				return
			}

			targetUser, err := helpers.GetUserFromMention(args[1])
			if err != nil || targetUser.ID == "" {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				return
			}

			var troublemakerReports []models.TroublemakerlogEntry
			err = helpers.MDbIter(helpers.MdbCollection(models.TroublemakerlogTable).Find(bson.M{"userid": targetUser.ID})).All(&troublemakerReports)
			helpers.Relax(err)

			if len(troublemakerReports) <= 0 {
				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.troublemaker.list-no-reports", targetUser.Username))
				helpers.Relax(err)
				return
			} else {
				troublemakerText := fmt.Sprintf("I found %d report(s) for `%s#%s` `#%s` <@%s>:\n",
					len(troublemakerReports), targetUser.Username, targetUser.Discriminator, targetUser.ID, targetUser.ID,
				)

				for _, troublemakerReport := range troublemakerReports {
					reportedByUser, err := helpers.GetUser(troublemakerReport.ReportedByUserID)
					if err != nil {
						reportedByUser = new(discordgo.User)
						reportedByUser.ID = troublemakerReport.ReportedByUserID
						reportedByUser.Username = "N/A"
					}
					reportedByGuild, err := helpers.GetGuild(troublemakerReport.ReportedByGuildID)
					if err != nil {
						reportedByGuild = new(discordgo.Guild)
						reportedByGuild.ID = troublemakerReport.ReportedByGuildID
						reportedByGuild.Name = "N/A"
					}
					troublemakerText += fmt.Sprintf("At: `%s`, Reason: `%s`, Reported By: `%s` (`#%s`) On: `%s` (`#%s`)\n",
						troublemakerReport.CreatedAt.Format(time.ANSIC), troublemakerReport.Reason,
						reportedByUser.Username, reportedByUser.ID, reportedByGuild.Name, reportedByGuild.ID,
					)
				}

				for _, page := range helpers.Pagify(troublemakerText, "\n") {
					_, err := helpers.SendMessage(msg.ChannelID, page)
					helpers.Relax(err)
				}
			}
		default:
			helpers.RequireMod(msg, func() {
				session.ChannelTyping(msg.ChannelID)

				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)
				guild, err := helpers.GetGuild(channel.GuildID)
				helpers.Relax(err)

				if len(args) < 2 {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					return
				}

				targetUser, err := helpers.GetUserFromMention(args[0])
				if err != nil || targetUser.ID == "" {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					return
				}

				reasonText := strings.TrimSpace(strings.Replace(content, strings.Join(args[:1], " "), "", 1))

				if helpers.ConfirmEmbed(msg.ChannelID, msg.Author, helpers.GetTextF("plugins.troublemaker.report-confirm",
					targetUser.Username, targetUser.Discriminator, targetUser.ID, targetUser.ID, reasonText,
				), "✅", "🚫") == true {
					// Save to log DB

					_, err = helpers.MDbInsert(models.TroublemakerlogTable, models.TroublemakerlogEntry{
						UserID:            targetUser.ID,
						Reason:            reasonText,
						CreatedAt:         time.Now(),
						ReportedByGuildID: guild.ID,
						ReportedByUserID:  msg.Author.ID,
					})
					helpers.Relax(err)

					cache.GetLogger().WithField("module", "troublemaker").Info(fmt.Sprintf("will notify about troublemaker %s (#%s) by %s (#%s) on %s (#%s) reason %s",
						targetUser.Username, targetUser.ID,
						msg.Author.Username, msg.Author.ID,
						guild.Name, guild.ID,
						reasonText,
					))

					guildsToNotify := make([]*discordgo.Guild, 0)

					for _, guildToNotify := range session.State.Guilds {
						if guildToNotify.ID != guild.ID {
							guildToNotifySettings := helpers.GuildSettingsGetCached(guildToNotify.ID)
							if guildToNotifySettings.TroublemakerIsParticipating == true && guildToNotifySettings.TroublemakerLogChannel != "" {
								guildsToNotify = append(guildsToNotify, guildToNotify)
							}
						}
					}
					_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetUser.ID,
						models.EventlogTargetTypeUser, msg.Author.ID,
						models.EventlogTypeRobyulTroublemakerReport, reasonText,
						nil,
						nil, false)
					helpers.RelaxLog(err)

					successMessages, _ := helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.troublemaker.report-successful", len(guildsToNotify)))

					// Send notifications out
					go func() {
						defer helpers.Recover()

						for _, guildToNotify := range guildsToNotify {
							guildToNotifySettings := helpers.GuildSettingsGetCached(guildToNotify.ID)
							if guildToNotifySettings.TroublemakerIsParticipating == true && guildToNotifySettings.TroublemakerLogChannel != "" {
								targetUserIsOnServer := helpers.GetIsInGuild(guildToNotify.ID, targetUser.ID)

								reportEmbed := &discordgo.MessageEmbed{
									Title:       helpers.GetTextF("plugins.troublemaker.report-embed-title", targetUser.Username, targetUser.Discriminator),
									Description: helpers.GetTextF("plugins.troublemaker.report-embed-description", targetUser.ID, targetUser.ID),
									URL:         helpers.GetAvatarUrl(targetUser),
									Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: helpers.GetAvatarUrl(targetUser)},
									Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.troublemaker.report-embed-footer",
										len(guildsToNotify), strings.Replace(helpers.GetStaffUsernamesText(), "`", "", -1))},
									Color: 0x0FADED,
									Fields: []*discordgo.MessageEmbedField{
										{Name: "Reason stated", Value: reasonText, Inline: false},
									},
								}

								if targetUserIsOnServer == true {
									reportEmbed.Fields = append(reportEmbed.Fields, &discordgo.MessageEmbedField{
										Name: "Member status", Value: ":warning: User is on this server", Inline: false,
									})
								} else {
									reportEmbed.Fields = append(reportEmbed.Fields, &discordgo.MessageEmbedField{
										Name: "Member status", Value: ":white_check_mark: User is not on this server", Inline: false,
									})
								}

								reportEmbed.Fields = append(reportEmbed.Fields, &discordgo.MessageEmbedField{
									Name: "Reported by", Value: fmt.Sprintf("**%s** on **%s**",
										msg.Author.Username, guild.Name,
									), Inline: false})

								_, err = helpers.SendEmbed(guildToNotifySettings.TroublemakerLogChannel, reportEmbed)
								if err != nil {
									cache.GetLogger().WithField("module", "troublemaker").Warnf("Failed to send troublemaker report to channel #%s on guild #%s: %s"+
										guildToNotifySettings.TroublemakerLogChannel, guildToNotifySettings.Guild, err.Error())
								}
							}
						}

						if len(successMessages) > 0 {
							session.MessageReactionAdd(msg.ChannelID, successMessages[0].ID, "👌")
						}
						return
					}()
					return
				}
				return
			})
			break
		}
	}
}
