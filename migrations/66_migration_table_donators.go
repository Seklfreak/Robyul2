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

func m66_migration_table_donators() {
	if !TableExists("donators") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving donators to mongodb")

	cursor, err := gorethink.Table("donators").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("donators").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID            string    `gorethink:"id,omitempty"`
		Name          string    `gorethink:"name"`
		HeartOverride string    `gorethink:"heart_override"`
		AddedAt       time.Time `gorethink:"added_at"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		err = helpers.MDbUpsert(
			models.DonatorsTable,
			bson.M{"userid": rethinkdbEntry.Name, "addedat": rethinkdbEntry.AddedAt},
			models.DonatorEntry{
				Name:          rethinkdbEntry.Name,
				HeartOverride: rethinkdbEntry.HeartOverride,
				AddedAt:       rethinkdbEntry.AddedAt,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb donators")
	_, err = gorethink.TableDrop("donators").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
