package migrations

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/globalsign/mgo/bson"
	"github.com/gorethink/gorethink"
)

func m63_migration_table_notifications() {
	if !TableExists("notifications") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving notifications to mongodb")

	cursor, err := gorethink.Table("notifications").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("notifications").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID        string `gorethink:"id,omitempty"`
		Keyword   string `gorethink:"keyword"`
		GuildID   string `gorethink:"guildid"` // can be "global" to affect every guild
		UserID    string `gorethink:"userid"`
		Triggered int    `gorethink:"triggered"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		err = helpers.MDbUpsert(
			models.NotificationsTable,
			bson.M{"userid": rethinkdbEntry.UserID, "guildid": rethinkdbEntry.GuildID, "keyword": rethinkdbEntry.Keyword},
			models.NotificationsEntry{
				Keyword:   rethinkdbEntry.Keyword,
				GuildID:   rethinkdbEntry.GuildID,
				UserID:    rethinkdbEntry.UserID,
				Triggered: rethinkdbEntry.Triggered,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb notifications")
	_, err = gorethink.TableDrop("notifications").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
