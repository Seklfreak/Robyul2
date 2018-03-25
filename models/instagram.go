package models

import (
	"github.com/globalsign/mgo/bson"
)

type InstagramSendPostType int
type InstagramPostType int

const (
	InstagramTable MongoDbCollection = "instagram"
)

const (
	InstagramPostTypePost InstagramPostType = iota
	InstagramPostTypeReel
)

const (
	InstagramSendPostTypeRobyulEmbed InstagramSendPostType = iota
	InstagramSendPostTypeDirectLinks
)

type InstagramEntry struct {
	ID              bson.ObjectId `bson:"_id,omitempty"`
	GuildID         string        // TODO: renamed from ServerID
	ChannelID       string
	Username        string
	InstagramUserID int64
	PostedPosts     []InstagramPostEntry
	IsLive          bool
	SendPostType    InstagramSendPostType
}

type InstagramPostEntry struct {
	ID        string
	Type      InstagramPostType
	CreatedAt int64
}
