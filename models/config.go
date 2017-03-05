package models

// Config is a struct describing all config options a guild may set
type Config struct {
    Id                     string `rethink:"id,omitempty"`
    Guild                  string `rethink:"guild"`

    Prefix                 string `rethink:"prefix"`

    CleanupEnabled         bool `rethink:"cleanup_enabled"`

    AnnouncementsEnabled   bool   `rethink:"announcements_enabled"`
    // AnnouncementsChannel stores the channel ID
    AnnouncementsChannel   string `rethink:"announcements_channel"`

    WelcomeNewUsersEnabled bool   `rethink:"welcome_new_users_enabled"`
    WelcomeNewUsersText    string `rethink:"welcome_new_users_text"`
}

// Default is a helper for generating default config values
func (c Config) Default(guild string) Config {
    return Config{
        Guild: guild,

        Prefix: "%",

        CleanupEnabled: false,

        AnnouncementsEnabled: false,
        AnnouncementsChannel: "",

        WelcomeNewUsersEnabled: false,
        WelcomeNewUsersText:    "",
    }
}
