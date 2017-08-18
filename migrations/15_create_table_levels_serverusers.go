package migrations

import (
	"github.com/Seklfreak/Robyul2/helpers"
	rethink "github.com/gorethink/gorethink"
)

func m15_create_table_levels_serverusers() {
	CreateTableIfNotExists("levels_serverusers")

	rethink.Table("levels_serverusers").IndexCreate("userid").Run(helpers.GetDB())
}
