package models

import (
	"github.com/globalsign/mgo/bson"
)

const (
	OldBiasGameTable MongoDbCollection = "biasgame"
	BiasGameTable    MongoDbCollection = "biasgame_new"
)

type OldBiasGameEntry struct {
	ID           bson.ObjectId `bson:"_id,omitempty"`
	UserID       string
	GuildID      string
	GameWinner   OldIdolEntry
	RoundWinners []OldIdolEntry
	RoundLosers  []OldIdolEntry
	Gender       string // girl, boy, mixed
	GameType     string // single, multi
}

type BiasGameEntry struct {
	ID           bson.ObjectId `bson:"_id,omitempty"`
	UserID       string
	GuildID      string
	GameWinner   bson.ObjectId
	RoundWinners []bson.ObjectId
	RoundLosers  []bson.ObjectId
	Gender       string // girl, boy, mixed
	GameType     string // single, multi
}
