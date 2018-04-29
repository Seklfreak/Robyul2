package models

import (
	"github.com/globalsign/mgo/bson"
)

const (
	ModulePermissionsTable MongoDbCollection = "module_permissions"
)

type ModulePermissionsModule string

type ModulePermissionEntry struct {
	ID             bson.ObjectId `bson:"_id,omitempty"`
	GuildID        string
	Type           string // "channel" or "role"
	TargetID       string
	AllowedModules []ModulePermissionsModule // nil for unset
	DeniedModules  []ModulePermissionsModule // nil for unset
}

func GetDefaultModulePermission() (defaultEntry ModulePermissionEntry) {
	defaultEntry.AllowedModules = nil
	defaultEntry.DeniedModules = nil
	return defaultEntry
}
