package plugins

import (
	"fmt"
	"strings"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	rethink "github.com/gorethink/gorethink"
)

type Troublemaker struct{}

func (t *Troublemaker) Commands() []string {
	return []string{
		"troublemaker",
		"troublemakers",
		"tm",
	}
}

type DB_Troublemaker_Entry struct {
	ID                string    `gorethink:"id,omitempty"`
	UserID            string    `gorethink:"userid"`
	Reason            string    `gorethink:"reason"`
	CreatedAt         time.Time `gorethink:"createdat"`
	ReportedByGuildID string    `gorethink:"reportedby_guildid"`
	ReportedByUserID  string    `gorethink:"reportedby_userid"`
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

					settings.TroublemakerIsParticipating = true
					settings.TroublemakerLogChannel = targetChannel.ID
					err = helpers.GuildSettingsSet(channel.GuildID, settings)
					helpers.Relax(err)

					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.troublemaker.participation-enabled"))
					helpers.Relax(err)

					return
				} else {
					// Disable logging
					settings.TroublemakerIsParticipating = false
					err = helpers.GuildSettingsSet(channel.GuildID, settings)
					helpers.Relax(err)

					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.troublemaker.participation-disabled"))
					helpers.Relax(err)

					return
				}
				return
			})
			break
		case "list":
			helpers.RequireMod(msg, func() {
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

				troublemakerReports := t.getTroublemakerReports(targetUser)

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
			})
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
					troublemakerLogEntry := t.getEntryByOrCreateEmpty("id", "")
					troublemakerLogEntry.UserID = targetUser.ID
					troublemakerLogEntry.Reason = reasonText
					troublemakerLogEntry.CreatedAt = time.Now()
					troublemakerLogEntry.ReportedByGuildID = guild.ID
					troublemakerLogEntry.ReportedByUserID = msg.Author.ID
					t.setEntry(troublemakerLogEntry)

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

					successMessages, _ := helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.troublemaker.report-successful", len(guildsToNotify)))

					// Send notifications out
					go func() {
						defer helpers.Recover()

						for _, guildToNotify := range guildsToNotify {
							guildToNotifySettings := helpers.GuildSettingsGetCached(guildToNotify.ID)
							if guildToNotifySettings.TroublemakerIsParticipating == true && guildToNotifySettings.TroublemakerLogChannel != "" {
								targetUserIsOnServer := false
								_, err := helpers.GetGuildMember(guildToNotify.ID, targetUser.ID)
								if err == nil {
									targetUserIsOnServer = true
								}

								reportEmbed := &discordgo.MessageEmbed{
									Title:       helpers.GetTextF("plugins.troublemaker.report-embed-title", targetUser.Username, targetUser.Discriminator),
									Description: helpers.GetTextF("plugins.troublemaker.report-embed-description", targetUser.ID, targetUser.ID),
									URL:         helpers.GetAvatarUrl(targetUser),
									Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: helpers.GetAvatarUrl(targetUser)},
									Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.troublemaker.report-embed-footer", len(guildsToNotify))},
									Color:       0x0FADED,
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

func (t *Troublemaker) getTroublemakerReports(user *discordgo.User) []DB_Troublemaker_Entry {
	var entryBucket []DB_Troublemaker_Entry
	listCursor, err := rethink.Table("troublemakerlog").Filter(
		rethink.Row.Field("userid").Eq(user.ID),
	).Run(helpers.GetDB())
	helpers.Relax(err)
	defer listCursor.Close()
	listCursor.All(&entryBucket)

	return entryBucket
}

func (t *Troublemaker) getEntryByOrCreateEmpty(key string, id string) DB_Troublemaker_Entry {
	var entryBucket DB_Troublemaker_Entry
	listCursor, err := rethink.Table("troublemakerlog").Filter(
		rethink.Row.Field(key).Eq(id),
	).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer listCursor.Close()
	err = listCursor.One(&entryBucket)

	// If user has no DB entries create an empty document
	if err == rethink.ErrEmptyResult {
		insert := rethink.Table("troublemakerlog").Insert(DB_Troublemaker_Entry{})
		res, e := insert.RunWrite(helpers.GetDB())
		// If the creation was successful read the document
		if e != nil {
			panic(e)
		} else {
			return t.getEntryByOrCreateEmpty("id", res.GeneratedKeys[0])
		}
	} else if err != nil {
		panic(err)
	}

	return entryBucket
}

func (t *Troublemaker) setEntry(entry DB_Troublemaker_Entry) {
	_, err := rethink.Table("troublemakerlog").Update(entry).Run(helpers.GetDB())
	helpers.Relax(err)
}
