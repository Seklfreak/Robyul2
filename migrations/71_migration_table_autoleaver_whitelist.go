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

func m71_migration_table_autoleaver_whitelist() {
	if !TableExists("autoleaver_whitelist") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving autoleaver_whitelist to mongodb")

	cursor, err := gorethink.Table("autoleaver_whitelist").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("autoleaver_whitelist").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID            string    `rethink:"id,omitempty"`
		AddedAt       time.Time `rethink:"added_at"`
		GuildID       string    `rethink:"guild_id"`
		AddedByUserID string    `rethink:"added_by_user_id"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		err = helpers.MDbUpsertWithoutLogging(
			models.AutoleaverWhitelistTable,
			bson.M{"guildid": rethinkdbEntry.GuildID},
			models.AutoleaverWhitelistEntry{
				AddedAt:       rethinkdbEntry.AddedAt,
				GuildID:       rethinkdbEntry.GuildID,
				AddedByUserID: rethinkdbEntry.AddedByUserID,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb autoleaver_whitelist")
	_, err = gorethink.TableDrop("autoleaver_whitelist").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
