package helpers

import (
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
)

func CanRevert(item models.ElasticEventlog) bool {
	if item.Reverted {
		return false
	}

	if len(item.Changes) <= 0 && len(item.Options) <= 0 {
		return false
	}

	switch item.ActionType {
	case models.EventlogTypeChannelUpdate:
		if containsAllowedChangesOrOptions(
			item,
			[]string{"channel_name", "channel_topic", "channel_nsfw", "channel_bitrate", "channel_parentid", "channel_permissionoverwrites"},
			nil,
		) {
			return true
		}
	case models.EventlogTypeRoleUpdate:
		if containsAllowedChangesOrOptions(
			item,
			[]string{"role_name", "role_mentionable", "role_hoist", "role_color", "role_permissions"},
			nil,
		) {
			return true
		}
	case models.EventlogTypeMemberUpdate:
		if containsAllowedChangesOrOptions(
			item,
			[]string{"member_nick"},
			[]string{"member_roles_added", "member_roles_removed"},
		) {
			return true
		}
	case models.EventlogTypeGuildUpdate:
		if containsAllowedChangesOrOptions(
			item,
			[]string{"guild_name", "guild_icon_object", "guild_region", "guild_afkchannelid", "guild_afktimeout", "guild_verificationlevel", "guild_defaultmessagenotifications"},
			nil,
		) {
			return true
		}
	case models.EventlogTypeEmojiDelete:
		return true
	case models.EventlogTypeEmojiUpdate:
		if containsAllowedChangesOrOptions(
			item,
			[]string{"emoji_name"},
			nil,
		) {
			return true
		}
	case models.EventlogTypeChannelDelete:
		if containsAllowedChangesOrOptions(
			item,
			nil,
			[]string{"channel_name", "channel_type", "channel_topic", "channel_nsfw", "channel_bitrate", "channel_parentid", "channel_permissionoverwrites"},
		) {
			return true
		}
	}

	return false
}

func containsAllowedChangesOrOptions(eventlogEntry models.ElasticEventlog, changes []string, options []string) bool {
	if len(eventlogEntry.Changes) > 0 {
		for _, change := range eventlogEntry.Changes {
			for _, key := range changes {
				if change.Key == key {
					return true
				}
			}
		}
	}
	if len(eventlogEntry.Options) > 0 {
		for _, option := range eventlogEntry.Options {
			for _, key := range options {
				if option.Key == key {
					return true
				}
			}
		}
	}
	return false
}

