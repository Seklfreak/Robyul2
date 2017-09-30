package migrations

import (
	"github.com/Seklfreak/Robyul2/helpers"
	rethink "github.com/gorethink/gorethink"
)

func m31_create_table_names() {
	CreateTableIfNotExists("names")

	rethink.Table("names").IndexCreate("user_id").Run(helpers.GetDB())
}
