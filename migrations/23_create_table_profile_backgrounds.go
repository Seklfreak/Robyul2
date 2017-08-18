package migrations

import (
	"github.com/Seklfreak/Robyul2/helpers"
	rethink "github.com/gorethink/gorethink"
)

func m23_create_table_profile_backgrounds() {
	CreateTableIfNotExists("profile_backgrounds")

	rethink.Table("profile_backgrounds").IndexCreate("name").Run(helpers.GetDB())
}
