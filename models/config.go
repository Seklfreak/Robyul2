package models

type Config struct {
    Id                     string `rethink:"id,omitempty"`
    Guild                  string `rethink:"guild"`

    Prefix                 string `rethink:"prefix"`

    CleanupEnabled         bool `rethink:"cleanup_enabled"`

    AnnouncementsEnabled   bool   `rethink:"announcements_enabled"`
    AnnouncementsChannel   string `rethink:"announcements_channel"`

    WelcomeNewUsersEnabled bool   `rethink:"welcome_new_users_enabled"`
    WelcomeNewUsersText    string `rethink:"welcome_new_users_text"`
}

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
