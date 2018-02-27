package models

import (
	"github.com/globalsign/mgo/bson"
)

const (
	LevelsServerusersTable MongoDbCollection = "levels_serverusers"
)

type LevelsServerusersEntry struct {
	ID      bson.ObjectId `bson:"_id,omitempty"`
	UserID  string
	GuildID string
	Exp     int64
}
