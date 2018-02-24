package models

import (
	"gopkg.in/mgo.v2/bson"
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
