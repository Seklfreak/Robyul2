package models

import "github.com/globalsign/mgo/bson"

const (
	LevelsRolesTable          MongoDbCollection = "levels_roles"
	LevelsRoleOverwritesTable MongoDbCollection = "levels_roles_overwrites"
)

type LevelsRoleEntry struct {
	ID         bson.ObjectId `bson:"_id,omitempty"`
	GuildID    string
	RoleID     string
	StartLevel int
	LastLevel  int
}

type LevelsRoleOverwriteType int

const (
	LevelsRoleOverwriteTypeGrant LevelsRoleOverwriteType = iota
	LevelsRoleOverwriteTypeDeny
)

type LevelsRoleOverwriteEntry struct {
	ID      bson.ObjectId `bson:"_id,omitempty"`
	GuildID string
	RoleID  string
	UserID  string
	Type    LevelsRoleOverwriteType
}
