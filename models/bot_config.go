package models

import (
	"time"
)

const (
	BotConfigTable = "bot_config"
)

type BotConfigEntry struct {
	Key         string    `rethink:"id,omitempty"`
	Value       []byte    `rethink:"value"`
	LastChanged time.Time `rethink:"last_changed"`
}
