package migrations

import (
	"github.com/Seklfreak/Robyul2/helpers"
	rethink "github.com/gorethink/gorethink"
)

func m34_create_table_levels_roles() {
	CreateTableIfNotExists("levels_roles")

	rethink.Table("levels_roles").IndexCreate("guild_id").Run(helpers.GetDB())
}
