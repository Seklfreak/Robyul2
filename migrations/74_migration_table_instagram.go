package migrations

import (
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m74_migration_table_instagram() {
	if !TableExists("instagram") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving instagram to mongodb")

	cursor, err := gorethink.Table("instagram").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("instagram").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	type rethinkdbSubPostEntry struct {
		ID        string `gorethink:"id,omitempty"`
		CreatedAt int    `gorethink:"createdat"`
	}
	type rethinkdbSubReelEntry struct {
		ID        string `gorethink:"id,omitempty"`
		CreatedAt int64  `gorethink:"createdat"`
	}
	var rethinkdbEntry struct {
		ID               string                  `gorethink:"id,omitempty"`
		ServerID         string                  `gorethink:"serverid"`
		ChannelID        string                  `gorethink:"channelid"`
		Username         string                  `gorethink:"username"`
		InstagramUserID  int64                   `gorethink:"instagramuserid"`
		PostedPosts      []rethinkdbSubPostEntry `gorethink:"posted_posts"`
		PostedReelMedias []rethinkdbSubReelEntry `gorethink:"posted_reelmedias"`
		IsLive           bool                    `gorethink:"islive"`
		PostDirectLinks  bool                    `gorethink:"post_direct_links"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		instagramPosts := make([]models.InstagramPostEntry, 0)
		for _, oldPost := range rethinkdbEntry.PostedPosts {
			instagramPosts = append(instagramPosts, models.InstagramPostEntry{
				ID:            oldPost.ID,
				Type:          models.InstagramPostTypePost,
				CreatedAtTime: time.Unix(int64(oldPost.CreatedAt), 0),
			})
		}
		for _, oldReel := range rethinkdbEntry.PostedReelMedias {
			instagramPosts = append(instagramPosts, models.InstagramPostEntry{
				ID:            oldReel.ID,
				Type:          models.InstagramPostTypeReel,
				CreatedAtTime: time.Unix(int64(oldReel.CreatedAt), 0),
			})
		}

		sendPostType := models.InstagramSendPostTypeRobyulEmbed
		if rethinkdbEntry.PostDirectLinks {
			sendPostType = models.InstagramSendPostTypeDirectLinks
		}

		_, err = helpers.MDbInsertWithoutLogging(
			models.InstagramTable,
			models.InstagramEntry{
				GuildID:         rethinkdbEntry.ServerID,
				ChannelID:       rethinkdbEntry.ChannelID,
				Username:        rethinkdbEntry.Username,
				InstagramUserID: rethinkdbEntry.InstagramUserID,
				PostedPosts:     instagramPosts,
				IsLive:          rethinkdbEntry.IsLive,
				SendPostType:    sendPostType,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb instagram")
	_, err = gorethink.TableDrop("instagram").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
