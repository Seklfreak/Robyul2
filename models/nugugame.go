package models

import "github.com/globalsign/mgo/bson"

const (
	NuguGameTable MongoDbCollection = "nugugame"
)

type NuguGameEntry struct {
	ID                  bson.ObjectId `bson:"_id,omitempty"`
	UserID              string        // person who start the game
	GuildID             string
	CorrectIdols        []bson.ObjectId
	IncorrectIdols      []bson.ObjectId
	Gender              string // girl, boy, mixed
	GameType            string // idol, group
	IsMultigame         bool
	Difficulty          string
	UsersCorrectGuesses map[string][]bson.ObjectId // userid => []ids of idols they got right.  used in multi only
}
