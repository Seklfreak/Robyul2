package models

import (
	"github.com/globalsign/mgo/bson"
)

const (
	FacebookTable MongoDbCollection = "facebook"
)

type FacebookEntry struct {
	ID          bson.ObjectId `bson:"_id,omitempty"`
	GuildID     string
	ChannelID   string
	Username    string
	PostedPosts []FacebookPostEntry
}

type FacebookPostEntry struct {
	ID        string
	CreatedAt string
}
