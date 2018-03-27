package models

import (
	"time"

	"github.com/globalsign/mgo/bson"
)

const (
	BiasGameTable            MongoDbCollection = "biasgame"
	BiasGameSuggestionsTable MongoDbCollection = "biasgame_suggestions"
)

type BiasEntry struct {
	Name      string
	GroupName string
	Gender    string
}

type BiasGameEntry struct {
	ID           bson.ObjectId `bson:"_id,omitempty"`
	UserID       string
	GuildID      string
	GameWinner   BiasEntry
	RoundWinners []BiasEntry
	RoundLosers  []BiasEntry
	Gender       string // girl, boy, mixed
	GameType     string // single, multi
}

type BiasGameSuggestionEntry struct {
	ID                bson.ObjectId `bson:"_id,omitempty"`
	UserID            string        // user who made the message
	ProcessedByUserId string
	Name              string
	GrouopName        string
	Gender            string
	ImageURL          string
	ChannelID         string // channel suggestion was made in
	Status            string
	Notes             string // misc notes from
	GroupMatch        bool
	IdolMatch         bool
	LastModifiedOn    time.Time
	ImageHashString   string
	ObjectName        string
}
