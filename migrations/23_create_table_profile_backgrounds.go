package migrations

import (
    rethink "github.com/gorethink/gorethink"
    "github.com/Seklfreak/Robyul2/helpers"
)

func m23_create_table_profile_backgrounds() {
    CreateTableIfNotExists("profile_backgrounds")

    rethink.Table("profile_backgrounds").IndexCreate("name").Run(helpers.GetDB())
}
