package models

import "time"

const (
	AutoleaverWhitelistTable = "autoleaver_whitelist"
)

type AutoleaverWhitelistEntry struct {
	ID            string    `rethink:"id,omitempty"`
	AddedAt       time.Time `rethink:"added_at"`
	GuildID       string    `rethink:"guild_id"`
	AddedByUserID string    `rethink:"added_by_user_id"`
}
