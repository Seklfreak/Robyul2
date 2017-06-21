package migrations

import (
    rethink "github.com/gorethink/gorethink"
    "github.com/Seklfreak/Robyul2/helpers"
)

func m1_create_table_guild_config() {
    CreateTableIfNotExists("guild_configs")

    rethink.Table("levels_serverusers").IndexCreate("guild").Run(helpers.GetDB())
}
