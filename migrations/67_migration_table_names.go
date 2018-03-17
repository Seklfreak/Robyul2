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

func m67_migration_table_names() {
	if !TableExists("names") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving names to mongodb")

	cursor, err := gorethink.Table("names").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("names").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID        string    `rethink:"id,omitempty"`
		ChangedAt time.Time `rethink:"changed_at"`
		GuildID   string    `rethink:"guild_id"`
		UserID    string    `rethink:"user_id"`
		Nickname  string    `rethink:"nickname"`
		Username  string    `rethink:"username"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		err = helpers.MDbUpsert(
			models.NamesTable,
			bson.M{"guildid": rethinkdbEntry.GuildID, "userid": rethinkdbEntry.UserID, "changedat": rethinkdbEntry.ChangedAt, "nickname": rethinkdbEntry.Nickname, "username": rethinkdbEntry.Username},
			models.NamesEntry{
				ChangedAt: rethinkdbEntry.ChangedAt,
				GuildID:   rethinkdbEntry.GuildID,
				UserID:    rethinkdbEntry.UserID,
				Nickname:  rethinkdbEntry.Nickname,
				Username:  rethinkdbEntry.Username,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb names")
	_, err = gorethink.TableDrop("names").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
