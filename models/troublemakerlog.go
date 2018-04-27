package models

import (
	"time"

	"github.com/globalsign/mgo/bson"
)

const (
	TroublemakerlogTable MongoDbCollection = "troublemakerlog"
)

type TroublemakerlogEntry struct {
	ID                bson.ObjectId `bson:"_id,omitempty"`
	UserID            string
	Reason            string
	CreatedAt         time.Time
	ReportedByGuildID string
	ReportedByUserID  string
}
