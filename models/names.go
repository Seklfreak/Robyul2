package models

import (
	"time"

	"github.com/globalsign/mgo/bson"
)

const (
	NamesTable MongoDbCollection = "names"
)

type NamesEntry struct {
	ID        bson.ObjectId `bson:"_id,omitempty"`
	ChangedAt time.Time
	GuildID   string
	UserID    string
	Nickname  string
	Username  string
}
