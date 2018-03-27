package models

import (
	"time"

	"fmt"

	"github.com/globalsign/mgo/bson"
)

const (
	ProfileBadgesTable MongoDbCollection = "profile_badges"
)

type ProfileBadgeEntry struct {
	ID               bson.ObjectId `bson:"_id,omitempty"`
	OldID            string        // deprecated
	CreatedByUserID  string
	Name             string
	Category         string
	BorderColor      string
	GuildID          string
	CreatedAt        time.Time
	URL              string // deprecated
	ObjectName       string
	LevelRequirement int
	RoleRequirement  string
	AllowedUserIDs   []string
	DeniedUserIDs    []string
}

func (e ProfileBadgeEntry) GetID() (ID string) {
	if e.OldID != "" {
		return e.OldID
	} else {
		return fmt.Sprintf(`%x`, string(e.ID))
	}
}
