package migrations

import (
	"github.com/Seklfreak/Robyul2/helpers"
	rethink "github.com/gorethink/gorethink"
)

func m30_create_table_autoleaver_whitelist() {
	CreateTableIfNotExists("autoleaver_whitelist")

	rethink.Table("autoleaver_whitelist").IndexCreate("guild_id").Run(helpers.GetDB())
}
