package migrations

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m90_migration_table_randompicture_source() {
	if !TableExists("randompictures_sources") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving randompictures_sources to mongodb")

	cursor, err := gorethink.Table("randompictures_sources").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("randompictures_sources").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID                 string   `gorethink:"id,omitempty"`
		GuildID            string   `gorethink:"guildid"`
		PostToChannelIDs   []string `gorethink:"post_to_channelids"`
		DriveFolderIDs     []string `gorethink:"drive_folderids"`
		Aliases            []string `gorethink:"aliases"`
		BlacklistedRoleIDs []string `gorethink:"blacklisted_role_ids"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		_, err = helpers.MDbInsertWithoutLogging(
			models.RandompictureSourcesTable,
			models.RandompictureSourceEntry{
				PreviousID:         rethinkdbEntry.ID,
				GuildID:            rethinkdbEntry.GuildID,
				PostToChannelIDs:   rethinkdbEntry.PostToChannelIDs,
				DriveFolderIDs:     rethinkdbEntry.DriveFolderIDs,
				Aliases:            rethinkdbEntry.Aliases,
				BlacklistedRoleIDs: rethinkdbEntry.BlacklistedRoleIDs,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb randompictures_sources")
	_, err = gorethink.TableDrop("randompictures_sources").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
