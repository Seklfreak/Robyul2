package models

import (
	"time"

	"github.com/globalsign/mgo/bson"
)

const (
	AutoleaverWhitelistTable MongoDbCollection = "autoleaver_whitelist"

	AutoleaverLogChannelKey = "autoleaver:log:channel-id"
)

type AutoleaverWhitelistEntry struct {
	ID            bson.ObjectId `bson:"_id,omitempty"`
	AddedAt       time.Time
	GuildID       string
	AddedByUserID string
	Until         time.Time
}
