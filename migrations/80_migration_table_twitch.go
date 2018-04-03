package migrations

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m80_migration_table_twitch() {
	if !TableExists("twitch") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving twitch to mongodb")

	cursor, err := gorethink.Table("twitch").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("twitch").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID                string `gorethink:"id,omitempty"`
		ServerID          string `gorethink:"serverid"`
		ChannelID         string `gorethink:"channelid"`
		TwitchChannelName string `gorethink:"twitchchannelname"`
		IsLive            bool   `gorethink:"islive"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		_, err = helpers.MDbInsertWithoutLogging(
			models.TwitchTable,
			models.TwitchEntry{
				GuildID:           rethinkdbEntry.ServerID,
				ChannelID:         rethinkdbEntry.ChannelID,
				TwitchChannelName: rethinkdbEntry.TwitchChannelName,
				IsLive:            rethinkdbEntry.IsLive,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb twitch")
	_, err = gorethink.TableDrop("twitch").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
