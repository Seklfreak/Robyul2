package models

import (
	"time"

	"github.com/globalsign/mgo/bson"
)

const (
	CustomCommandsTable MongoDbCollection = "customcommands"
)

type CustomCommandsEntry struct {
	ID              bson.ObjectId `bson:"_id,omitempty"`
	GuildID         string
	CreatedByUserID string
	CreatedAt       time.Time
	Triggered       int
	Keyword         string
	Content         string
}
