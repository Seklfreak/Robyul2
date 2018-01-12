package models

const (
	EventlogTypeMemberJoin    = "Member_Join"    // EventlogTargetTypeUser
	EventlogTypeMemberLeave   = "Member_Leave"   // EventlogTargetTypeUser
	EventlogTypeChannelCreate = "Channel_Create" // EventlogTargetTypeChannel
	EventlogTypeChannelDelete = "Channel_Delete" // EventlogTargetTypeChannel

	EventlogTargetTypeUser    = "user"
	EventlogTargetTypeChannel = "channel"

	AuditLogBackfillTypeChannelCreateRedisSet = "robyul-discord:eventlog:auditlog-backfill:channel-create"
	AuditLogBackfillTypeChannelDeleteRedisSet = "robyul-discord:eventlog:auditlog-backfill:channel-delete"
)
