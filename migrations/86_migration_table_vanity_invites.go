package migrations

import (
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m86_migration_table_vanity_invites() {
	if !TableExists("vanity_invites") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving vanity_invites to mongodb")

	cursor, err := gorethink.Table("vanity_invites").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("vanity_invites").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID               string    `gorethink:"id,omitempty"`
		GuildID          string    `gorethink:"guild_id"`
		ChannelID        string    `gorethink:"channel_id"`
		VanityName       string    `gorethink:"vanity_name"`
		VanityNamePretty string    `gorethink:"vanity_name_pretty"`
		SetByUserID      string    `gorethink:"set_by_user_id"`
		SetAt            time.Time `gorethink:"set_at"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		_, err = helpers.MDbInsertWithoutLogging(
			models.VanityInvitesTable,
			models.VanityInviteEntry{
				GuildID:          rethinkdbEntry.GuildID,
				ChannelID:        rethinkdbEntry.ChannelID,
				VanityName:       rethinkdbEntry.VanityName,
				VanityNamePretty: rethinkdbEntry.VanityNamePretty,
				SetByUserID:      rethinkdbEntry.SetByUserID,
				SetAt:            rethinkdbEntry.SetAt,
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

	return

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb vanity_invites")
	_, err = gorethink.TableDrop("vanity_invites").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
