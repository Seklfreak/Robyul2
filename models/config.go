package models

// Config is a struct describing all config options a guild may set
type Config struct {
    Id string `rethink:"id,omitempty"`
    // Guild contains the guild ID
    Guild string `rethink:"guild"`

    Prefix string `rethink:"prefix"`

    CleanupEnabled bool `rethink:"cleanup_enabled"`

    AnnouncementsEnabled bool   `rethink:"announcements_enabled"`
    // AnnouncementsChannel stores the channel ID
    AnnouncementsChannel string `rethink:"announcements_channel"`

    WelcomeNewUsersEnabled bool   `rethink:"welcome_new_users_enabled"`
    WelcomeNewUsersText    string `rethink:"welcome_new_users_text"`

    MutedRoleName string `rethink:"muted_role_name"`

    // Polls contains all open polls for the guild,
    // closed polls are also stored but will be auto-deleted
    // one day after its state changes to closed
    Polls []Poll `rethink:"polls"`

    InspectTriggersEnabled struct {
        UserBannedOnOtherServers bool
        UserNoCommonServers      bool
        UserNewlyCreatedAccount  bool
        UserReported             bool
        UserMultipleJoins        bool
    }   `rethink:"inspect_triggers_enabled"`
    InspectsChannel string `rethink:"inspects_channel"`

    NukeIsParticipating bool `rethink:"nuke_participation"`
    NukeLogChannel      string `rethink:"nuke_channel"`

    LevelsIgnoredUserIDs []string `rethink:"levels_ignored_user_ids"`
    LevelsIgnoredChannelIDs []string `rethink:"levels_ignored_channel_ids"`

    MutedMembers []string `rethink:"muted_member_ids"`

    TroublemakerIsParticipating bool `rethink:"troublemaker_participation"`
    TroublemakerLogChannel      string `rethink:"troublemaker_channel"`

    LevelsMaxBadges      int `rethink:"levels_maxbadges"`

    AutoRoleIDs []string `rethink:"autorole_roleids"`
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
