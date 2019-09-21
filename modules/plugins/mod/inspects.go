package mod

import (
	"fmt"
	"sort"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	humanize "github.com/dustin/go-humanize"
)

func (m *Mod) inspectsUserGotBannedOnGuild(bannedUser *discordgo.GuildBanAdd) error {
	// don't inspect robyul
	if bannedUser.User.ID == cache.GetSession().SessionForGuildS(bannedUser.GuildID).State.User.ID {
		return nil
	}

	cache.GetLogger().WithFields(logrus.Fields{
		"UserID":  bannedUser.User.ID,
		"GuildID": bannedUser.GuildID,
	}).Infof("displaying inspects because User got banned on a guild")

	// get banned list
	bannedOnServerList, checkFailedServerList, totalGuilds := m.inspectUserBans(bannedUser.User)

	// build base embed
	resultEmbed := &discordgo.MessageEmbed{
		Title: helpers.GetTextF("plugins.mod.inspect-embed-title", bannedUser.User.Username, bannedUser.User.Discriminator),
		Description: helpers.GetTextF("plugins.mod.inspect-description-done", bannedUser.User.ID) +
			"\n_inspected because User got banned on a different Server._",
		URL:       helpers.GetAvatarUrl(bannedUser.User),
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: helpers.GetAvatarUrl(bannedUser.User)},
		Footer:    &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.mod.inspect-embed-footer", bannedUser.User.ID, totalGuilds)},
		Color:     0x0FADED,
	}

	resultBansText := ""
	if len(bannedOnServerList) <= 0 {
		resultBansText += fmt.Sprintf(":white_check_mark: User is banned on none servers.\n:black_medium_small_square:Checked %d servers.", totalGuilds-len(checkFailedServerList))
	} else {
		resultBansText += fmt.Sprintf(":warning: User is banned on **%d** server(s).\n:black_medium_small_square:Checked %d servers.", len(bannedOnServerList), totalGuilds-len(checkFailedServerList))
	}

	isOnServerList := m.inspectCommonServers(bannedUser.User)
	commonGuildsText := ""
	if len(isOnServerList) > 0 { // -1 to exclude the server the user is currently on
		commonGuildsText += fmt.Sprintf(":white_check_mark: User is on **%d** server(s) with Robyul.", len(isOnServerList)-1)
	} else {
		commonGuildsText += ":question: User is on **none** servers with Robyul."
	}

	joinedTime := helpers.GetTimeFromSnowflake(bannedUser.User.ID)
	oneDayAgo := time.Now().AddDate(0, 0, -1)
	oneWeekAgo := time.Now().AddDate(0, 0, -7)
	joinedTimeText := ""
	if !joinedTime.After(oneWeekAgo) {
		joinedTimeText += fmt.Sprintf(":white_check_mark: User Account got created %s.\n:black_medium_small_square:Joined at %s.", helpers.SinceInDaysText(joinedTime), joinedTime.Format(time.ANSIC))
	} else if !joinedTime.After(oneDayAgo) {
		joinedTimeText += fmt.Sprintf(":question: User Account is less than one Week old.\n:black_medium_small_square:Joined at %s.", joinedTime.Format(time.ANSIC))
	} else {
		joinedTimeText += fmt.Sprintf(":warning: User Account is less than one Day old.\n:black_medium_small_square:Joined at %s.", joinedTime.Format(time.ANSIC))
	}

	// send embeds for guilds
	for _, shard := range cache.GetSession().Sessions {
		for _, targetGuild := range shard.State.Guilds {
			// skip on guild where user was banned
			if targetGuild.ID == bannedUser.GuildID {
				continue
			}

			// skip if user banned trigger is not enabled on guild
			if !helpers.GuildSettingsGetCached(targetGuild.ID).InspectTriggersEnabled.UserBannedOnOtherServers {
				continue
			}

			// skip if user is not on guild
			if !helpers.GetIsInGuild(targetGuild.ID, bannedUser.User.ID) {
				continue
			}

			joins, _ := m.GetJoins(bannedUser.User.ID, targetGuild.ID)
			joinsText := ""
			if len(joins) == 0 {
				joinsText = ":white_check_mark: User never joined this server\n"
			} else if len(joins) == 1 {
				if joins[0].InviteCodeUsed != "" {
					createdByUser, _ := helpers.GetUser(joins[0].InviteCodeCreatedByUserID)
					if createdByUser == nil {
						createdByUser = new(discordgo.User)
						createdByUser.ID = joins[0].InviteCodeCreatedByUserID
						createdByUser.Username = "N/A"
					}

					var labelText string
					if joins[0].VanityInviteUsedName != "" {
						labelText = " (`" + helpers.GetConfig().Path("website.vanityurl_domain").Data().(string) + "/" + joins[0].VanityInviteUsedName + "`)"
					}

					joinsText = fmt.Sprintf(":white_check_mark: User joined this server once (%s) with the invite `%s`%s created by `%s (#%s)` %s\n",
						humanize.Time(joins[0].JoinedAt), joins[0].InviteCodeUsed, labelText, createdByUser.Username,
						createdByUser.ID, humanize.Time(joins[0].InviteCodeCreatedAt))
				} else {
					joinsText = fmt.Sprintf(":white_check_mark: User joined this server once (%s)\nGive Robyul the `Manage Server` permission to see using which invite.\n",
						humanize.Time(joins[0].JoinedAt))
				}
			} else if len(joins) > 1 {
				sort.Slice(joins, func(i, j int) bool { return joins[i].JoinedAt.After(joins[j].JoinedAt) })
				lastJoin := joins[0]

				if lastJoin.InviteCodeUsed != "" {
					createdByUser, _ := helpers.GetUser(lastJoin.InviteCodeCreatedByUserID)
					if createdByUser == nil {
						createdByUser = new(discordgo.User)
						createdByUser.ID = lastJoin.InviteCodeCreatedByUserID
						createdByUser.Username = "N/A"
					}

					var labelText string
					if lastJoin.VanityInviteUsedName != "" {
						labelText = " (`" + helpers.GetConfig().Path("website.vanityurl_domain").Data().(string) + "/" + joins[0].VanityInviteUsedName + "`)"
					}

					joinsText = fmt.Sprintf(":warning: User joined this server %d times (last time %s)\n"+
						"Last time with the invite `%s`%s created by `%s (#%s)` %s\n",
						len(joins), humanize.Time(lastJoin.JoinedAt), lastJoin.InviteCodeUsed,
						labelText, createdByUser.Username, createdByUser.ID, humanize.Time(lastJoin.InviteCodeCreatedAt))
				} else {
					joinsText = fmt.Sprintf(":warning: User joined this server %d times (last time %s)\n"+
						"Give Robyul the `Manage Server` permission to see using which invites.\n",
						len(joins), humanize.Time(lastJoin.JoinedAt))
				}
			}

			resultEmbed.Fields = []*discordgo.MessageEmbedField{
				{Name: "Bans", Value: resultBansText, Inline: false},
				{Name: "Join History", Value: joinsText, Inline: false},
				{Name: "Common Servers", Value: commonGuildsText, Inline: false},
				{Name: "Account Age", Value: joinedTimeText, Inline: false},
			}

			for _, failedServer := range checkFailedServerList {
				if failedServer.ID != targetGuild.ID {
					continue
				}

				resultEmbed.Description += "\n:warning: I wasn't able to gather the ban list for this server!\nPlease give Robyul the permission `Ban Members` to help other servers."
				break
			}

			targetChannel, err := helpers.GetChannelWithoutApi(helpers.GuildSettingsGetCached(targetGuild.ID).InspectsChannel)
			if err == nil {
				// confirm user is still in Guild before sending
				if !helpers.GetIsInGuild(targetGuild.ID, bannedUser.User.ID) {
					continue
				}

				_, err = helpers.SendEmbed(targetChannel.ID, resultEmbed)
				if err != nil {
					cache.GetLogger().WithField("module", "mod").Warnf("Failed to send guild ban inspect to channel #%s on guild #%s: %s",
						helpers.GuildSettingsGetCached(targetGuild.ID).InspectsChannel, targetGuild.ID, err.Error())
					if errD, ok := err.(*discordgo.RESTError); ok {
						if errD.Message.Code != discordgo.ErrCodeMissingAccess {
							helpers.RelaxLog(err)
						}
					} else {
						helpers.RelaxLog(err)
					}
					continue
				}
			}
		}
	}

	return nil
}
