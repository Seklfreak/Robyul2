package models

import (
	"github.com/globalsign/mgo/bson"
)

const (
	NotificationsTable MongoDbCollection = "notifications"
)

type NotificationsEntry struct {
	ID        bson.ObjectId `bson:"_id,omitempty"`
	Keyword   string
	GuildID   string // can be "global" to affect every guild
	UserID    string
	Triggered int
}
