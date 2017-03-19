package migrations

import (
    "github.com/Seklfreak/Robyul2/helpers"
    "github.com/Seklfreak/Robyul2/logger"
    "reflect"
    "runtime"
)

var migrations = []helpers.Callback{
    m0_create_db,
    m1_create_table_guild_config,
    m2_create_table_reminders,
    m3_create_table_music,
    m4_create_table_vlive,
    m5_create_table_twitter,
    m6_create_table_instagram,
    m7_create_table_facebook,
    m8_create_table_lastfm,
    m9_create_table_bias,
    m10_create_table_guild_announcements,
    m11_create_table_twitch,
    m12_create_table_notifications,
    m13_create_table_notifications_ignored_channels,
    m14_create_table_stats_voicetimes,
    m15_create_table_levels_serverusers,
    m16_create_table_galleries,
    m17_create_table_mirrors,
    m18_create_table_randompictures_sources,
    m19_create_table_customcommands,
    m20_create_table_reactionpolls,
}

// Run executes all registered migrations
func Run() {
    logger.BOOT.L("migrator", "Running migrations...")
    for _, migration := range migrations {
        migrationName := runtime.FuncForPC(
            reflect.ValueOf(migration).Pointer(),
        ).Name()

        logger.BOOT.L("migrator", "Running "+migrationName)
        migration()
    }

    logger.BOOT.L("migrator", "Migrations finished!")
}
