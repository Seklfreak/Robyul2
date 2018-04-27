package migrations

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m85_migration_table_mirrors() {
	if !TableExists("mirrors") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving mirrors to mongodb")

	cursor, err := gorethink.Table("mirrors").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("mirrors").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	type rethinkdbSubEntry struct {
		ChannelID string
		GuildID   string
	}
	var rethinkdbEntry struct {
		ID                string `gorethink:"id,omitempty"`
		Type              string `gorethink:"type"`
		ConnectedChannels []rethinkdbSubEntry
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		theType := models.MirrorTypeLink
		if rethinkdbEntry.Type == "text" {
			theType = models.MirrorTypeText
		}
		channels := make([]models.MirrorChannelEntry, 0)
		for _, rethinkChannel := range rethinkdbEntry.ConnectedChannels {
			channels = append(channels, models.MirrorChannelEntry{
				GuildID:   rethinkChannel.GuildID,
				ChannelID: rethinkChannel.ChannelID,
			})
		}
		_, err = helpers.MDbInsertWithoutLogging(
			models.MirrorsTable,
			models.MirrorEntry{
				Type:              theType,
				ConnectedChannels: channels,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb mirrors")
	_, err = gorethink.TableDrop("mirrors").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
