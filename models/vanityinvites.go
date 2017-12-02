package models

import "time"

const (
	VanityInvitesTable = "vanity_invites"
	// {guildID} resolves to VanityInviteRedisEntry
	VanityInvitesInviteRedisKey = "robyul2-discord:vanityinvites:invite:%s"
)

type VanityInviteEntry struct {
	ID               string    `gorethink:"id,omitempty"`
	GuildID          string    `gorethink:"guild_id"`
	ChannelID        string    `gorethink:"channel_id"`
	VanityName       string    `gorethink:"vanity_name"`
	VanityNamePretty string    `gorethink:"vanity_name_pretty"`
	SetByUserID      string    `gorethink:"set_by_user_id"`
	SetAt            time.Time `gorethink:"set_at"`
}

type VanityInviteRedisEntry struct {
	InviteCode string    `gorethink:"invitecode"`
	ExpiresAt  time.Time `gorethink:"expiresat"`
}
