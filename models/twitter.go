package models

import "github.com/globalsign/mgo/bson"

const (
	TwitterTable MongoDbCollection = "twitter"

	TwitterPostModeRobyulEmbed TwitterPostMode = iota
	TwitterPostModeDiscordEmbed
	TwitterPostModeText
)

type TwitterPostMode int

type TwitterEntry struct {
	ID                bson.ObjectId `bson:"_id,omitempty"`
	GuildID           string
	ChannelID         string
	AccountScreenName string
	AccountID         string
	PostedTweets      []TwitterTweetEntry
	MentionRoleID     string
	PostMode          TwitterPostMode
	ExcludeRTs        bool
	ExcludeMentions   bool
}

type TwitterTweetEntry struct {
	ID        string
	CreatedAt string
}
