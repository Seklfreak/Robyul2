package migrations

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m81_migration_table_youtube_channels() {
	if !TableExists("youtube_channels") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving youtube_channels to mongodb")

	cursor, err := gorethink.Table("youtube_channels").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("youtube_channels").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		// Discord related fields.
		ID                      string `gorethink:"id,omitempty"`
		ServerID                string `gorethink:"server_id"`
		ChannelID               string `gorethink:"channel_id"`
		NextCheckTime           int64  `gorethink:"next_check_time"`
		LastSuccessfulCheckTime int64  `gorethink:"last_successful_check_time"`

		// Youtube channel specific fields.
		YoutubeChannelID    string   `gorethink:"youtube_channel_id"`
		YoutubeChannelName  string   `gorethink:"youtube_channel_name"`
		YoutubePostedVideos []string `gorethink:"youtube_posted_videos"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		_, err = helpers.MDbInsertWithoutLogging(
			models.YoutubeChannelTable,
			models.YoutubeChannelEntry{
				GuildID:                 rethinkdbEntry.ServerID,
				ChannelID:               rethinkdbEntry.ChannelID,
				NextCheckTime:           rethinkdbEntry.NextCheckTime,
				LastSuccessfulCheckTime: rethinkdbEntry.LastSuccessfulCheckTime,
				YoutubeChannelID:        rethinkdbEntry.YoutubeChannelID,
				YoutubeChannelName:      rethinkdbEntry.YoutubeChannelName,
				YoutubePostedVideos:     rethinkdbEntry.YoutubePostedVideos,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb youtube_channels")
	_, err = gorethink.TableDrop("youtube_channels").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
