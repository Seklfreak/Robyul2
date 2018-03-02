package models

import (
	"time"

	"github.com/globalsign/mgo/bson"
)

const (
	UseruploadsDisabledUsersTable MongoDbCollection = "useruploads_disabled_users"
)

type UseruploadsDisabledUsersEntry struct {
	ID             bson.ObjectId `bson:"_id,omitempty"`
	UserID         string
	BannedByUserID string
	BannedAt       time.Time
}
