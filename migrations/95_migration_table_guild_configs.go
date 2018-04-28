package migrations

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m95_migration_table_guild_configs() {
	if !TableExists("guild_configs") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving guild_configs to mongodb")

	cursor, err := gorethink.Table("guild_configs").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("guild_configs").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		Id string `rethink:"id,omitempty"`
		// Guild contains the guild ID
		Guild string `rethink:"guild"`

		Prefix string `rethink:"prefix"`

		CleanupEnabled bool `rethink:"cleanup_enabled"`

		AnnouncementsEnabled bool `rethink:"announcements_enabled"`
		// AnnouncementsChannel stores the channel ID
		AnnouncementsChannel string `rethink:"announcements_channel"`

		WelcomeNewUsersEnabled bool   `rethink:"welcome_new_users_enabled"`
		WelcomeNewUsersText    string `rethink:"welcome_new_users_text"`

		MutedRoleName string `rethink:"muted_role_name"`

		InspectTriggersEnabled models.InspectTriggersEnabled `rethink:"inspect_triggers_enabled"`
		InspectsChannel        string                        `rethink:"inspects_channel"`

		NukeIsParticipating bool   `rethink:"nuke_participation"`
		NukeLogChannel      string `rethink:"nuke_channel"`

		LevelsIgnoredUserIDs          []string `rethink:"levels_ignored_user_ids"`
		LevelsIgnoredChannelIDs       []string `rethink:"levels_ignored_channel_ids"`
		LevelsNotificationCode        string   `rethink:"level_notification_code"`
		LevelsNotificationDeleteAfter int      `rethink:"level_notification_deleteafter"`
		LevelsMaxBadges               int      `rethink:"levels_maxbadges"`

		MutedMembers []string `rethink:"muted_member_ids"` // deprecated

		TroublemakerIsParticipating bool   `rethink:"troublemaker_participation"`
		TroublemakerLogChannel      string `rethink:"troublemaker_channel"`

		AutoRoleIDs      []string                 `rethink:"autorole_roleids"`
		DelayedAutoRoles []models.DelayedAutoRole `rethink:"delayed_autoroles"`

		StarboardChannelID string   `rethink:"starboard_channel_id"`
		StarboardMinimum   int      `rethink:"starboard_minimum"`
		StarboardEmoji     []string `rethink:"starboard_emoji"`

		ChatlogDisabled bool `rethink:"chatlog_disabled"`

		EventlogDisabled   bool     `rethink:"eventlog_disabled"`
		EventlogChannelIDs []string `rethink:"eventlog_channelids"`

		PersistencyBiasEnabled bool     `rethink:"persistency_bias_enabled"`
		PersistencyRoleIDs     []string `rethink:"persistency_roleids"`

		RandomPicturesPicDelay                  int      `rethink:"randompictures_pic_delay"`
		RandomPicturesPicDelayIgnoredChannelIDs []string `rethink:"randompictures_pic_delay_ignored_channelids"`

		PerspectiveIsParticipating bool   `rethink:"perspective_participation"`
		PerspectiveChannelID       string `rethink:"perspective_channelid"`

		CustomCommandsEveryoneCanAdd bool   `rethink:"customcommands_everyonecanadd"`
		CustomCommandsAddRoleID      string `rethink:"customcommands_add_roleid"`

		AdminRoleIDs []string `rethink:"admin_roleids"`
		ModRoleIDs   []string `rethink:"mod_roleids"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		_, err = helpers.MDbInsertWithoutLogging(
			models.GuildConfigTable,
			models.Config{
				GuildID:                                 rethinkdbEntry.Guild,
				Prefix:                                  rethinkdbEntry.Prefix,
				CleanupEnabled:                          rethinkdbEntry.CleanupEnabled,
				AnnouncementsEnabled:                    rethinkdbEntry.AnnouncementsEnabled,
				AnnouncementsChannel:                    rethinkdbEntry.AnnouncementsChannel,
				WelcomeNewUsersEnabled:                  rethinkdbEntry.WelcomeNewUsersEnabled,
				WelcomeNewUsersText:                     rethinkdbEntry.WelcomeNewUsersText,
				MutedRoleName:                           rethinkdbEntry.MutedRoleName,
				InspectTriggersEnabled:                  rethinkdbEntry.InspectTriggersEnabled,
				InspectsChannel:                         rethinkdbEntry.InspectsChannel,
				NukeIsParticipating:                     rethinkdbEntry.NukeIsParticipating,
				NukeLogChannel:                          rethinkdbEntry.NukeLogChannel,
				LevelsIgnoredUserIDs:                    rethinkdbEntry.LevelsIgnoredUserIDs,
				LevelsIgnoredChannelIDs:                 rethinkdbEntry.LevelsIgnoredChannelIDs,
				LevelsNotificationCode:                  rethinkdbEntry.LevelsNotificationCode,
				LevelsNotificationDeleteAfter:           rethinkdbEntry.LevelsNotificationDeleteAfter,
				LevelsMaxBadges:                         rethinkdbEntry.LevelsMaxBadges,
				MutedMembers:                            rethinkdbEntry.MutedMembers,
				TroublemakerIsParticipating:             rethinkdbEntry.TroublemakerIsParticipating,
				TroublemakerLogChannel:                  rethinkdbEntry.TroublemakerLogChannel,
				AutoRoleIDs:                             rethinkdbEntry.AutoRoleIDs,
				DelayedAutoRoles:                        rethinkdbEntry.DelayedAutoRoles,
				StarboardChannelID:                      rethinkdbEntry.StarboardChannelID,
				StarboardMinimum:                        rethinkdbEntry.StarboardMinimum,
				StarboardEmoji:                          rethinkdbEntry.StarboardEmoji,
				ChatlogDisabled:                         rethinkdbEntry.ChatlogDisabled,
				EventlogDisabled:                        rethinkdbEntry.EventlogDisabled,
				EventlogChannelIDs:                      rethinkdbEntry.EventlogChannelIDs,
				PersistencyBiasEnabled:                  rethinkdbEntry.PersistencyBiasEnabled,
				PersistencyRoleIDs:                      rethinkdbEntry.PersistencyRoleIDs,
				RandomPicturesPicDelay:                  rethinkdbEntry.RandomPicturesPicDelay,
				RandomPicturesPicDelayIgnoredChannelIDs: rethinkdbEntry.RandomPicturesPicDelayIgnoredChannelIDs,
				PerspectiveIsParticipating:              rethinkdbEntry.PerspectiveIsParticipating,
				PerspectiveChannelID:                    rethinkdbEntry.PerspectiveChannelID,
				CustomCommandsEveryoneCanAdd:            rethinkdbEntry.CustomCommandsEveryoneCanAdd,
				CustomCommandsAddRoleID:                 rethinkdbEntry.CustomCommandsAddRoleID,
				AdminRoleIDs:                            rethinkdbEntry.AdminRoleIDs,
				ModRoleIDs:                              rethinkdbEntry.ModRoleIDs,
			},
		)
		if err != nil {
			panic(err)
		}

		bar.Increment()
	}

	if cursor.Err() != nil {
		panic(err)
	}
	bar.Finish()

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb guild_configs")
	_, err = gorethink.TableDrop("guild_configs").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
