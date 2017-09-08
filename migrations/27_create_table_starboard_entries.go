package migrations

import (
	"github.com/Seklfreak/Robyul2/helpers"
	rethink "github.com/gorethink/gorethink"
)

func m27_create_table_starboard_entries() {
	CreateTableIfNotExists("starboard_entries")

	rethink.Table("starboard_entries").IndexCreate("guild_id").Run(helpers.GetDB())
	rethink.Table("starboard_entries").IndexCreate("message_id").Run(helpers.GetDB())
}
