package models

import (
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/globalsign/mgo/bson"
)

const (
	BotStatusTable MongoDbCollection = "bot_status"
)

type BotStatusEntry struct {
	ID            bson.ObjectId `bson:"_id,omitempty"`
	AddedByUserID string
	AddedAt       time.Time
	Text          string
	Type          discordgo.GameType
}
