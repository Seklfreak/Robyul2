package models

import "github.com/globalsign/mgo/bson"

const (
	PersistencyRolesTable MongoDbCollection = "persistency_roles"
)

type PersistencyRolesEntry struct {
	ID      bson.ObjectId `bson:"_id,omitempty"`
	GuildID string
	UserID  string
	Roles   []string
}
