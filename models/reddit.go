package models

import (
	"time"

	"github.com/globalsign/mgo/bson"
)

const (
	RedditSubredditsTable MongoDbCollection = "reddit_subreddits"
)

type RedditSubredditEntry struct {
	ID              bson.ObjectId `bson:"_id,omitempty"`
	SubredditName   string
	LastChecked     time.Time
	GuildID         string
	ChannelID       string
	AddedByUserID   string
	AddedAt         time.Time
	PostDelay       int
	PostDirectLinks bool
}
