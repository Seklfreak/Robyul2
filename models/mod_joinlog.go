package models

import (
	"time"

	"github.com/globalsign/mgo/bson"
)

const (
	ModJoinlogTable MongoDbCollection = "mod_joinlog"
)

type ModJoinlogEntry struct {
	ID                        bson.ObjectId `bson:"_id,omitempty"`
	GuildID                   string
	UserID                    string
	JoinedAt                  time.Time
	InviteCodeUsed            string
	InviteCodeCreatedByUserID string
	InviteCodeCreatedAt       time.Time
	VanityInviteUsedName      string
}
