package models

import (
	"time"
)

const (
	UserConfigTable = "user_config"
)

type UserConfigEntry struct {
	ID          string    `rethink:"id,omitempty"`
	UserID      string    `rethink:"user_id"`
	Key         string    `rethink:"key"`
	Value       []byte    `rethink:"value"`
	LastChanged time.Time `rethink:"last_changed"`
}
