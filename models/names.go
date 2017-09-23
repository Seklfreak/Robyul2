package models

import "time"

const (
	NamesTable = "names"
)

type NamesEntry struct {
	ID        string    `rethink:"id,omitempty"`
	ChangedAt time.Time `rethink:"changed_at"`
	GuildID   string    `rethink:"guild_id"`
	UserID    string    `rethink:"user_id"`
	Nickname  string    `rethink:"nickname"`
	Username  string    `rethink:"username"`
}
