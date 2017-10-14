package models

const (
	PersistencyRolesTable = "persistency_roles"
)

type PersistencyRolesEntry struct {
	ID      string   `rethink:"id,omitempty"`
	GuildID string   `rethink:"guild_id"`
	UserID  string   `rethink:"user_id"`
	Roles   []string `rethink:"roles"`
}
