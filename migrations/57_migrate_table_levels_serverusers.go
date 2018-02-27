package migrations

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/globalsign/mgo/bson"
	"github.com/gorethink/gorethink"
)

func m57_migrate_table_levels_serverusers() {
	if !TableExists("levels_serverusers") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving levels_serverusers to mongodb")

	cursor, err := gorethink.Table("levels_serverusers").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("levels_serverusers").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID      string `gorethink:"id,omitempty"`
		UserID  string `gorethink:"userid"`
		GuildID string `gorethink:"guildid"`
		Exp     int64  `gorethink:"exp"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		err = helpers.MDbUpsert(
			models.LevelsServerusersTable,
			bson.M{"userid": rethinkdbEntry.UserID, "guildid": rethinkdbEntry.GuildID},
			models.LevelsServerusersEntry{
				UserID:  rethinkdbEntry.UserID,
				GuildID: rethinkdbEntry.GuildID,
				Exp:     rethinkdbEntry.Exp,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb levels_serverusers")

	_, err = gorethink.TableDrop("levels_serverusers").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
