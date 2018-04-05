package models

import "github.com/globalsign/mgo/bson"

const (
	GreeterTable MongoDbCollection = "greeter"

	GreeterTypeJoin GreeterType = iota
	GreeterTypeLeave
	GreeterTypeBan
)

type GreeterType int

type GreeterEntry struct {
	Id        bson.ObjectId `bson:"_id,omitempty"`
	GuildID   string
	ChannelID string
	EmbedCode string
	Type      GreeterType
}
