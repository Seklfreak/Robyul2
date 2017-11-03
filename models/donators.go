package models

import "time"

const (
	DonatorsTable = "donators"
)

type DonatorEntry struct {
	ID            string    `gorethink:"id,omitempty"`
	Name          string    `gorethink:"name"`
	HeartOverride string    `gorethink:"heart_override"`
	AddedAt       time.Time `gorethink:"added_at"`
}