func Revert(eventlogID, userID string, item models.ElasticEventlog) (err error) {
	switch item.ActionType {
	case models.EventlogTypeChannelUpdate:
		channel, err := GetChannel(item.TargetID)
		if err != nil {
			return err
		}

		channelEdit := &discordgo.ChannelEdit{ // restore ints because go
			Position: channel.Position,
			Bitrate:  channel.Bitrate,
		}
		for _, change := range item.Changes {
			switch change.Key {
			case "channel_name":
				channelEdit.Name = change.OldValue
			case "channel_topic":
				channelEdit.Topic = change.OldValue
			case "channel_nsfw":
				channelEdit.NSFW = GetStringAsBool(change.OldValue)
			case "channel_bitrate":
				newBitrate, err := strconv.Atoi(change.OldValue)
				if err == nil {
					channelEdit.Bitrate = newBitrate
				}
			case "channel_parentid":
				channelEdit.ParentID = change.OldValue
			case "channel_permissionoverwrites":
				newOverwrites := make([]*discordgo.PermissionOverwrite, 0)
				oldOverwritesTexts := strings.Split(change.OldValue, ";")
				for _, oldOverwriteText := range oldOverwritesTexts {
					var oldOverwrite *discordgo.PermissionOverwrite
					err = jsoniter.UnmarshalFromString(oldOverwriteText, &oldOverwrite)
					RelaxLog(err)
					if err == nil && oldOverwrite != nil {
						newOverwrites = append(newOverwrites, oldOverwrite)
					}
				}
				channelEdit.PermissionOverwrites = newOverwrites
			}
		}

		_, err = cache.GetSession().SessionForGuildS(item.GuildID).ChannelEditComplex(item.TargetID, channelEdit)
		if err != nil {
			return err
		}

		return logRevert(channel.GuildID, userID, eventlogID)
	case models.EventlogTypeRoleUpdate:
		role, err := cache.GetSession().SessionForGuildS(item.GuildID).State.Role(item.GuildID, item.TargetID)
		if err != nil {
			return err
		}

		newName := role.Name
		newMentionable := role.Mentionable
		newHoist := role.Hoist
		newColor := role.Color
		newPermissions := role.Permissions

		for _, change := range item.Changes {
			switch change.Key {
			case "role_name":
				newName = change.OldValue
			case "role_mentionable":
				newMentionable = GetStringAsBool(change.OldValue)
			case "role_hoist":
				newHoist = GetStringAsBool(change.OldValue)
			case "role_color":
				newColor = GetDiscordColorFromHex(change.OldValue)
			case "role_permissions":
				tempPermissions, err := strconv.Atoi(change.OldValue)
				if err == nil {
					newPermissions = tempPermissions
				}
			}
		}

		role, err = cache.GetSession().SessionForGuildS(item.GuildID).GuildRoleEdit(item.GuildID, item.TargetID, newName, newColor, newHoist, newPermissions, newMentionable)
		if err != nil {
			return err
		}

		return logRevert(item.GuildID, userID, eventlogID)
	case models.EventlogTypeMemberUpdate:
		for _, change := range item.Changes {
			switch change.Key {
			case "member_nick":
				err = cache.GetSession().SessionForGuildS(item.GuildID).GuildMemberNickname(item.GuildID, item.TargetID, change.OldValue)
				if err != nil {
					return err
				}
			}
		}

		for _, option := range item.Options {
			switch option.Key {
			case "member_roles_added":
				for _, roleID := range strings.Split(option.Value, ";") {
					err = cache.GetSession().SessionForGuildS(item.GuildID).GuildMemberRoleRemove(item.GuildID, item.TargetID, roleID)
					if err != nil {
						return err
					}
				}
			case "member_roles_removed":
				for _, roleID := range strings.Split(option.Value, ";") {
					err = cache.GetSession().SessionForGuildS(item.GuildID).GuildMemberRoleAdd(item.GuildID, item.TargetID, roleID)
					if err != nil {
						return err
					}
				}
			}
		}

		return logRevert(item.GuildID, userID, eventlogID)
	case models.EventlogTypeGuildUpdate:
		guild, err := GetGuildWithoutApi(item.TargetID)
		if err != nil {
			return err
		}

		guildParams := discordgo.GuildParams{
			DefaultMessageNotifications: int(guild.DefaultMessageNotifications),
			AfkTimeout:                  guild.AfkTimeout,
			AfkChannelID:                guild.AfkChannelID,
		}

		for _, change := range item.Changes {
			switch change.Key {
			case "guild_name":
				guildParams.Name = change.OldValue
			case "guild_icon_object":
				// retrieve previous icon
				iconData, err := RetrieveFile(change.OldValue)
				if err != nil {
					return err
				}

				// convert icon to jpeg
				mimeType, _ := SniffMime(iconData)

				// encode jpeg to base64
				iconJpegBase64 := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(iconData)

				guildParams.Icon = iconJpegBase64
			case "guild_region":
				guildParams.Region = change.OldValue
			case "guild_afkchannelid":
				guildParams.AfkChannelID = change.OldValue
			case "guild_afktimeout":
				newTimeout, err := strconv.Atoi(change.OldValue)
				RelaxLog(err)
				if err == nil {
					guildParams.AfkTimeout = newTimeout
				}
			case "guild_verificationlevel":
				newVerificationLevel, err := strconv.Atoi(change.OldValue)
				RelaxLog(err)
				if err == nil {
					level := discordgo.VerificationLevel(newVerificationLevel)
					guildParams.VerificationLevel = &level
				}
			case "guild_defaultmessagenotifications":
				newDefaultMessageNotifications, err := strconv.Atoi(change.OldValue)
				RelaxLog(err)
				if err == nil {
					guildParams.DefaultMessageNotifications = newDefaultMessageNotifications
				}
			}
		}

		_, err = cache.GetSession().SessionForGuildS(item.GuildID).GuildEdit(item.TargetID, guildParams)
		if err != nil {
			return err
		}

		return logRevert(item.GuildID, userID, eventlogID)
	case models.EventlogTypeEmojiDelete:
		var emojiName, emojiImage, emojiURL string
		var emojiRoles []string

		emojiURL = discordgo.EndpointEmoji(item.TargetID)
		for _, option := range item.Options {
			switch option.Key {
			case "emoji_animated":
				if GetStringAsBool(option.Value) {
					emojiURL = strings.Replace(emojiURL, ".png", ".gif", -1)
				}
			}
		}

		// retrieve previous icon
		iconData, err := NetGetUAWithError(emojiURL, DEFAULT_UA)
		if err != nil {
			return err
		}

		// read icon
		filetype, err := SniffMime(iconData)
		if err != nil {
			return err
		}

		// encode jpeg to base64
		emojiImage = "data:" + filetype + ";base64," + base64.StdEncoding.EncodeToString(iconData)

		for _, option := range item.Options {
			switch option.Key {
			case "emoji_name":
				emojiName = option.Value
			case "emoji_roleids":
				if option.Value != "" {
					emojiRoles = strings.Split(option.Value, ";")
				}
			}
		}

		_, err = cache.GetSession().SessionForGuildS(item.GuildID).GuildEmojiCreate(item.GuildID, emojiName, emojiImage, emojiRoles)
		if err != nil {
			return err
		}

		return logRevert(item.GuildID, userID, eventlogID)
	case models.EventlogTypeEmojiUpdate:
		emoji, err := cache.GetSession().SessionForGuildS(item.GuildID).State.Emoji(item.GuildID, item.TargetID)
		if err != nil {
			return err
		}

		var emojiName string

		for _, change := range item.Changes {
			switch change.Key {
			case "emoji_name":
				emojiName = change.OldValue
			}
		}

		_, err = cache.GetSession().SessionForGuildS(item.GuildID).GuildEmojiEdit(item.GuildID, item.TargetID, emojiName, emoji.Roles)
		if err != nil {
			return err
		}

		return logRevert(item.GuildID, userID, eventlogID)
	case models.EventlogTypeChannelDelete:
		var channelName, channelTopic, channelParentID string
		var channelType discordgo.ChannelType
		var channelNSFW bool
		var channelBitrate int
		channelOverwrites := make([]*discordgo.PermissionOverwrite, 0)

		for _, option := range item.Options {
			switch option.Key {
			case "channel_name":
				channelName = option.Value
			case "channel_type":
				level, err := strconv.Atoi(option.Value)
				if err == nil {
					channelType = discordgo.ChannelType(level)
				}
			case "channel_topic":
				channelTopic = option.Value
			case "channel_nsfw":
				if GetStringAsBool(option.Value) {
					channelNSFW = true
				}
			case "channel_bitrate":
				bitrate, err := strconv.Atoi(option.Value)
				if err == nil {
					channelBitrate = bitrate
				}
			case "channel_parentid":
				channelParentID = option.Value
			case "channel_permissionoverwrites":
				overwritesTexts := strings.Split(option.Value, ";")
				for _, overwriteText := range overwritesTexts {
					var overwrite *discordgo.PermissionOverwrite
					err = jsoniter.UnmarshalFromString(overwriteText, &overwrite)
					RelaxLog(err)
					if err == nil && overwrite != nil {
						channelOverwrites = append(channelOverwrites, overwrite)
					}
				}
			}
		}

		channel, err := cache.GetSession().SessionForGuildS(item.GuildID).GuildChannelCreate(item.GuildID, channelName, channelType)
		if err != nil {
			return err
		}

		_, err = cache.GetSession().SessionForGuildS(item.GuildID).ChannelEditComplex(channel.ID, &discordgo.ChannelEdit{
			Name:                 channelName,
			Topic:                channelTopic,
			NSFW:                 channelNSFW,
			Bitrate:              channelBitrate,
			PermissionOverwrites: channelOverwrites,
			ParentID:             channelParentID,
		})
		if err != nil {
			return err
		}

		return logRevert(item.GuildID, userID, eventlogID)
	}

	return errors.New("eventlog action type not supported")
}

func logRevert(guildID, userID, eventlogID string) error {
	// add new eventlog entry for revert
	_, err := EventlogLog(time.Now(), guildID, eventlogID,
		models.EventlogTargetTypeRobyulEventlogItem, userID,
		models.EventlogTypeRobyulActionRevert, "",
		nil,
		nil,
		false,
	)
	if err != nil {
		return err
	}

	// get issuer user
	user, err := GetUserWithoutAPI(userID)
	if err != nil {
		user = new(discordgo.User)
		user.ID = userID
		user.Username = "N/A"
		user.Discriminator = "N/A"
	}

	// add option to reverted action with information
	err = EventlogLogUpdate(
		eventlogID,
		"",
		[]models.ElasticEventlogOption{{
			Key:   "reverted_by_userid",
			Value: user.ID,
			Type:  models.EventlogTargetTypeUser,
		}},
		nil,
		"",
		false,
		true,
	)
	return err
}
