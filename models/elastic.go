package models

import "time"

const (
	ElasticIndexMessages           = "robyul-messages"
	ElasticIndexJoins              = "robyul-joins"
	ElasticIndexLeaves             = "robyul-leaves"
	ElasticIndexPresenceUpdates    = "robyul-presence_updates"
	ElasticIndexVanityInviteClicks = "robyul-vanity_invite_clicks"
	ElasticIndexVoiceSessions      = "robyul-voice_session"
	ElasticIndexEventlogs          = "robyul-eventlogs"
)

type ElasticLegacyMessage struct {
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

type ElasticMessage struct {
	CreatedAt     time.Time
	MessageID     string
	Content       []string
	ContentLength int
	Attachments   []string
	UserID        string
	GuildID       string
	ChannelID     string
	Embeds        int
	Deleted       bool
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
	Referer          string
}

type ElasticVoiceSession struct {
	CreatedAt       time.Time
	GuildID         string
	ChannelID       string
	UserID          string
	JoinTime        time.Time
	LeaveTime       time.Time
	DurationSeconds int64
}

type ElasticEventlog struct {
	CreatedAt  time.Time
	GuildID    string
	TargetID   string
	TargetType string
	UserID     string
	ActionType string
	Reason     string
	Changes    []ElasticEventlogChange
	Options    []ElasticEventlogOption
	WaitingFor struct {
		AuditLogBackfill bool
	}
	EventlogMessages []string
}

type ElasticEventlogChange struct {
	Key      string
	OldValue string
	NewValue string
	Type     string
}

type ElasticEventlogOption struct {
	Key   string
	Value string
	Type  string
}
