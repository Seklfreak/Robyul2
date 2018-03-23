package models

import (
	"time"

	"github.com/globalsign/mgo/bson"
)

const (
	DogLinksTable MongoDbCollection = "dog_links"
)

type DogLinkEntry struct {
	ID            bson.ObjectId `bson:"_id,omitempty"`
	URL           string        // deprecated
	ObjectName    string
	AddedByUserID string
	AddedAt       time.Time
}
