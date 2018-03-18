package models

import (
	"github.com/globalsign/mgo/bson"
)

const (
	ModulePermissionsTable MongoDbCollection = "module_permissions"
)

type ModulePermissionsModule int64

/*
func (m *ModulePermissionsModule) SetInt(n int64) {
	*m = ModulePermissionsModule(strconv.FormatInt(n, 10))
}

func (m *ModulePermissionsModule) GetInt() int64 {
	n, _ := strconv.ParseInt(string(*m), 10, 64)
	return n
}
*/

type ModulePermissionEntry struct {
	ID       bson.ObjectId `bson:"_id,omitempty"`
	GuildID  string
	Type     string // "channel" or "role"
	TargetID string
	Allowed  ModulePermissionsModule // -1 for unset
	Denied   ModulePermissionsModule // -1 for unset
}

func GetDefaultModulePermission() (defaultEntry ModulePermissionEntry) {
	defaultEntry.Allowed = -1 // TODO
	defaultEntry.Denied = -1
	return defaultEntry
}
