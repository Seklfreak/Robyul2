package models

import (
	"time"

	"github.com/globalsign/mgo/bson"
)

const (
	StarboardEntriesTable MongoDbCollection = "starboard_entries"
)

type StarboardEntry struct {
	ID                        bson.ObjectId `bson:"_id,omitempty"`
	GuildID                   string
	MessageID                 string
	ChannelID                 string
	AuthorID                  string
	MessageContent            string
	MessageAttachmentURLs     []string
	MessageEmbedImageURL      string
	StarboardMessageID        string
	StarboardMessageChannelID string
	StarUserIDs               []string
	Stars                     int
	FirstStarred              time.Time
}
