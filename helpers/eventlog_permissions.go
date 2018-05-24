package helpers

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

func GetPermissionsText(permissions int) string {
	if permissions == 0 {
		return "/"
	}
	var result string
	if permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator {
		result += "Administrator, "
	}
	if permissions&discordgo.PermissionViewAuditLogs == discordgo.PermissionViewAuditLogs {
		result += "View Audit Log, "
	}
	if permissions&discordgo.PermissionManageServer == discordgo.PermissionManageServer {
		result += "Manage Server, "
	}
	if permissions&discordgo.PermissionManageRoles == discordgo.PermissionManageRoles {
		result += "Manage Roles, "
	}
	if permissions&discordgo.PermissionManageChannels == discordgo.PermissionManageChannels {
		result += "Manage Channels, "
	}
	if permissions&discordgo.PermissionKickMembers == discordgo.PermissionKickMembers {
		result += "Kick Members, "
	}
	if permissions&discordgo.PermissionBanMembers == discordgo.PermissionBanMembers {
		result += "Ban Members, "
	}
	if permissions&discordgo.PermissionCreateInstantInvite == discordgo.PermissionCreateInstantInvite {
		result += "Create Instant Invite, "
	}
	if permissions&discordgo.PermissionChangeNickname == discordgo.PermissionChangeNickname {
		result += "Change Nickname, "
	}
	if permissions&discordgo.PermissionManageNicknames == discordgo.PermissionManageNicknames {
		result += "Manage Nicknames, "
	}
	if permissions&discordgo.PermissionManageEmojis == discordgo.PermissionManageEmojis {
		result += "Manage Emojis, "
	}
	if permissions&discordgo.PermissionManageWebhooks == discordgo.PermissionManageWebhooks {
		result += "Manage Webhooks, "
	}
	if permissions&discordgo.PermissionReadMessages == discordgo.PermissionReadMessages {
		result += "View Channels, "
	}
	if permissions&discordgo.PermissionSendMessages == discordgo.PermissionSendMessages {
		result += "Send Messages, "
	}
	if permissions&discordgo.PermissionSendTTSMessages == discordgo.PermissionSendTTSMessages {
		result += "Send TTS Messages, "
	}
	if permissions&discordgo.PermissionManageMessages == discordgo.PermissionManageMessages {
		result += "Manage Messages, "
	}
	if permissions&discordgo.PermissionEmbedLinks == discordgo.PermissionEmbedLinks {
		result += "Embed Links, "
	}
	if permissions&discordgo.PermissionAttachFiles == discordgo.PermissionAttachFiles {
		result += "Attach Files, "
	}
	if permissions&discordgo.PermissionReadMessageHistory == discordgo.PermissionReadMessageHistory {
		result += "Read Message History, "
	}
	if permissions&discordgo.PermissionMentionEveryone == discordgo.PermissionMentionEveryone {
		result += "Mention Everyone, "
	}
	if permissions&discordgo.PermissionUseExternalEmojis == discordgo.PermissionUseExternalEmojis {
		result += "Use External Emojis, "
	}
	if permissions&discordgo.PermissionAddReactions == discordgo.PermissionAddReactions {
		result += "Add Reactions, "
	}
	if permissions&discordgo.PermissionVoiceConnect == discordgo.PermissionVoiceConnect {
		result += "Voice Connect, "
	}
	if permissions&discordgo.PermissionVoiceSpeak == discordgo.PermissionVoiceSpeak {
		result += "Voice Speak, "
	}
	if permissions&discordgo.PermissionVoiceMuteMembers == discordgo.PermissionVoiceMuteMembers {
		result += "Voice Mute Members, "
	}
	if permissions&discordgo.PermissionVoiceDeafenMembers == discordgo.PermissionVoiceDeafenMembers {
		result += "Voice Deafen Members, "
	}
	if permissions&discordgo.PermissionVoiceMoveMembers == discordgo.PermissionVoiceMoveMembers {
		result += "Voice Move Members, "
	}
	if permissions&discordgo.PermissionVoiceUseVAD == discordgo.PermissionVoiceUseVAD {
		result += "Voice Use Voice Acivity, "
	}
	result = strings.TrimRight(result, ", ")
	return result
}
