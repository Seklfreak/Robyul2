package migrations

import (
    rethink "github.com/gorethink/gorethink"
    "github.com/Seklfreak/Robyul2/helpers"
)

func m26_create_table_mod_joinlog() {
    CreateTableIfNotExists("mod_joinlog")

    rethink.Table("mod_joinlog").IndexCreate("userid").Run(helpers.GetDB())
}
