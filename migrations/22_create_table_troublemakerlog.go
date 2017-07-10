package migrations

import (
    rethink "github.com/gorethink/gorethink"
    "github.com/Seklfreak/Robyul2/helpers"
)

func m22_create_table_troublemakerlog() {
    CreateTableIfNotExists("troublemakerlog")

    rethink.Table("troublemakerlog").IndexCreate("userid").Run(helpers.GetDB())
}
