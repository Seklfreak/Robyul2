package models

import "time"

const (
	DogLinksTable = "dog_links"
)

type DogLinkEntry struct {
	ID            string    `gorethink:"id,omitempty"`
	URL           string    `gorethink:"url"`
	AddedByUserID string    `gorethink:"added_by_userid"`
	AddedAt       time.Time `gorethink:"added_at"`
}
