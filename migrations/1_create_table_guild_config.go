package migrations

import (
	"github.com/Seklfreak/Robyul2/helpers"
	rethink "github.com/gorethink/gorethink"
)

func m1_create_table_guild_config() {
	CreateTableIfNotExists("guild_configs")

	rethink.Table("guild_configs").IndexCreate("guild").Run(helpers.GetDB())
}
