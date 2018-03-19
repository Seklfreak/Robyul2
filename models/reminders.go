package models

import "github.com/globalsign/mgo/bson"

const (
	RemindersTable MongoDbCollection = "reminders"
)

type RemindersEntry struct {
	ID        bson.ObjectId `bson:"_id,omitempty"`
	UserID    string
	Reminders []RemindersReminderEntry
}

type RemindersReminderEntry struct {
	Message   string
	ChannelID string
	GuildID   string
	Timestamp int64
}
