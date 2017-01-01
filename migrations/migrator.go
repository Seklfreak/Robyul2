package migrations

import (
    "fmt"
    "reflect"
    "runtime"
    "github.com/sn0w/Karen/helpers"
)

var migrations = []helpers.Callback{
    m0_create_db,
    m1_create_table_guild_config,
    m2_create_table_reminders,
    m3_create_table_music,
}

// Run executes all registered migrations
func Run() {
    fmt.Println("[DB] Running migrations...")
    for _, migration := range migrations {
        migrationName := runtime.FuncForPC(
            reflect.ValueOf(migration).Pointer(),
        ).Name()

        fmt.Printf("[DB] Running %s\n", migrationName)
        migration()
    }

    fmt.Println("[DB] Migrations finished!")
}
