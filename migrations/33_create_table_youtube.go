package migrations

import (
	"github.com/Seklfreak/Robyul2/helpers"
	rethink "github.com/gorethink/gorethink"
)

func m33_create_table_youtube() {
	CreateTableIfNotExists("youtube")

	rethink.Table("youtube").IndexCreate("next_check_time").Run(helpers.GetDB())
}
