package models

import (
	"time"

	"github.com/globalsign/mgo/bson"
)

const (
	UserConfigTable MongoDbCollection = "user_config"
)

type UserConfigEntry struct {
	ID          bson.ObjectId `bson:"_id,omitempty"`
	UserID      string
	Key         string
	Value       []byte
	LastChanged time.Time
}
