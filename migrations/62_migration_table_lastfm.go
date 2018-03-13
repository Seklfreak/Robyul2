package migrations

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/globalsign/mgo/bson"
	"github.com/gorethink/gorethink"
)

func m62_migration_table_lastfm() {
	if !TableExists("lastfm") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving lastfm to mongodb")

	cursor, err := gorethink.Table("lastfm").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("lastfm").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		UserID         string `gorethink:"userid,omitempty"`
		LastFmUsername string `gorethink:"lastfmusername"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		err = helpers.MDbUpsert(
			models.LastFmTable,
			bson.M{"userid": rethinkdbEntry.UserID},
			models.LastFmEntry{
				UserID:         rethinkdbEntry.UserID,
				LastFmUsername: rethinkdbEntry.LastFmUsername,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb lastfm")
	_, err = gorethink.TableDrop("lastfm").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
