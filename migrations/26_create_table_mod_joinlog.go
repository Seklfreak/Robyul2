package migrations

import (
	"github.com/Seklfreak/Robyul2/helpers"
	rethink "github.com/gorethink/gorethink"
)

func m26_create_table_mod_joinlog() {
	CreateTableIfNotExists("mod_joinlog")

	rethink.Table("mod_joinlog").IndexCreate("userid").Run(helpers.GetDB())
}
