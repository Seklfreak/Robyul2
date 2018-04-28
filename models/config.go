package models

import (
	"time"

	"github.com/globalsign/mgo/bson"
)

const (
	GuildConfigTable MongoDbCollection = "guild_configs"
)

// Config is a struct describing all config options a guild may set
type Config struct {
	ID bson.ObjectId `bson:"_id,omitempty"`

	// Guild contains the guild ID
	GuildID string

	Prefix string

	CleanupEnabled bool

	AnnouncementsEnabled bool
	// AnnouncementsChannel stores the channel ID
	AnnouncementsChannel string

	WelcomeNewUsersEnabled bool
	WelcomeNewUsersText    string

	MutedRoleName string

	InspectTriggersEnabled InspectTriggersEnabled
	InspectsChannel        string

	NukeIsParticipating bool
	NukeLogChannel      string

	LevelsIgnoredUserIDs          []string
	LevelsIgnoredChannelIDs       []string
	LevelsNotificationCode        string
	LevelsNotificationDeleteAfter int
	LevelsMaxBadges               int

	MutedMembers []string // deprecated

	TroublemakerIsParticipating bool
	TroublemakerLogChannel      string

	AutoRoleIDs      []string
	DelayedAutoRoles []DelayedAutoRole

	StarboardChannelID string
	StarboardMinimum   int
	StarboardEmoji     []string

	ChatlogDisabled bool

	EventlogDisabled   bool
	EventlogChannelIDs []string

	PersistencyBiasEnabled bool
	PersistencyRoleIDs     []string

	RandomPicturesPicDelay                  int
	RandomPicturesPicDelayIgnoredChannelIDs []string

	PerspectiveIsParticipating bool
	PerspectiveChannelID       string

	CustomCommandsEveryoneCanAdd bool
	CustomCommandsAddRoleID      string

	AdminRoleIDs []string
	ModRoleIDs   []string
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
		GuildID: guild,

		Prefix: "_",

		CleanupEnabled: false,

		AnnouncementsEnabled: false,
		AnnouncementsChannel: "",

		WelcomeNewUsersEnabled: false,
		WelcomeNewUsersText:    "",

		MutedRoleName: "Muted",
	}
}
