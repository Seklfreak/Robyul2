package migrations

import (
    "fmt"
    "runtime"
    "reflect"
)

type Migration func()

var Migrations = []Migration{
    M0_create_db,
    M1_create_table_guild_config,
    M2_create_table_reminders,
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
