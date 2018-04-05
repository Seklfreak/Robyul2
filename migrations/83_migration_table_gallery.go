package migrations

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m83_migration_table_galleries() {
	if !TableExists("galleries") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving galleries to mongodb")

	cursor, err := gorethink.Table("galleries").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("galleries").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		SourceChannelID           string `gorethink:"source_channel_id"`
		TargetChannelID           string `gorethink:"target_channel_id"`
		TargetChannelWebhookID    string `gorethink:"target_channel_webhook_id"`
		TargetChannelWebhookToken string `gorethink:"target_channel_webhook_token"`
		GuildID                   string `gorethink:"guild_id"`
		AddedByUserID             string `gorethink:"addedby_user_id"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		_, err = helpers.MDbInsertWithoutLogging(
			models.GalleryTable,
			models.GalleryEntry{
				SourceChannelID: rethinkdbEntry.SourceChannelID,
				TargetChannelID: rethinkdbEntry.TargetChannelID,
				GuildID:         rethinkdbEntry.GuildID,
				AddedByUserID:   rethinkdbEntry.AddedByUserID,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb galleries")
	_, err = gorethink.TableDrop("galleries").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
