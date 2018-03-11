package models

import (
	"time"

	"strconv"

	"github.com/globalsign/mgo/bson"
)

const (
	CustomCommandsTable MongoDbCollection = "customcommands"
)

type CustomCommandsEntry struct {
	ID                bson.ObjectId `bson:"_id,omitempty"`
	GuildID           string
	CreatedByUserID   string
	CreatedAt         time.Time
	Triggered         int
	Keyword           string
	Content           string
	StorageObjectName string
	StorageMimeType   string
	StorageHash       string
	StorageFilename   string
}

func CustomCommandsNewObjectName(guildID, userID string) (objectName string) {
	return "robyul-customcommands-" + guildID + "-" + userID + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
}
