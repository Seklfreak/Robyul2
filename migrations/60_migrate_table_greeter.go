package migrations

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/globalsign/mgo/bson"
	"github.com/gorethink/gorethink"
)

func m60_migrate_table_greeter() {
	if !TableExists("guild_announcements") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving greeter to mongodb")

	cursor, err := gorethink.Table("guild_announcements").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("guild_announcements").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		Id                  string `rethink:"id,omitempty"`
		GuildID             string `rethink:"guildid"`
		GuildJoinChannelID  string `rethink:"guild_join_channelid"`
		GuildJoinText       string `rethink:"guild_join_text"`
		GuildJoinEnabled    bool   `rethink:"guild_join_enabled"`
		GuildLeaveChannelID string `rethink:"guild_leave_channelid"`
		GuildLeaveText      string `rethink:"guild_leave_text"`
		GuildLeaveEnabled   bool   `rethink:"guild_leave_enabled"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		if rethinkdbEntry.GuildJoinEnabled && rethinkdbEntry.GuildJoinText != "" {
			err = helpers.MDbUpsert(
				models.GreeterTable,
				bson.M{"type": models.GreeterTypeJoin, "guildid": rethinkdbEntry.GuildID},
				models.GreeterEntry{
					GuildID:   rethinkdbEntry.GuildID,
					ChannelID: rethinkdbEntry.GuildJoinChannelID,
					EmbedCode: rethinkdbEntry.GuildJoinText,
					Type:      models.GreeterTypeJoin,
				},
			)
			if err != nil {
				panic(err)
			}
		}
		if rethinkdbEntry.GuildLeaveEnabled && rethinkdbEntry.GuildLeaveText != "" {
			err = helpers.MDbUpsert(
				models.GreeterTable,
				bson.M{"type": models.GreeterTypeLeave, "guildid": rethinkdbEntry.GuildID},
				models.GreeterEntry{
					GuildID:   rethinkdbEntry.GuildID,
					ChannelID: rethinkdbEntry.GuildLeaveChannelID,
					EmbedCode: rethinkdbEntry.GuildLeaveText,
					Type:      models.GreeterTypeLeave,
				},
			)
			if err != nil {
				panic(err)
			}
		}

		bar.Increment()
	}

	if cursor.Err() != nil {
		panic(err)
	}
	bar.Finish()

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb guild_announcements")
	_, err = gorethink.TableDrop("guild_announcements").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
