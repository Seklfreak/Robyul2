package models

import "time"

// Config is a struct describing all config options a guild may set
type Config struct {
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

	// Polls contains all open polls for the guild,
	// closed polls are also stored but will be auto-deleted
	// one day after its state changes to closed
	Polls []Poll `rethink:"polls"`

	InspectTriggersEnabled InspectTriggersEnabled `rethink:"inspect_triggers_enabled"`
	InspectsChannel        string                 `rethink:"inspects_channel"`

	NukeIsParticipating bool   `rethink:"nuke_participation"`
	NukeLogChannel      string `rethink:"nuke_channel"`

	LevelsIgnoredUserIDs    []string `rethink:"levels_ignored_user_ids"`
	LevelsIgnoredChannelIDs []string `rethink:"levels_ignored_channel_ids"`

	MutedMembers []string `rethink:"muted_member_ids"` // deprecated

	TroublemakerIsParticipating bool   `rethink:"troublemaker_participation"`
	TroublemakerLogChannel      string `rethink:"troublemaker_channel"`

	LevelsMaxBadges int `rethink:"levels_maxbadges"`

	AutoRoleIDs      []string          `rethink:"autorole_roleids"`
	DelayedAutoRoles []DelayedAutoRole `rethink:"delayed_autoroles"`

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
}

type InspectTriggersEnabled struct {
	UserBannedOnOtherServers bool
	UserNoCommonServers      bool
	UserNewlyCreatedAccount  bool
	UserReported             bool
	UserMultipleJoins        bool
	UserBannedDiscordlistNet bool // https://bans.discordlist.net/
	UserJoins                bool
}

type DelayedAutoRole struct {
	RoleID string
	Delay  time.Duration
}

// Default is a helper for generating default config values
func (c Config) Default(guild string) Config {
	return Config{
		Guild: guild,

		Prefix: "_",

		CleanupEnabled: false,

		AnnouncementsEnabled: false,
		AnnouncementsChannel: "",

		WelcomeNewUsersEnabled: false,
		WelcomeNewUsersText:    "",

		MutedRoleName: "Muted",
	}
}
