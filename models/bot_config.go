package models

import (
	"time"

	"github.com/globalsign/mgo/bson"
)

const (
	BotConfigTable MongoDbCollection = "bot_config"
)

type BotConfigEntry struct {
	ID          bson.ObjectId `bson:"_id,omitempty"`
	Key         string
	Value       []byte
	LastChanged time.Time
}
