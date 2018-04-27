package models

import "github.com/globalsign/mgo/bson"

const (
	MirrorsTable = "mirrors"
)

type MirrorType int

const (
	MirrorTypeLink MirrorType = iota
	MirrorTypeText
)

type MirrorEntry struct {
	ID                bson.ObjectId `bson:"_id,omitempty"`
	Type              MirrorType
	ConnectedChannels []MirrorChannelEntry
}

type MirrorChannelEntry struct {
	GuildID   string
	ChannelID string
}
