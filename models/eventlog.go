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
	EventlogTypeMemberUpdate  = "Member_Update"  // EventlogTargetTypeUser
	EventlogTypeRoleUpdate    = "Role_Update"    // EventlogTargetTypeRole

	EventlogTypeInvitePosted = "Invite_Posted" // EvenlogTargetTypeGuild

	EventlogTargetTypeUser                             = "user"
	EventlogTargetTypeChannel                          = "channel"
	EventlogTargetTypeRole                             = "role"
	EventlogTargetTypeEmoji                            = "emoji"
	EventlogTargetTypeGuild                            = "guild"
	EventlogTargetTypeMessage                          = "message"
	EventlogTargetTypeInviteCode                       = "invite_code"
	EventlogTargetTypePermissionOverwrite              = "permission_overwrite"
	EventlogTargetTypeRolePermissions                  = "role_permissions"
	EventlogTargetTypeVerificationLevel                = "verification_level"
	EventlogTargetTypeGuildDefaultMessageNotifications = "guild_default_message_notifications"

	EventlogTypeRobyulBadgeCreate                   = "Robyul_Badge_Create"                    // EventlogTargetTypeRobyulBadge
	EventlogTypeRobyulBadgeDelete                   = "Robyul_Badge_Delete"                    // EventlogTargetTypeRobyulBadge
	EventlogTypeRobyulBadgeAllow                    = "Robyul_Badge_Allow"                     // EventlogTargetTypeRobyulBadge
	EventlogTypeRobyulBadgeDeny                     = "Robyul_Badge_Deny"                      // EventlogTargetTypeRobyulBadge
	EventlogTypeRobyulLevelsReset                   = "Robyul_Levels_Reset"                    // EventlogTargetTypeUser
	EventlogTypeRobyulLevelsIgnoreUser              = "Robyul_Levels_Ignore_User"              // EventlogTargetTypeUser
	EventlogTypeRobyulLevelsIgnoreChannel           = "Robyul_Levels_Ignore_Channel"           // EventlogTargetTypeChannel
	EventlogTypeRobyulLevelsProcessedHistory        = "Robyul_Levels_ProcessedHistory"         // EventlogTargetTypeGuild
	EventlogTypeRobyulLevelsRoleAdd                 = "Robyul_Levels_Role_Add"                 // EventlogTargetTypeRole
	EventlogTypeRobyulLevelsRoleApply               = "Robyul_Levels_Role_Apply"               // EventlogTargetTypeGuild
	EventlogTypeRobyulLevelsRoleDelete              = "Robyul_Levels_Role_Delete"              // EventlogTargetTypeRole
	EventlogTypeRobyulLevelsRoleGrant               = "Robyul_Levels_Role_Grant"               // EventlogTargetTypeUser
	EventlogTypeRobyulLevelsRoleDeny                = "Robyul_Levels_Role_Deny"                // EventlogTargetTypeUser
	EventlogTypeRobyulNotificationsChannelIgnore    = "Robyul_Notifications_Channel_Ignore"    // EventlogTargetTypeChannel
	EventlogTypeRobyulVliveFeedAdd                  = "Robyul_Vlive_Feed_Add"                  // EventlogTargetTypeRobyulVliveFeed
	EventlogTypeRobyulVliveFeedRemove               = "Robyul_Vlive_Feed_Remove"               // EventlogTargetTypeRobyulVliveFeed
	EventlogTypeRobyulYouTubeChannelFeedAdd         = "Robyul_YouTube_Channel_Feed_Add"        // EventlogTargetTypeRobyulYouTubeChannelFeed
	EventlogTypeRobyulYouTubeChannelFeedRemove      = "Robyul_YouTube_Channel_Feed_Remove"     // EventlogTargetTypeRobyulYouTubeChannelFeed
	EventlogTypeRobyulInstagramFeedAdd              = "Robyul_Instagram_Feed_Add"              // EventlogTargetTypeRobyulInstagramFeed
	EventlogTypeRobyulInstagramFeedRemove           = "Robyul_Instagram_Feed_Remove"           // EventlogTargetTypeRobyulInstagramFeed
	EventlogTypeRobyulInstagramFeedUpdate           = "Robyul_Instagram_Feed_Update"           // EventlogTargetTypeRobyulInstagramFeed
	EventlogTypeRobyulRedditFeedAdd                 = "Robyul_Reddit_Feed_Add"                 // EventlogTargetTypeRobyulRedditFeed
	EventlogTypeRobyulRedditFeedRemove              = "Robyul_Reddit_Feed_Remove"              // EventlogTargetTypeRobyulRedditFeed
	EventlogTypeRobyulRedditFeedUpdate              = "Robyul_Reddit_Feed_Update"              // EventlogTargetTypeRobyulRedditFeed
	EventlogTypeRobyulFacebookFeedAdd               = "Robyul_Facebook_Feed_Add"               // EventlogTargetTypeRobyulFacebookFeed
	EventlogTypeRobyulFacebookFeedRemove            = "Robyul_Facebook_Feed_Remove"            // EventlogTargetTypeRobyulFacebookFeed
	EventlogTypeRobyulCleanup                       = "Robyul_Cleanup"                         //
	EventlogTypeRobyulMute                          = "Robyul_Mute"                            // EventlogTargetTypeUser
	EventlogTypeRobyulUnmute                        = "Robyul_Unmute"                          // EventlogTargetTypeUser
	EventlogTypeRobyulPostCreate                    = "Robyul_Post_Create"                     // EventlogTargetTypeMessage
	EventlogTypeRobyulPostUpdate                    = "Robyul_Post_Update"                     // EventlogTargetTypeMessage
	EventlogTypeRobyulBatchRolesCreate              = "Robyul_BatchRoles_Create"               // EventlogTargetTypeGuild
	EventlogTypeRobyulAutoInspectsChannel           = "Robyul_AutoInspectsChannel"             // EventlogTargetTypeChannel
	EventlogTypeRobyulPrefixUpdate                  = "Robyul_Prefix_Update"                   // EventlogTargetTypeGuild
	EventlogTypeRobyulChatlogUpdate                 = "Robyul_Chatlog_Update"                  // EventlogTargetTypeGuild
	EventlogTypeRobyulVanityInviteCreate            = "Robyul_VanityInvite_Create"             // EventlogTargetTypeGuild
	EventlogTypeRobyulVanityInviteDelete            = "Robyul_VanityInvite_Delete"             // EventlogTargetTypeGuild
	EventlogTypeRobyulVanityInviteUpdate            = "Robyul_VanityInvite_Update"             // EventlogTargetTypeGuild
	EventlogTypeRobyulBiasConfigCreate              = "Robyul_Bias_Config_Create"              // EventlogTargetTypeChannel
	EventlogTypeRobyulBiasConfigDelete              = "Robyul_Bias_Config_Delete"              // EventlogTargetTypeChannel
	EventlogTypeRobyulBiasConfigUpdate              = "Robyul_Bias_Config_Update"              // EventlogTargetTypeChannel
	EventlogTypeRobyulAutoroleAdd                   = "Robyul_Autorole_Add"                    // EventlogTargetTypeRole
	EventlogTypeRobyulAutoroleRemove                = "Robyul_Autorole_Remove"                 // EventlogTargetTypeRole
	EventlogTypeRobyulAutoroleApply                 = "Robyul_Autorole_Apply"                  // EventlogTargetTypeRole
	EventlogTypeRobyulGuildAnnouncementsJoinSet     = "Robyul_GuildAnnouncements_Join_Set"     // EventlogTargetTypeChannel
	EventlogTypeRobyulGuildAnnouncementsJoinRemove  = "Robyul_GuildAnnouncements_Join_Remove"  // EventlogTargetTypeChannel
	EventlogTypeRobyulGuildAnnouncementsLeaveSet    = "Robyul_GuildAnnouncements_Leave_Set"    // EventlogTargetTypeChannel
	EventlogTypeRobyulGuildAnnouncementsLeaveRemove = "Robyul_GuildAnnouncements_Leave_Remove" // EventlogTargetTypeChannel
	EventlogTypeRobyulGuildAnnouncementsBanSet      = "Robyul_GuildAnnouncements_Ban_Set"      // EventlogTargetTypeChannel
	EventlogTypeRobyulGalleryAdd                    = "Robyul_Gallery_Add"                     // EventlogTargetTypeRobyulGallery
	EventlogTypeRobyulGalleryRemove                 = "Robyul_Gallery_Remove"                  // EventlogTargetTypeRobyulGallery
	EventlogTypeRobyulMirrorCreate                  = "Robyul_Mirror_Create"                   // EventlogTargetTypeRobyulMirror
	EventlogTypeRobyulMirrorDelete                  = "Robyul_Mirror_Delete"                   // EventlogTargetTypeRobyulMirror
	EventlogTypeRobyulMirrorUpdate                  = "Robyul_Mirror_Update"                   // EventlogTargetTypeRobyulMirror
	EventlogTypeRobyulStarboardCreate               = "Robyul_Starboard_Create"                // EventlogTargetTypeChannel
	EventlogTypeRobyulStarboardDelete               = "Robyul_Starboard_Delete"                // EventlogTargetTypeChannel
	EventlogTypeRobyulStarboardUpdate               = "Robyul_Starboard_Update"                // EventlogTargetTypeChannel
	EventlogTypeRobyulRandomPictureSourceCreate     = "Robyul_RandomPicture_Source_Create"     // EventlogTargetTypeRobyulRandomPictureSource
	EventlogTypeRobyulRandomPictureConfigUpdate     = "Robyul_RandomPicture_Config_Update"     // EventlogTargetTypeRobyulRandomPictureSource
	EventlogTypeRobyulRandomPictureSourceRemove     = "Robyul_RandomPicture_Source_Remove"     // EventlogTargetTypeRobyulRandomPictureSource
	EventlogTypeRobyulCommandsAdd                   = "Robyul_Commands_Add"                    // EventlogTargetTypeGuild
	EventlogTypeRobyulCommandsDelete                = "Robyul_Commands_Delete"                 // EventlogTargetTypeGuild
	EventlogTypeRobyulCommandsUpdate                = "Robyul_Commands_Update"                 // EventlogTargetTypeGuild
	EventlogTypeRobyulCommandsJsonExport            = "Robyul_Commands_Json_Export"            // EventlogTargetTypeGuild
	EventlogTypeRobyulCommandsJsonImport            = "Robyul_Commands_Json_Import"            // EventlogTargetTypeGuild
	EventlogTypeRobyulTwitchFeedAdd                 = "Robyul_Twitch_Feed_Add"                 // EventlogTargetTypeRobyulTwitchFeed
	EventlogTypeRobyulTwitchFeedRemove              = "Robyul_Twitch_Feed_Remove"              // EventlogTargetTypeRobyulTwitchFeed
	EventlogTypeRobyulNukeParticipate               = "Robyul_Nuke_Participate"                // EventlogTargetTypeGuild
	EventlogTypeRobyulTroublemakerParticipate       = "Robyul_Troublemaker_Participate"        // EventlogTargetTypeGuild
	EventlogTypeRobyulTroublemakerReport            = "Robyul_Troublemaker_Report"             // EventlogTargetTypeUser
	EventlogTypeRobyulPersistencyBiasRoles          = "Robyul_Persistency_BiasRoles"           // EventlogTargetTypeGuild
	EventlogTypeRobyulPersistencyRoleAdd            = "Robyul_Persistency_Role_Add"            // EventlogTargetTypeRole
	EventlogTypeRobyulPersistencyRoleRemove         = "Robyul_Persistency_Role_Remove"         // EventlogTargetTypeRole
	EventlogTypeRobyulModuleAllowRoleAdd            = "Robyul_Module_Allow_Role_Add"           // EventlogTargetTypeRole
	EventlogTypeRobyulModuleAllowRoleRemove         = "Robyul_Module_Allow_Role_Remove"        // EventlogTargetTypeRole
	EventlogTypeRobyulModuleAllowChannelAdd         = "Robyul_Module_Allow_Channel_Add"        // EventlogTargetTypeChannel
	EventlogTypeRobyulModuleAllowChannelRemove      = "Robyul_Module_Allow_Channel_Remove"     // EventlogTargetTypeChannel
	EventlogTypeRobyulModuleDenyRoleAdd             = "Robyul_Module_Deny_Role_Add"            // EventlogTargetTypeRole
	EventlogTypeRobyulModuleDenyRoleRemove          = "Robyul_Module_Deny_Role_Remove"         // EventlogTargetTypeRole
	EventlogTypeRobyulModuleDenyChannelAdd          = "Robyul_Module_Deny_Channel_Add"         // EventlogTargetTypeChannel
	EventlogTypeRobyulModuleDenyChannelRemove       = "Robyul_Module_Deny_Channel_Remove"      // EventlogTargetTypeChannel
	EventlogTypeRobyulEventlogConfigUpdate          = "Robyul_Module_Eventlog_Config_Update"   // EventlogTargetTypeGuild
	EventlogTypeRobyulTwitterFeedAdd                = "Robyul_Twitter_Feed_Add"                // EventlogTargetTypeRobyulTwitterFeed
	EventlogTypeRobyulTwitterFeedRemove             = "Robyul_Twitter_Feed_Remove"             // EventlogTargetTypeRobyulTwitterFeed
	EventlogTypeRobyulActionRevert                  = "Robyul_Action_Revert"                   // EventlogTargetTypeRobyulEventlogItem

	EventlogTargetTypeRobyulBadge               = "robyul-badge"
	EventlogTargetTypeRobyulVliveFeed           = "robyul-vlive-feed"
	EventlogTargetTypeRobyulYouTubeChannelFeed  = "robyul-youtube-channel-feed"
	EventlogTargetTypeRobyulInstagramFeed       = "robyul-instagram-feed"
	EventlogTargetTypeRobyulRedditFeed          = "robyul-reddit-feed"
	EventlogTargetTypeRobyulFacebookFeed        = "robyul-facebook-feed"
	EventlogTargetTypeRobyulGallery             = "robyul-gallery"
	EventlogTargetTypeRobyulMirror              = "robyul-mirror"
	EventlogTargetTypeRobyulRandomPictureSource = "robyul-randompicture-source"
	EventlogTargetTypeRobyulTwitchFeed          = "robyul-twitch-feed"
	EventlogTargetTypeRobyulTwitterFeed         = "robyul-twitter-feed"
	EventlogTargetTypeRobyulPublicObject        = "robyul-public-object"
	EventlogTargetTypeRobyulMirrorType          = "robyul-mirror-type"
	EventlogTargetTypeRobyulEventlogItem        = "robyul-eventlog-item"

	AuditLogBackfillRedisList = "robyul-discord:eventlog:auditlog-backfills:v2"
)

type AuditLogBackfillType int

const (
	AuditLogBackfillTypeChannelCreate AuditLogBackfillType = iota
	AuditLogBackfillTypeChannelDelete
	AuditLogBackfillTypeChannelUpdate
	AuditLogBackfillTypeRoleCreate
	AuditLogBackfillTypeRoleDelete
	AuditLogBackfillTypeBanAdd
	AuditLogBackfillTypeBanRemove
	AuditLogBackfillTypeMemberRemove
	AuditLogBackfillTypeEmojiCreate
	AuditLogBackfillTypeEmojiDelete
	AuditLogBackfillTypeEmojiUpdate
	AuditLogBackfillTypeGuildUpdate
	AuditLogBackfillTypeRoleUpdate
	AuditlogBackfillTypeMemberRoleUpdate
	AuditlogBackfillTypeMemberUpdate
	AuditLogBackfillTypeChannelOverridesAdd
	AuditLogBackfillTypeChannelOverridesRemove
	AuditLogBackfillTypeChannelOverridesUpdate
)

type AuditLogBackfillRequest struct {
	GuildID string               // required
	Type    AuditLogBackfillType // required
	Count   int                  // required
	UserID  string
}
