package migrations

import (
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m94_migration_table_bot_config() {
	if !TableExists("bot_config") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving bot_config to mongodb")

	cursor, err := gorethink.Table("bot_config").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("bot_config").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		Key         string    `rethink:"id,omitempty"`
		Value       []byte    `rethink:"value"`
		LastChanged time.Time `rethink:"last_changed"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		_, err = helpers.MDbInsertWithoutLogging(
			models.BotConfigTable,
			models.BotConfigEntry{
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb bot_config")
	_, err = gorethink.TableDrop("bot_config").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
