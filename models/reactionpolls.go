package models

import (
	"time"

	"github.com/globalsign/mgo/bson"
)

const (
	ReactionpollsTable MongoDbCollection = "reactionpolls"
)

type ReactionpollsEntry struct {
	ID              bson.ObjectId `bson:"_id,omitempty"`
	Text            string
	MessageID       string
	ChannelID       string
	GuildID         string
	CreatedByUserID string
	CreatedAt       time.Time
	Active          bool
	AllowedEmotes   []string
	MaxAllowedVotes int
	Reactions       map[string][]string // [emoji][]userIDs
	Initialised     bool
}
