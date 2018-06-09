package models

import (
	"github.com/globalsign/mgo/bson"
)

const (
	BiasGameTable MongoDbCollection = "biasgame"
)

type BiasGameEntry struct {
	ID           bson.ObjectId `bson:"_id,omitempty"`
	UserID       string
	GuildID      string
	GameWinner   IdolEntry
	RoundWinners []IdolEntry
	RoundLosers  []IdolEntry
	Gender       string // girl, boy, mixed
	GameType     string // single, multi
}
