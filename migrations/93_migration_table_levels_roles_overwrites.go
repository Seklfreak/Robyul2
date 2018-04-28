package migrations

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m93_migration_table_levels_roles_overwrites() {
	if !TableExists("levels_roles_overwrites") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving levels_roles_overwrites to mongodb")

	cursor, err := gorethink.Table("levels_roles_overwrites").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("levels_roles_overwrites").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID      string `rethink:"id,omitempty"`
		GuildID string `rethink:"guild_id"`
		RoleID  string `rethink:"role_id"`
		UserID  string `rethink:"user_id"`
		Type    string `rethink:"type"` // "grant" or "deny"
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		theType := models.LevelsRoleOverwriteTypeGrant
		if rethinkdbEntry.Type == "deny" {
			theType = models.LevelsRoleOverwriteTypeDeny
		}

		_, err = helpers.MDbInsertWithoutLogging(
			models.LevelsRoleOverwritesTable,
			models.LevelsRoleOverwriteEntry{
				GuildID: rethinkdbEntry.GuildID,
				RoleID:  rethinkdbEntry.RoleID,
				UserID:  rethinkdbEntry.UserID,
				Type:    theType,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb levels_roles_overwrites")
	_, err = gorethink.TableDrop("levels_roles_overwrites").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
