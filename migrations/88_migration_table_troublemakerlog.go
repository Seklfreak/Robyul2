package migrations

import (
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m88_migration_table_troublemakerlog() {
	if !TableExists("troublemakerlog") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving troublemakerlog to mongodb")

	cursor, err := gorethink.Table("troublemakerlog").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("troublemakerlog").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID                string    `gorethink:"id,omitempty"`
		UserID            string    `gorethink:"userid"`
		Reason            string    `gorethink:"reason"`
		CreatedAt         time.Time `gorethink:"createdat"`
		ReportedByGuildID string    `gorethink:"reportedby_guildid"`
		ReportedByUserID  string    `gorethink:"reportedby_userid"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		_, err = helpers.MDbInsertWithoutLogging(
			models.TroublemakerlogTable,
			models.TroublemakerlogEntry{
				UserID:            rethinkdbEntry.UserID,
				Reason:            rethinkdbEntry.Reason,
				CreatedAt:         rethinkdbEntry.CreatedAt,
				ReportedByGuildID: rethinkdbEntry.ReportedByGuildID,
				ReportedByUserID:  rethinkdbEntry.ReportedByUserID,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb troublemakerlog")
	_, err = gorethink.TableDrop("troublemakerlog").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
