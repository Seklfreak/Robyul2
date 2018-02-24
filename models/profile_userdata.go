package models

import (
	"time"

	"gopkg.in/mgo.v2/bson"
)

const (
	ProfileUserdataTable MongoDbCollection = "profile_userdata"
)

type ProfileUserdataEntry struct {
	ID                bson.ObjectId `bson:"_id,omitempty"`
	UserID            string
	Background        string
	Title             string
	Bio               string
	Rep               int
	LastRepped        time.Time
	ActiveBadgeIDs    []string
	BackgroundColor   string
	AccentColor       string
	TextColor         string
	BackgroundOpacity string
	DetailOpacity     string
	Timezone          string
	Birthday          string
}
