package migrations

import (
    "fmt"
    "reflect"
    "runtime"
)

type Migration func()

var Migrations = []Migration{
    m0_create_db,
    m1_create_table_guild_config,
    m2_create_table_reminders,
    m3_create_table_music,
}

// Runs all registered migrations
func Run() {
    fmt.Println("[DB] Running migrations...")
    for _, migration := range Migrations {
        migrationName := runtime.FuncForPC(
            reflect.ValueOf(migration).Pointer(),
        ).Name()

        fmt.Printf("[DB] Running %s\n", migrationName)
        migration()
    }

    fmt.Println("[DB] Migrations finished!")
}
