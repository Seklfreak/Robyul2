package models

import (
	"time"

	"github.com/globalsign/mgo/bson"
)

const (
	ProfileBackgroundsTable MongoDbCollection = "profile_backgrounds"
)

type ProfileBackgroundEntry struct {
	ID         bson.ObjectId `bson:"_id,omitempty"`
	Name       string
	URL        string // deprecated
	ObjectName string
	CreatedAt  time.Time
	Tags       []string
}
