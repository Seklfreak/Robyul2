package models

import (
	"github.com/globalsign/mgo/bson"
)

const (
	GalleryTable MongoDbCollection = "galleries"
)

type GalleryEntry struct {
	ID              bson.ObjectId `bson:"_id,omitempty"`
	SourceChannelID string
	TargetChannelID string
	GuildID         string
	AddedByUserID   string
}
