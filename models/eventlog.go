package models

const (
	EventlogTypeMemberJoin    = "Member_Join"    // EventlogTargetTypeUser
	EventlogTypeMemberLeave   = "Member_Leave"   // EventlogTargetTypeUser
	EventlogTypeChannelCreate = "Channel_Create" // EventlogTargetTypeChannel
	EventlogTypeChannelDelete = "Channel_Delete" // EventlogTargetTypeChannel
	EventlogTypeRoleCreate    = "Role_Create"    // EventlogTargetTypeRole
	EventlogTypeRoleDelete    = "Role_Delete"    // EventlogTargetTypeRole

	EventlogTargetTypeUser    = "user"
	EventlogTargetTypeChannel = "channel"
	EventlogTargetTypeRole    = "role"

	AuditLogBackfillTypeChannelCreateRedisSet = "robyul-discord:eventlog:auditlog-backfill:channel-create"
	AuditLogBackfillTypeChannelDeleteRedisSet = "robyul-discord:eventlog:auditlog-backfill:channel-delete"
	AuditLogBackfillTypeRoleCreateRedisSet    = "robyul-discord:eventlog:auditlog-backfill:role-create"
	AuditLogBackfillTypeRoleDeleteRedisSet    = "robyul-discord:eventlog:auditlog-backfill:role-delete"
)
