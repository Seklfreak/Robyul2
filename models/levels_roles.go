package models

const (
	LevelsRolesTable          = "levels_roles"
	LevelsRoleOverwritesTable = "levels_roles_overwrites"
)

type LevelsRoleEntry struct {
	ID         string `rethink:"id,omitempty"`
	GuildID    string `rethink:"guild_id"`
	RoleID     string `rethink:"role_id"`
	StartLevel int    `rethink:"start_level"`
	LastLevel  int    `rethink:"last_level"`
}

type LevelsRoleOverwriteEntry struct {
	ID      string `rethink:"id,omitempty"`
	GuildID string `rethink:"guild_id"`
	RoleID  string `rethink:"role_id"`
	UserID  string `rethink:"user_id"`
	Type    string `rethink:"type"` // "grant" or "deny"
}
