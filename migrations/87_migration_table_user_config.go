package migrations

import (
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m87_migration_table_user_config() {
	if !TableExists("user_config") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving user_config to mongodb")

	cursor, err := gorethink.Table("user_config").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("user_config").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID          string    `rethink:"id,omitempty"`
		UserID      string    `rethink:"user_id"`
		Key         string    `rethink:"key"`
		Value       []byte    `rethink:"value"`
		LastChanged time.Time `rethink:"last_changed"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		_, err = helpers.MDbInsertWithoutLogging(
			models.UserConfigTable,
			models.UserConfigEntry{
				UserID:      rethinkdbEntry.UserID,
				Key:         rethinkdbEntry.Key,
				Value:       rethinkdbEntry.Value,
				LastChanged: rethinkdbEntry.LastChanged,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb user_config")
	_, err = gorethink.TableDrop("user_config").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
