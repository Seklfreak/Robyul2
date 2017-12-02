package models

import "time"

const (
	ElasticIndex                 = "robyul"
	ElasticTypeMessage           = "message"
	ElasticTypeJoin              = "join"
	ElasticTypeLeave             = "leave"
	ElasticTypeReaction          = "reaction"
	ElasticTypePresenceUpdate    = "presence_update"
	ElasticTypeVanityInviteClick = "vanity_invite_click"
)

type ElasticMessage struct {
	CreatedAt     time.Time
	MessageID     string
	Content       string
	ContentLength int
	Attachments   []string
	UserID        string
	GuildID       string
	ChannelID     string
	Embeds        int
}

type ElasticJoin struct {
	CreatedAt      time.Time
	GuildID        string
	UserID         string
	UsedInviteCode string
	VanityInvite   string
}

type ElasticLeave struct {
	CreatedAt time.Time
	GuildID   string
	UserID    string
}

type ElasticReaction struct {
	CreatedAt time.Time
	UserID    string
	MessageID string
	ChannelID string
	GuildID   string
	EmojiID   string
	EmojiName string
}

type ElasticPresenceUpdate struct {
	CreatedAt  time.Time
	UserID     string
	GameType   int
	GameTypeV2 string
	GameName   string
	GameURL    string
	Status     string
}

type ElasticVanityInviteClick struct {
	CreatedAt        time.Time
	VanityInviteName string
	GuildID          string
}
