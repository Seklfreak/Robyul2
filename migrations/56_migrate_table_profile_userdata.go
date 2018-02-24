package migrations

import (
	"time"

	"gopkg.in/mgo.v2/bson"

	"strings"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m56_migratetable_profile_userdate() {
	if !TableExists("profile_userdata") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving profile_userdata to mongodb")

	cursor, err := gorethink.Table("profile_userdata").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("profile_userdata").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID                string    `gorethink:"id,omitempty"`
		UserID            string    `gorethink:"userid"`
		Background        string    `gorethink:"background"`
		Title             string    `gorethink:"title"`
		Bio               string    `gorethink:"bio"`
		Rep               int       `gorethink:"rep"`
		LastRepped        time.Time `gorethink:"last_repped"`
		ActiveBadgeIDs    []string  `gorethink:"active_badgeids"`
		BackgroundColor   string    `gorethink:"background_color"`
		AccentColor       string    `gorethink:"accent_color"`
		TextColor         string    `gorethink:"text_color"`
		BackgroundOpacity string    `gorethink:"background_opacity"`
		DetailOpacity     string    `gorethink:"detail_opacity"`
		Timezone          string    `gorethink:"timezone"`
		Birthday          string    `gorethink:"birthday"`
	}

	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		err = helpers.MDbUpsert(
			models.ProfileUserdataTable,
			bson.M{"userid": rethinkdbEntry.UserID},
			models.ProfileUserdataEntry{
				UserID:            rethinkdbEntry.UserID,
				Background:        strings.ToLower(rethinkdbEntry.Background),
				Title:             rethinkdbEntry.Title,
				Bio:               rethinkdbEntry.Bio,
				Rep:               rethinkdbEntry.Rep,
				LastRepped:        rethinkdbEntry.LastRepped,
				ActiveBadgeIDs:    rethinkdbEntry.ActiveBadgeIDs,
				BackgroundColor:   rethinkdbEntry.BackgroundColor,
				AccentColor:       rethinkdbEntry.AccentColor,
				TextColor:         rethinkdbEntry.TextColor,
				BackgroundOpacity: rethinkdbEntry.BackgroundOpacity,
				DetailOpacity:     rethinkdbEntry.DetailOpacity,
				Timezone:          rethinkdbEntry.Timezone,
				Birthday:          rethinkdbEntry.Birthday,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb profile_userdata")

	_, err = gorethink.TableDrop("profile_userdata").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
