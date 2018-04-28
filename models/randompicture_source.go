package models

import "github.com/globalsign/mgo/bson"

const (
	RandompictureSourcesTable MongoDbCollection = "randompicture_sources"
)

type RandompictureSourceEntry struct {
	ID                 bson.ObjectId `bson:"_id,omitempty"`
	PreviousID         string
	GuildID            string
	PostToChannelIDs   []string
	DriveFolderIDs     []string
	Aliases            []string
	BlacklistedRoleIDs []string
}
