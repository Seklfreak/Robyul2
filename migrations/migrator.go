package migrations

import (
	"reflect"
	"runtime"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
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
	m21_create_table_nukelog,
	m22_create_table_troublemakerlog,
	m23_create_table_profile_backgrounds,
	m24_create_table_profile_userdata,
	m25_create_table_profile_badge,
	m26_create_table_mod_joinlog,
	m27_create_table_starboard_entries,
	m28_create_elastic_indexes,
	m29_create_elastic_presence_update_index,
	m30_create_table_autoleaver_whitelist,
	m31_create_table_names,
	m32_create_table_reddit_subreddits,
	m33_create_table_youtube_channels,
	m34_create_table_levels_roles,
}

// Run executes all registered migrations
func Run() {
	log := cache.GetLogger()
	log.WithField("module", "migrator").Info("Running migrations...")
	for _, migration := range migrations {
		migrationName := runtime.FuncForPC(
			reflect.ValueOf(migration).Pointer(),
		).Name()

		log.WithField("module", "migrator").Info("Running " + migrationName)
		migration()
	}

	log.WithField("module", "migrator").Info("Migrations finished!")
}
