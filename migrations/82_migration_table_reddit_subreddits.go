package migrations

import (
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m82_migration_table_reddit_subreddits() {
	if !TableExists("reddit_subreddits") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving reddit_subreddits to mongodb")

	cursor, err := gorethink.Table("reddit_subreddits").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("reddit_subreddits").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID              string    `rethink:"id,omitempty"`
		SubredditName   string    `rethink:"subreddit_name"`
		LastChecked     time.Time `rethink:"last_checked"`
		GuildID         string    `rethink:"guild_id"`
		ChannelID       string    `rethink:"channel_id"`
		AddedByUserID   string    `rethink:"addedby_user_id"`
		AddedAt         time.Time `rethink:"addedat"`
		PostDelay       int       `rethink:"post_delay"`
		PostDirectLinks bool      `rethink:"post_direct_links"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		_, err = helpers.MDbInsertWithoutLogging(
			models.RedditSubredditsTable,
			models.RedditSubredditEntry{
				SubredditName:   rethinkdbEntry.SubredditName,
				LastChecked:     rethinkdbEntry.LastChecked,
				GuildID:         rethinkdbEntry.GuildID,
				ChannelID:       rethinkdbEntry.ChannelID,
				AddedByUserID:   rethinkdbEntry.AddedByUserID,
				AddedAt:         rethinkdbEntry.AddedAt,
				PostDelay:       rethinkdbEntry.PostDelay,
				PostDirectLinks: rethinkdbEntry.PostDirectLinks,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb reddit_subreddits")
	_, err = gorethink.TableDrop("reddit_subreddits").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
