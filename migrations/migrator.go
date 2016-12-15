package migrations

import "fmt"

type Migration func()

var Migrations = []Migration{
    M0_create_db,
    M1_create_table_guild_config,
}

func Run() {
    fmt.Println("[DB] Running migrations...")
    for _, migration := range Migrations {
        migration()
    }
    fmt.Println("[DB] Migrations finished!")
}