package models

import (
	"time"

	"github.com/globalsign/mgo/bson"
)

const (
	DonatorsTable MongoDbCollection = "donators"
)

type DonatorEntry struct {
	ID            bson.ObjectId `bson:"_id,omitempty"`
	Name          string
	HeartOverride string
	AddedAt       time.Time
}
