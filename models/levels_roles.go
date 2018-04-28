package models

import "github.com/globalsign/mgo/bson"

const (
	LevelsRolesTable          MongoDbCollection = "levels_roles"
	LevelsRoleOverwritesTable                   = "levels_roles_overwrites"
)

type LevelsRoleEntry struct {
	ID         bson.ObjectId `bson:"_id,omitempty"`
	GuildID    string
	RoleID     string
	StartLevel int
	LastLevel  int
}

type LevelsRoleOverwriteEntry struct {
	ID      string `rethink:"id,omitempty"`
	GuildID string `rethink:"guild_id"`
	RoleID  string `rethink:"role_id"`
	UserID  string `rethink:"user_id"`
	Type    string `rethink:"type"` // "grant" or "deny"
}
