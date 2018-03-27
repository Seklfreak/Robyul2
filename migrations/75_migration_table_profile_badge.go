package migrations

import (
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m75_migration_table_profile_badge() {
	if !TableExists("profile_badge") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving profile_badge to mongodb")

	cursor, err := gorethink.Table("profile_badge").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("profile_badge").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID               string    `gorethink:"id,omitempty"`
		CreatedByUserID  string    `gorethink:"createdby_userid"`
		Name             string    `gorethink:"name"`
		Category         string    `gorethink:"category"`
		BorderColor      string    `gorethink:"bordercolor"`
		GuildID          string    `gorethink:"guildid"`
		CreatedAt        time.Time `gorethink:"createdat"`
		URL              string    `gorethink:"url"`
		LevelRequirement int       `gorethink:"levelrequirement"`
		RoleRequirement  string    `gorethink:"rolerequirement"`
		AllowedUserIDs   []string  `gorethinK:"allowed_userids"`
		DeniedUserIDs    []string  `gorethinK:"allowed_userids"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		_, err = helpers.MDbInsertWithoutLogging(
			models.ProfileBadgesTable,
			models.ProfileBadgeEntry{
				OldID:            rethinkdbEntry.ID,
				CreatedByUserID:  rethinkdbEntry.CreatedByUserID,
				Name:             rethinkdbEntry.Name,
				Category:         rethinkdbEntry.Category,
				BorderColor:      rethinkdbEntry.BorderColor,
				GuildID:          rethinkdbEntry.GuildID,
				CreatedAt:        rethinkdbEntry.CreatedAt,
				URL:              rethinkdbEntry.URL,
				ObjectName:       "",
				LevelRequirement: rethinkdbEntry.LevelRequirement,
				RoleRequirement:  rethinkdbEntry.RoleRequirement,
				AllowedUserIDs:   rethinkdbEntry.AllowedUserIDs,
				DeniedUserIDs:    rethinkdbEntry.DeniedUserIDs,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb profile_badge")
	_, err = gorethink.TableDrop("profile_badge").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
