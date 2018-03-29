package migrations

import (
	"time"

	"strings"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m76_migration_table_profile_backgrounds() {
	if !TableExists("profile_backgrounds") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving profile_backgrounds to mongodb")

	cursor, err := gorethink.Table("profile_backgrounds").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("profile_backgrounds").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		Name      string    `gorethink:"id,omitempty"`
		URL       string    `gorethink:"url"`
		CreatedAt time.Time `gorethink:"createdat"`
		Tags      []string  `gorethink:"tags"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		_, err = helpers.MDbInsertWithoutLogging(
			models.ProfileBackgroundsTable,
			models.ProfileBackgroundEntry{
				Name:      strings.ToLower(rethinkdbEntry.Name),
				URL:       rethinkdbEntry.URL,
				CreatedAt: rethinkdbEntry.CreatedAt,
				Tags:      rethinkdbEntry.Tags,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb profile_backgrounds")
	_, err = gorethink.TableDrop("profile_backgrounds").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
