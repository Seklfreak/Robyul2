package models

import (
	"time"

	"github.com/globalsign/mgo/bson"
)

const (
	VanityInvitesTable MongoDbCollection = "vanity_invites"
	// {guildID} resolves to VanityInviteRedisEntry
	VanityInvitesInviteRedisKey = "robyul2-discord:vanityinvites:invite:%s"

	VanityInviteLogChannelKey = "vanityinvite:log:channel-id"
)

type VanityInviteEntry struct {
	ID               bson.ObjectId `bson:"_id,omitempty"`
	GuildID          string
	ChannelID        string
	VanityName       string
	VanityNamePretty string
	SetByUserID      string
	SetAt            time.Time
}

type VanityInviteRedisEntry struct {
	InviteCode string    `gorethink:"invitecode"`
	ExpiresAt  time.Time `gorethink:"expiresat"`
}
