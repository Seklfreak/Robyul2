package migrations

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m91_migration_table_persistency_roles() {
	if !TableExists("persistency_roles") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving persistency_roles to mongodb")

	cursor, err := gorethink.Table("persistency_roles").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("persistency_roles").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID      string   `rethink:"id,omitempty"`
		GuildID string   `rethink:"guild_id"`
		UserID  string   `rethink:"user_id"`
		Roles   []string `rethink:"roles"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		_, err = helpers.MDbInsertWithoutLogging(
			models.PersistencyRolesTable,
			models.PersistencyRolesEntry{
				GuildID: rethinkdbEntry.GuildID,
				UserID:  rethinkdbEntry.UserID,
				Roles:   rethinkdbEntry.Roles,
			},
		)
		if err != nil {
			panic(err)
		}

		bar.Increment()
	}

	if cursor.Err() != nil {
		panic(err)
	}
	bar.Finish()

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb persistency_roles")
	_, err = gorethink.TableDrop("persistency_roles").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
