package models

import "github.com/globalsign/mgo/bson"

const (
	YoutubeChannelTable  MongoDbCollection = "youtube_channels"
	YoutubeQuotaRedisKey                   = "robyul2-discord:youtube:quota"
)

type YoutubeChannelEntry struct {
	// Discord related fields.
	ID                      bson.ObjectId `bson:"_id,omitempty"`
	GuildID                 string        // renamed from ServerID
	ChannelID               string
	NextCheckTime           int64
	LastSuccessfulCheckTime int64

	// Youtube channel specific fields.
	YoutubeChannelID    string
	YoutubeChannelName  string
	YoutubePostedVideos []string
}

type YoutubeQuota struct {
	Daily     int64
	Left      int64
	ResetTime int64
}
