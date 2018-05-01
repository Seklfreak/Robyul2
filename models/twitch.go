package models

import "github.com/globalsign/mgo/bson"

const (
	TwitchTable MongoDbCollection = "twitch"
)

type TwitchEntry struct {
	ID                bson.ObjectId `bson:"_id,omitempty"`
	GuildID           string        // renamed from serverid
	ChannelID         string
	TwitchChannelName string
	IsLive            bool
	MentionRoleID     string
}
