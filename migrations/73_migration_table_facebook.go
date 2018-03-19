package migrations

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/globalsign/mgo/bson"
	"github.com/gorethink/gorethink"
)

func m73_migration_table_facebook() {
	if !TableExists("facebook") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving facebook to mongodb")

	cursor, err := gorethink.Table("facebook").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("facebook").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	type rethinkdbSubEntry struct {
		ID        string `gorethink:"id,omitempty"`
		CreatedAt string `gorethink:"CreatedAt"`
	}
	var rethinkdbEntry struct {
		ID          string              `gorethink:"id,omitempty"`
		ServerID    string              `gorethink:"serverid"`
		ChannelID   string              `gorethink:"channelid"`
		Username    string              `gorethink:"username"`
		PostedPosts []rethinkdbSubEntry `gorethink:"posted_posts"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		facebookPosts := make([]models.FacebookPostEntry, 0)
		for _, oldPost := range rethinkdbEntry.PostedPosts {
			facebookPosts = append(facebookPosts, models.FacebookPostEntry{
				ID:        oldPost.ID,
				CreatedAt: oldPost.CreatedAt,
			})
		}

		err = helpers.MDbUpsertWithoutLogging(
			models.FacebookTable,
			bson.M{"guildid": rethinkdbEntry.ServerID, "channelid": rethinkdbEntry.ChannelID, "username": rethinkdbEntry.Username},
			models.FacebookEntry{
				GuildID:     rethinkdbEntry.ServerID,
				ChannelID:   rethinkdbEntry.ChannelID,
				Username:    rethinkdbEntry.Username,
				PostedPosts: facebookPosts,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb facebook")
	_, err = gorethink.TableDrop("facebook").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
