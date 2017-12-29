package migrations

import (
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/gorethink/gorethink"
)

func m54_create_table_module_permissions() {
	CreateTableIfNotExists("module_permissions")

	gorethink.Table("module_permissions").IndexCreate("type").Run(helpers.GetDB())
	gorethink.Table("module_permissions").IndexCreate("guild_id").Run(helpers.GetDB())
}
