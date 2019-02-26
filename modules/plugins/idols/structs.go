package idols

import (
	"github.com/globalsign/mgo/bson"
)

type IdolImage struct {
	ImageBytes []byte
	HashString string
	ObjectName string
}

type Idol struct {
	ID           bson.ObjectId
	Name         string
	NameAliases  []string
	GroupName    string
	Gender       string
	NameAndGroup string
	Images       []IdolImage
	Deleted      bool
	BGGames      int
	BGGameWins   int
	BGRounds     int
	BGRoundWins  int
}
