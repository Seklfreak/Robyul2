package migrations

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m65_migration_table_twitter() {
	if !TableExists("twitter") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving twitter to mongodb")

	cursor, err := gorethink.Table("twitter").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("twitter").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	type DB_Twitter_Tweet struct {
		ID        string `gorethink:"id,omitempty"`
		CreatedAt string `gorethink:"createdat"`
	}
	var rethinkdbEntry struct {
		ID                string             `gorethink:"id,omitempty"`
		ServerID          string             `gorethink:"serverid"`
		ChannelID         string             `gorethink:"channelid"`
		AccountScreenName string             `gorethink:"account_screen_name"`
		PostedTweets      []DB_Twitter_Tweet `gorethink:"posted_tweets"`
		AccountID         string             `gorethink:"account_id"`
		MentionRoleID     string             `gorethink:"mention_role_id"`
		PostMode          int                `gorethink:"post_mode"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		tweetEntries := make([]models.TwitterTweetEntry, 0)
		for _, tweetEntry := range rethinkdbEntry.PostedTweets {
			tweetEntries = append(tweetEntries, models.TwitterTweetEntry{
				ID:        tweetEntry.ID,
				CreatedAt: tweetEntry.CreatedAt,
			})
		}

		_, err = helpers.MDbInsert(
			models.TwitterTable,
			models.TwitterEntry{
				GuildID:           rethinkdbEntry.ServerID,
				ChannelID:         rethinkdbEntry.ChannelID,
				AccountScreenName: rethinkdbEntry.AccountScreenName,
				AccountID:         rethinkdbEntry.AccountID,
				PostedTweets:      tweetEntries,
				MentionRoleID:     rethinkdbEntry.MentionRoleID,
				PostMode:          models.TwitterPostMode(rethinkdbEntry.PostMode), // TODO :test
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb twitter")
	_, err = gorethink.TableDrop("twitter").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
