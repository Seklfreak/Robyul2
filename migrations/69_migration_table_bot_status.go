package migrations

import (
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/cheggaaa/pb"
	"github.com/globalsign/mgo/bson"
	"github.com/gorethink/gorethink"
)

func m69_migration_table_bot_status() {
	if !TableExists("bot_status") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving bot_status to mongodb")

	cursor, err := gorethink.Table("bot_status").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("bot_status").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID            string    `rethink:"id,omitempty"`
		AddedByUserID string    `rethink:"added_by_userid"`
		AddedAt       time.Time `rethink:"added_at"`
		Text          string    `rethink:"text"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		err = helpers.MDbUpsertWithoutLogging(
			models.BotStatusTable,
			bson.M{"text": rethinkdbEntry.Text},
			models.BotStatusEntry{
				AddedByUserID: rethinkdbEntry.AddedByUserID,
				AddedAt:       rethinkdbEntry.AddedAt,
				Text:          rethinkdbEntry.Text,
				Type:          discordgo.GameTypeGame,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb bot_status")
	_, err = gorethink.TableDrop("bot_status").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
