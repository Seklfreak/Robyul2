package models

const (
	LevelsRolesTable = "levels_roles"
)

type LevelsRoleEntry struct {
	ID         string `rethink:"id,omitempty"`
	GuildID    string `rethink:"guild_id"`
	RoleID     string `rethink:"role_id"`
	StartLevel int    `rethink:"start_level"`
	LastLevel  int    `rethink:"last_level"`
}
