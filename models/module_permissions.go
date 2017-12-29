package models

const (
	ModulePermissionsTable = "module_permissions"
)

type ModulePermissionsModule int

type ModulePermission struct {
	ID       string                  `rethink:"id,omitempty"`
	GuildID  string                  `rethink:"guild_id"`
	Type     string                  `rethink:"type"` // "channel" or "role"
	TargetID string                  `rethink:"target_id"`
	Allowed  ModulePermissionsModule `rethink:"allowed"` // -1 for unset
	Denied   ModulePermissionsModule `rethink:"denied"`  // -1 for unset
}

func GetDefaultModulePermission() (defaultEntry ModulePermission) {
	defaultEntry.Allowed = -1
	defaultEntry.Denied = -1
	return defaultEntry
}
