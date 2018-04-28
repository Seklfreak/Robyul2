package migrations

import (
	"reflect"
	"runtime"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
)

var migrations = []helpers.Callback{
	m28_create_elastic_indexes,
	m29_create_elastic_presence_update_index,
	m43_create_elastic_vanityinvite_click_index,
	m45_create_elastic_index_messages,
	m46_create_elastic_index_joins,
	m47_create_elastic_index_leaves,
	m48_create_elastic_index_reactions,
	m49_create_elastic_index_presence_updates,
	m50_create_elastic_vanity_invite_clicks,
	m51_reindex_elasticv5_to_v6,
	m52_create_elastic_index_voice_sessions,
	m53_move_rethinkdb_voicesessions_to_elasticsearch,
	m55_create_elastic_index_eventlogs,
	m56_migratetable_profile_userdate,
	m57_migrate_table_levels_serverusers,
	m58_migrate_table_customcommands,
	m59_migrate_table_dog,
	m60_migrate_table_greeter,
	m61_migrate_table_nukelog,
	m62_migration_table_lastfm,
	m63_migration_table_notifications,
	m64_migration_table_notifications_ignored_channels,
	m65_migration_table_twitter,
	m66_migration_table_donators,
	m67_migration_table_names,
	m68_migration_table_module_permission,
	m69_migration_table_bot_status,
	m70_migration_table_weather_last_locations,
	m71_migration_table_autoleaver_whitelist,
	m72_migration_table_reminders,
	m73_migration_table_facebook,
	m74_migration_table_instagram,
	m75_migration_table_profile_badge,
	m76_migration_table_profile_backgrounds,
	m77_migration_table_bias,
	m78_migration_table_mod_joinlog,
	m79_migration_table_vlive,
	m80_migration_table_twitch,
	m81_migration_table_youtube_channels,
	m82_migration_table_reddit_subreddits,
	m83_migration_table_galleries,
	m84_migration_table_reactionpolls,
	m85_migration_table_mirrors,
	m86_migration_table_vanity_invites,
	m87_migration_table_user_config,
	m88_migration_table_troublemakerlog,
	m89_migration_table_starboard_entries,
	m90_migration_table_randompicture_source,
	m91_migration_table_persistency_roles,
	m92_migration_table_levels_roles,
	m93_migration_table_levels_roles_overwrites,
	m94_migration_table_bot_config,
	m95_migration_table_guild_configs,
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
