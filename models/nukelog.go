package models

import (
	"time"

	"github.com/globalsign/mgo/bson"
)

const (
	NukelogTable MongoDbCollection = "nukelog"
)

type NukelogEntry struct {
	ID       bson.ObjectId `bson:"_id,omitempty"`
	UserID   string
	UserName string
	NukerID  string
	NukedAt  time.Time
}
