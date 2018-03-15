package migrations

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/globalsign/mgo/bson"
	"github.com/gorethink/gorethink"
)

func m64_migration_table_notifications_ignored_channels() {
	if !TableExists("notifications_ignored_channels") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving notifications_ignored_channels to mongodb")

	cursor, err := gorethink.Table("notifications_ignored_channels").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("notifications_ignored_channels").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID        string `gorethink:"id,omitempty"`
		GuildID   string `gorethink:"guildid"`
		ChannelID string `gorethink:"channelid"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		err = helpers.MDbUpsert(
			models.NotificationsIgnoredChannelsTable,
			bson.M{"channelid": rethinkdbEntry.ChannelID, "guildid": rethinkdbEntry.GuildID},
			models.NotificationsIgnoredChannelsEntry{
				GuildID:   rethinkdbEntry.GuildID,
				ChannelID: rethinkdbEntry.ChannelID,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb notifications_ignored_channels")
	_, err = gorethink.TableDrop("notifications_ignored_channels").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
