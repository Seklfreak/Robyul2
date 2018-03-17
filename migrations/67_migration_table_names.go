package migrations

import (
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
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

	previousUsernames := make(map[string]string, 0)
	previousNicknames := make(map[string]string, 0)
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
		if rethinkdbEntry.GuildID == "global" {
			// check for username duplicate first
			if previousUsername, ok := previousUsernames[rethinkdbEntry.UserID]; ok && previousUsername == rethinkdbEntry.Username {
				cache.GetLogger().WithField("module", "migrations").Infof("skipped username %s for #%s because already in DB", rethinkdbEntry.Username, rethinkdbEntry.UserID)
				bar.Increment()
				continue
			}
			previousUsernames[rethinkdbEntry.UserID] = rethinkdbEntry.Username
		} else {
			// check for nickname duplicate first
			if previousNickname, ok := previousNicknames[rethinkdbEntry.UserID+rethinkdbEntry.GuildID]; ok && previousNickname == rethinkdbEntry.Nickname {
				cache.GetLogger().WithField("module", "migrations").Infof("skipped nickname %s for #%s because already in DB", rethinkdbEntry.Nickname, rethinkdbEntry.UserID)
				bar.Increment()
				continue
			}
			previousNicknames[rethinkdbEntry.UserID+rethinkdbEntry.GuildID] = rethinkdbEntry.Nickname
		}

		_, err = helpers.MDbInsertWithoutLogging(
			models.NamesTable,
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
