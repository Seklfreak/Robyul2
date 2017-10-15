package migrations

import (
	"github.com/Seklfreak/Robyul2/helpers"
	rethink "github.com/gorethink/gorethink"
)

func m36_create_table_levels_roles_overwrites() {
	CreateTableIfNotExists("levels_roles_overwrites")

	rethink.Table("levels_roles_overwrites").IndexCreate("guild_id").Run(helpers.GetDB())
	rethink.Table("levels_roles_overwrites").IndexCreate("user_id").Run(helpers.GetDB())
}
