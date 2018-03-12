package migrations

import (
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/globalsign/mgo/bson"
	"github.com/gorethink/gorethink"
)

func m61_migrate_table_nukelog() {
	if !TableExists("nukelog") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving nukelog to mongodb")

	cursor, err := gorethink.Table("nukelog").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("nukelog").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID       string    `gorethink:"id,omitempty"`
		UserID   string    `gorethink:"userid"`
		UserName string    `gorethink:"username"`
		NukerID  string    `gorethink:"nukerid"`
		Reason   string    `gorethink:"reason"`
		NukedAt  time.Time `gorethink:"nukedat"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		err = helpers.MDbUpsert(
			models.NukelogTable,
			bson.M{"userid": rethinkdbEntry.UserID, "nukedat": rethinkdbEntry.NukedAt},
			models.NukelogEntry{
				UserID:   rethinkdbEntry.UserID,
				UserName: rethinkdbEntry.UserName,
				NukerID:  rethinkdbEntry.NukerID,
				Reason:   rethinkdbEntry.Reason,
				NukedAt:  rethinkdbEntry.NukedAt,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb nukelog")
	_, err = gorethink.TableDrop("nukelog").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
