package migrations

import (
	"github.com/Seklfreak/Robyul2/helpers"
	rethink "github.com/gorethink/gorethink"
)

func m35_create_table_persistency_roles() {
	CreateTableIfNotExists("persistency_roles")

	rethink.Table("persistency_roles").IndexCreate("guild_id").Run(helpers.GetDB())
	rethink.Table("persistency_roles").IndexCreate("user_id").Run(helpers.GetDB())
}
