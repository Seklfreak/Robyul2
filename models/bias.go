package models

import (
	"github.com/globalsign/mgo/bson"
)

const (
	BiasTable MongoDbCollection = "bias"
)

type BiasEntry struct {
	ID         bson.ObjectId `bson:"_id,omitempty"`
	GuildID    string        // TODO: renamed from serverID
	ChannelID  string
	Categories []BiasEntryCategory
}

type BiasEntryCategory struct {
	Label   string
	Message string
	Pool    string
	Hidden  bool
	Limit   int
	Roles   []BiasEntryRole
}

type BiasEntryRole struct {
	Name      string
	Print     string
	Aliases   []string
	Reactions []string
}
