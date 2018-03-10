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

func m58_migrate_table_customcommands() {
	if !TableExists("customcommands") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving customcommands to mongodb")

	cursor, err := gorethink.Table("customcommands").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("customcommands").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID              string    `gorethink:"id,omitempty"`
		GuildID         string    `gorethink:"guildid"`
		CreatedByUserID string    `gorethink:"createdby_userid"`
		CreatedAt       time.Time `gorethink:"createdat"`
		Triggered       int       `gorethink:"triggered"`
		Keyword         string    `gorethink:"keyword"`
		Content         string    `gorethink:"content"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		err = helpers.MDbUpsert(
			models.CustomCommandsTable,
			bson.M{"keyword": rethinkdbEntry.Keyword, "guildid": rethinkdbEntry.GuildID},
			models.CustomCommandsEntry{
				GuildID:         rethinkdbEntry.GuildID,
				CreatedByUserID: rethinkdbEntry.CreatedByUserID,
				CreatedAt:       rethinkdbEntry.CreatedAt,
				Triggered:       rethinkdbEntry.Triggered,
				Keyword:         rethinkdbEntry.Keyword,
				Content:         rethinkdbEntry.Content,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb customcommands")
	_, err = gorethink.TableDrop("customcommands").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
