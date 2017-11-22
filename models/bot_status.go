package models

import "time"

const (
	BotStatusTable = "bot_status"
)

type BotStatus struct {
	ID            string    `rethink:"id,omitempty"`
	AddedByUserID string    `rethink:"added_by_userid"`
	AddedAt       time.Time `rethink:"added_at"`
	Text          string    `rethink:"text"`
}
