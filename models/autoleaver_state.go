package models

import (
	"github.com/globalsign/mgo/bson"
)

const (
	AutoleaverStateTable MongoDbCollection = "autoleaver_state"
)

type AutoleaverStateEntry struct {
	ID      bson.ObjectId `bson:"_id,omitempty"`
	GuildID string
}
