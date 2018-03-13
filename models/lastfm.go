package models

import (
	"github.com/globalsign/mgo/bson"
)

const (
	LastFmTable MongoDbCollection = "lastfm"
)

type LastFmEntry struct {
	ID             bson.ObjectId `bson:"_id,omitempty"`
	UserID         string
	LastFmUsername string
}
