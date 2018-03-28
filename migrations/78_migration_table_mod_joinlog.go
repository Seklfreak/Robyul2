package migrations

import (
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m78_migration_table_mod_joinlog() {
	if !TableExists("mod_joinlog") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving mod_joinlog to mongodb")

	cursor, err := gorethink.Table("mod_joinlog").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("mod_joinlog").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID                        string    `gorethink:"id,omitempty"`
		GuildID                   string    `gorethink:"guildid"`
		UserID                    string    `gorethink:"userid"`
		JoinedAt                  time.Time `gorethink:"joinedat"`
		InviteCodeUsed            string    `gorethink:"invitecode"`
		InviteCodeCreatedByUserID string    `gorethink:"invitecode_createdbyuserid"`
		InviteCodeCreatedAt       time.Time `gorethink:"invitecode_createdat"`
		VanityInviteUsedName      string    `gorethink:"vanityinvite_name"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		_, err = helpers.MDbInsertWithoutLogging(
			models.ModJoinlogTable,
			models.ModJoinlogEntry{
				GuildID:                   rethinkdbEntry.GuildID,
				UserID:                    rethinkdbEntry.UserID,
				JoinedAt:                  rethinkdbEntry.JoinedAt,
				InviteCodeUsed:            rethinkdbEntry.InviteCodeUsed,
				InviteCodeCreatedByUserID: rethinkdbEntry.InviteCodeCreatedByUserID,
				InviteCodeCreatedAt:       rethinkdbEntry.InviteCodeCreatedAt,
				VanityInviteUsedName:      rethinkdbEntry.VanityInviteUsedName,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb mod_joinlog")
	_, err = gorethink.TableDrop("mod_joinlog").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
