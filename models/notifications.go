package models

import (
	"github.com/globalsign/mgo/bson"
)

const (
	NotificationsTable                MongoDbCollection = "notifications"
	NotificationsIgnoredChannelsTable MongoDbCollection = "notifications_ignored_channels"
)

type NotificationsEntry struct {
	ID              bson.ObjectId `bson:"_id,omitempty"`
	Keyword         string
	GuildID         string // can be "global" to affect every guild
	UserID          string
	Triggered       int
	IgnoredGuildIDs []string
}

type NotificationsIgnoredChannelsEntry struct {
	ID        bson.ObjectId `bson:"_id,omitempty"`
	GuildID   string
	ChannelID string
}
