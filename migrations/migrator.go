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
