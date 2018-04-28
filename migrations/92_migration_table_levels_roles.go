package migrations

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m92_migration_table_levels_roles() {
	if !TableExists("levels_roles") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving levels_roles to mongodb")

	cursor, err := gorethink.Table("levels_roles").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("levels_roles").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID         string `rethink:"id,omitempty"`
		GuildID    string `rethink:"guild_id"`
		RoleID     string `rethink:"role_id"`
		StartLevel int    `rethink:"start_level"`
		LastLevel  int    `rethink:"last_level"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		_, err = helpers.MDbInsertWithoutLogging(
			models.LevelsRolesTable,
			models.LevelsRoleEntry{
				GuildID:    rethinkdbEntry.GuildID,
				RoleID:     rethinkdbEntry.RoleID,
				StartLevel: rethinkdbEntry.StartLevel,
				LastLevel:  rethinkdbEntry.LastLevel,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb levels_roles")
	_, err = gorethink.TableDrop("levels_roles").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
