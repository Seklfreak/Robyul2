package migrations

import (
    "git.lukas.moe/sn0w/Karen/helpers"
    "git.lukas.moe/sn0w/Karen/logger"
    "reflect"
    "runtime"
)

var migrations = []helpers.Callback{
    m0_create_db,
    m1_create_table_guild_config,
    m2_create_table_reminders,
    m3_create_table_music,
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
