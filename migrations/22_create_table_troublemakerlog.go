package migrations

import (
	"github.com/Seklfreak/Robyul2/helpers"
	rethink "github.com/gorethink/gorethink"
)

func m22_create_table_troublemakerlog() {
	CreateTableIfNotExists("troublemakerlog")

	rethink.Table("troublemakerlog").IndexCreate("userid").Run(helpers.GetDB())
}
