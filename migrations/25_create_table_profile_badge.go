package migrations

import (
    rethink "github.com/gorethink/gorethink"
    "github.com/Seklfreak/Robyul2/helpers"
)

func m25_create_table_profile_badge() {
    CreateTableIfNotExists("profile_badge")

    rethink.Table("profile_badge").IndexCreate("category").Run(helpers.GetDB())
    rethink.Table("profile_badge").IndexCreate("name").Run(helpers.GetDB())
    rethink.Table("profile_badge").IndexCreate("guildid").Run(helpers.GetDB())
}
