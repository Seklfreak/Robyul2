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

func m59_migrate_table_dog() {
	if !TableExists("dog_links") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving dog_links to mongodb")

	cursor, err := gorethink.Table("dog_links").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("dog_links").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID            string    `gorethink:"id,omitempty"`
		URL           string    `gorethink:"url"`
		AddedByUserID string    `gorethink:"added_by_userid"`
		AddedAt       time.Time `gorethink:"added_at"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		err = helpers.MDbUpsert(
			models.DogLinksTable,
			bson.M{"url": rethinkdbEntry.URL},
			models.DogLinkEntry{
				URL:           rethinkdbEntry.URL,
				AddedByUserID: rethinkdbEntry.AddedByUserID,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb dog_links")
	_, err = gorethink.TableDrop("dog_links").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
