package models

const (
	EventlogTypeMemberJoin    = "Member_Join"    // EventlogTargetTypeUser
	EventlogTypeMemberLeave   = "Member_Leave"   // EventlogTargetTypeUser
	EventlogTypeChannelCreate = "Channel_Create" // EventlogTargetTypeChannel
	EventlogTypeChannelDelete = "Channel_Delete" // EventlogTargetTypeChannel
	EventlogTypeChannelUpdate = "Channel_Update" // EventlogTargetTypeChannel
	EventlogTypeRoleCreate    = "Role_Create"    // EventlogTargetTypeRole
	EventlogTypeRoleDelete    = "Role_Delete"    // EventlogTargetTypeRole
	EventlogTypeBanAdd        = "Ban_Add"        // EventlogTargetTypeUser
	EventlogTypeBanRemove     = "Ban_Remove"     // EventlogTargetTypeUser
	EventlogTypeEmojiCreate   = "Emoji_Create"   // EventlogTargetTypeEmoji
	EventlogTypeEmojiDelete   = "Emoji_Delete"   // EventlogTargetTypeEmoji
	EventlogTypeEmojiUpdate   = "Emoji_Update"   // EventlogTargetTypeEmoji
	EventlogTypeGuildUpdate   = "Guild_Update"   // EventlogTargetTypeGuild

	EventlogTargetTypeUser    = "user"
	EventlogTargetTypeChannel = "channel"
	EventlogTargetTypeRole    = "role"
	EventlogTargetTypeEmoji   = "emoji"
	EventlogTargetTypeGuild   = "guild"

	AuditLogBackfillTypeChannelCreateRedisSet = "robyul-discord:eventlog:auditlog-backfill:channel-create"
	AuditLogBackfillTypeChannelDeleteRedisSet = "robyul-discord:eventlog:auditlog-backfill:channel-delete"
	AuditLogBackfillTypeChannelUpdateRedisSet = "robyul-discord:eventlog:auditlog-backfill:channel-update"
	AuditLogBackfillTypeRoleCreateRedisSet    = "robyul-discord:eventlog:auditlog-backfill:role-create"
	AuditLogBackfillTypeRoleDeleteRedisSet    = "robyul-discord:eventlog:auditlog-backfill:role-delete"
	AuditLogBackfillTypeBanAddRedisSet        = "robyul-discord:eventlog:auditlog-backfill:ban-add"
	AuditLogBackfillTypeBanRemoveRedisSet     = "robyul-discord:eventlog:auditlog-backfill:ban-remove"
	AuditLogBackfillTypeMemberRemoveRedisSet  = "robyul-discord:eventlog:auditlog-backfill:member-remove"
	AuditLogBackfillTypeEmojiCreateRedisSet   = "robyul-discord:eventlog:auditlog-backfill:emoji-create"
	AuditLogBackfillTypeEmojiDeleteRedisSet   = "robyul-discord:eventlog:auditlog-backfill:emoji-delete"
	AuditLogBackfillTypeEmojiUpdateRedisSet   = "robyul-discord:eventlog:auditlog-backfill:emoji-update"
	AuditLogBackfillTypeGuildUpdateRedisSet   = "robyul-discord:eventlog:auditlog-backfill:guild-update"
)
