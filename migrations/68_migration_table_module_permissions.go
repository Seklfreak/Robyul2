package migrations

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/globalsign/mgo/bson"
	"github.com/gorethink/gorethink"
)

func m68_migration_table_module_permission() {
	if !TableExists("module_permissions") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving module_permissions to mongodb")

	cursor, err := gorethink.Table("module_permissions").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("module_permissions").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID       string `rethink:"id,omitempty"`
		GuildID  string `rethink:"guild_id"`
		Type     string `rethink:"type"` // "channel" or "role"
		TargetID string `rethink:"target_id"`
		Allowed  int    `rethink:"allowed"` // -1 for unset
		Denied   int    `rethink:"denied"`  // -1 for unset
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		err = helpers.MDbUpsertWithoutLogging(
			models.ModulePermissionsTable,
			bson.M{"guildid": rethinkdbEntry.GuildID, "type": rethinkdbEntry.Type, "targetid": rethinkdbEntry.TargetID},
			models.ModulePermissionEntry{
				GuildID:  rethinkdbEntry.GuildID,
				Type:     rethinkdbEntry.Type,
				TargetID: rethinkdbEntry.TargetID,
				Allowed:  models.ModulePermissionsModule(rethinkdbEntry.Allowed),
				Denied:   models.ModulePermissionsModule(rethinkdbEntry.Denied),
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb module_permissions")
	_, err = gorethink.TableDrop("module_permissions").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
