package helpers

import (
	"encoding/json"
	"time"

	"sync"

	"strconv"

	"strings"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
)

var (
	AuditLogBackfillRequestsLock = sync.Mutex{}
)

/*
_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetID,
	models.EventlogTargetType, msg.Author.ID,
	models.EventlogType, "",
	[]models.ElasticEventlogChange{
		{
			Key:      "key",
			OldValue: "",
			NewValue: "",
		},
	},
	[]models.ElasticEventlogOption{
		{
			Key:   "",
			Value: "",
		},
	}, false)
helpers.RelaxLog(err)
*/

func EventlogLog(createdAt time.Time, guildID, targetID, targetType, userID, actionType, reason string,
	changes []models.ElasticEventlogChange, options []models.ElasticEventlogOption, waitingForAuditLogBackfill bool) (added bool, err error) {
	if guildID == "" {
		return false, nil
	}

	if IsBlacklistedGuild(guildID) {
		return false, nil
	}

	if IsLimitedGuild(guildID) {
		return false, nil
	}

	if GuildSettingsGetCached(guildID).EventlogDisabled {
		return false, nil
	}

	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	/*
		cache.GetLogger().WithField("module", "helpers/eventlog").Debugf(
			"adding to eventlog time %s guildID %s targetID %s userID %s actionType %s reason %s changes %+v options %+v",
			createdAt.Format(time.RFC3339), guildID, targetID, userID, actionType, reason, changes, options,
		)
	*/

	messageIDs := make([]string, 0)
	eventlogChannelIDs := GuildSettingsGetCached(guildID).EventlogChannelIDs
	for _, eventlogChannelID := range eventlogChannelIDs {
		messages, _ := SendEmbed(eventlogChannelID, getEventlogEmbed(createdAt, guildID, targetID, targetType, userID,
			actionType, reason, cleanChanges(changes), cleanOptions(options), waitingForAuditLogBackfill))
		if messages != nil && len(messages) >= 1 {
			messageIDs = append(messageIDs, eventlogChannelID+"|"+messages[0].ID)
		}
	}

	err = ElasticAddEventlog(createdAt, guildID, targetID, targetType, userID, actionType, reason, changes, options, waitingForAuditLogBackfill, messageIDs)

	if err != nil {
		return false, err
	}

	return true, nil
}

func EventlogLogUpdate(elasticID string, UserID string,
	options []models.ElasticEventlogOption, changes []models.ElasticEventlogChange,
	reason string, auditLogBackfilled bool) (err error) {
	eventlogItem, err := ElasticUpdateEventLog(elasticID, UserID, cleanOptions(options), cleanChanges(changes), reason,
		auditLogBackfilled)
	if err != nil {
		return
	}

	if eventlogItem != nil && eventlogItem.EventlogMessages != nil && len(eventlogItem.EventlogMessages) > 0 {
		embed := getEventlogEmbed(eventlogItem.CreatedAt, eventlogItem.GuildID, eventlogItem.TargetID,
			eventlogItem.TargetType, eventlogItem.UserID, eventlogItem.ActionType, eventlogItem.Reason,
			eventlogItem.Changes, eventlogItem.Options, eventlogItem.WaitingFor.AuditLogBackfill)
		for _, messageID := range eventlogItem.EventlogMessages {
			if strings.Contains(messageID, "|") {
				parts := strings.SplitN(messageID, "|", 2)
				if len(parts) >= 2 {
					EditEmbed(parts[0], parts[1], embed)
				}
			}
		}
	}

	return
}

func eventlogTargetsToText(guildID, targetType, idsText string) (names []string) {
	names = make([]string, 0)
	ids := strings.Split(idsText, ",")
	for _, id := range ids {
		targetName := id
		switch targetType {
		case models.EventlogTargetTypeUser:
			targetUser, err := GetUserWithoutAPI(id)
			if err == nil {
				targetName = targetUser.Username + "#" + targetUser.Discriminator
			}
			break
		case models.EventlogTargetTypeChannel:
			targetChannel, err := GetChannelWithoutApi(id)
			if err == nil {
				targetName = "#" + targetChannel.Name
				if targetChannel.ParentID != "" {
					targetParentChannel, err := GetChannelWithoutApi(targetChannel.ParentID)
					if err == nil {
						targetName = "#" + targetParentChannel.Name + " / " + targetName
					}
				}
			}
			break
		case models.EventlogTargetTypeRole:
			targetRole, err := cache.GetSession().State.Role(guildID, id)
			if err == nil {
				targetName = "@" + targetRole.Name
			}
			break
		case models.EventlogTargetTypeEmoji:
			targetEmoji, err := cache.GetSession().State.Emoji(guildID, id)
			if err == nil {
				targetName = targetEmoji.Name
			}
			break
		case models.EventlogTargetTypeGuild:
			targetGuild, err := GetGuildWithoutApi(id)
			if err == nil {
				targetName = targetGuild.Name
			}
			break
		case models.EventlogTargetTypeRobyulPublicObject:
			targetUrl, err := GetFileLink(id)
			if err == nil {
				targetName = targetUrl
			}
		case models.EventlogTargetTypeMessage:
			break
		}
		names = append(names, targetName)
	}
	return names
}

func getEventlogEmbed(createdAt time.Time, guildID, targetID, targetType, userID, actionType, reason string,
	changes []models.ElasticEventlogChange, options []models.ElasticEventlogOption, waitingForAuditLogBackfill bool) (embed *discordgo.MessageEmbed) {

	targetNames := strings.Join(eventlogTargetsToText(guildID, targetType, targetID), ", ")
	if targetNames == targetID {
		targetNames = ""
	} else {
		targetNames += ", "
	}

	embed = &discordgo.MessageEmbed{
		URL:       "",
		Type:      "",
		Title:     actionType + ": #" + targetID + " (" + targetNames + targetType + ")",
		Timestamp: createdAt.Format(time.RFC3339),
		Fields: []*discordgo.MessageEmbedField{{
			Name:  "Reason",
			Value: reason,
		},
		},
		Footer: &discordgo.MessageEmbedFooter{Text: "Robyul Eventlog is currently in Beta"},
	}

	embed.Color = GetDiscordColorFromHex("#73d016") // lime green
	// mark possibly destructive events red
	if actionType == models.EventlogTypeMemberLeave ||
		actionType == models.EventlogTypeChannelDelete ||
		actionType == models.EventlogTypeRoleDelete ||
		actionType == models.EventlogTypeBanAdd ||
		actionType == models.EventlogTypeBanRemove ||
		actionType == models.EventlogTypeEmojiDelete ||
		actionType == models.EventlogTypeRobyulBadgeDelete ||
		actionType == models.EventlogTypeRobyulLevelsReset ||
		actionType == models.EventlogTypeRobyulLevelsRoleDelete ||
		actionType == models.EventlogTypeRobyulVliveFeedRemove ||
		actionType == models.EventlogTypeRobyulInstagramFeedRemove ||
		actionType == models.EventlogTypeRobyulRedditFeedRemove ||
		actionType == models.EventlogTypeRobyulFacebookFeedRemove ||
		actionType == models.EventlogTypeRobyulCleanup ||
		actionType == models.EventlogTypeRobyulMute ||
		actionType == models.EventlogTypeRobyulUnmute ||
		actionType == models.EventlogTypeRobyulChatlogUpdate ||
		actionType == models.EventlogTypeRobyulBiasConfigDelete ||
		actionType == models.EventlogTypeRobyulAutoroleRemove ||
		actionType == models.EventlogTypeRobyulGalleryRemove ||
		actionType == models.EventlogTypeRobyulMirrorDelete ||
		actionType == models.EventlogTypeRobyulStarboardDelete ||
		actionType == models.EventlogTypeRobyulRandomPictureSourceRemove ||
		actionType == models.EventlogTypeRobyulCommandsDelete ||
		actionType == models.EventlogTypeRobyulTwitchFeedRemove ||
		actionType == models.EventlogTypeRobyulTroublemakerReport ||
		actionType == models.EventlogTypeRobyulPersistencyRoleRemove ||
		actionType == models.EventlogTypeRobyulEventlogConfigUpdate ||
		actionType == models.EventlogTypeRobyulTwitterFeedRemove {
		embed.Color = GetDiscordColorFromHex("#b22222") // firebrick red
	}
	if waitingForAuditLogBackfill {
		embed.Color = GetDiscordColorFromHex("#ffb80a") // orange
	}

	if changes != nil {
		for _, change := range changes {
			oldValueText := "`" + change.OldValue + "`"
			if change.OldValue == "" {
				oldValueText = "_/_"
			} else {
				oldValueText = strings.Join(eventlogTargetsToText(guildID, change.Type, change.OldValue), ", ")
			}
			newValueText := "`" + change.NewValue + "`"
			if change.NewValue == "" {
				newValueText = "_/_"
			} else {
				newValueText = strings.Join(eventlogTargetsToText(guildID, change.Type, change.NewValue), ", ")
			}
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:  change.Key,
				Value: oldValueText + " ➡ " + newValueText,
			})
		}
	}

	if options != nil {
		for _, option := range options {
			valueText := "`" + option.Value + "`"
			if option.Value == "" {
				valueText = "_/_"
			} else {
				valueText = strings.Join(eventlogTargetsToText(guildID, option.Type, option.Value), ", ")
			}
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:  option.Key,
				Value: valueText,
			})
		}
	}

	if userID != "" {
		user, err := GetUserWithoutAPI(userID)
		if err != nil {
			user = new(discordgo.User)
			user.Username = "N/A"
		}
		embed.Author = &discordgo.MessageEmbedAuthor{
			Name:    user.Username,
			IconURL: user.AvatarURL("64"),
		}
	}

	return embed
}

func RequestAuditLogBackfill(guildID string, backfillType models.AuditLogBackfillType) (err error) {
	AuditLogBackfillRequestsLock.Lock()
	defer AuditLogBackfillRequestsLock.Unlock()

	marshaledData, err := json.Marshal(models.AuditLogBackfillRequest{
		GuildID: guildID,
		Type:    backfillType,
	})
	if err != nil {
		return err
	}

	redis := cache.GetRedisClient()

	_, err = redis.SAdd(models.AuditLogBackfillRedisSet, marshaledData).Result()
	return
}

func OnEventlogEmojiCreate(guildID string, emoji *discordgo.Emoji) {
	leftAt := time.Now()

	options := make([]models.ElasticEventlogOption, 0)

	iconObjectOption := getDiscordFileHashOption("emoji_icon_object",
		emoji.ID,
		discordgo.EndpointEmoji(emoji.ID),
		"", "", guildID)

	options = append(options, models.ElasticEventlogOption{
		Key:   "emoji_name",
		Value: emoji.Name,
	})

	options = append(options, models.ElasticEventlogOption{
		Key:   "emoji_managed",
		Value: StoreBoolAsString(emoji.Managed),
	})

	options = append(options, models.ElasticEventlogOption{
		Key:   "emoji_requirecolons",
		Value: StoreBoolAsString(emoji.RequireColons),
	})

	options = append(options, models.ElasticEventlogOption{
		Key:   "emoji_animated",
		Value: StoreBoolAsString(emoji.Animated),
	})

	options = append(options, models.ElasticEventlogOption{
		Key:   "emoji_apiname",
		Value: emoji.APIName(),
	})

	options = append(options, models.ElasticEventlogOption{
		Key:   "emoji_roleids",
		Value: strings.Join(emoji.Roles, ","),
		Type:  models.EventlogTargetTypeRole,
	})

	if iconObjectOption.Value != "" {
		options = append(options, iconObjectOption)
	}

	added, err := EventlogLog(leftAt, guildID, emoji.ID, models.EventlogTargetTypeEmoji, "", models.EventlogTypeEmojiCreate, "", nil, options, true)
	RelaxLog(err)
	if added {
		err := RequestAuditLogBackfill(guildID, models.AuditLogBackfillTypeEmojiCreate)
		RelaxLog(err)
	}
}

func OnEventlogEmojiDelete(guildID string, emoji *discordgo.Emoji) {
	leftAt := time.Now()

	options := make([]models.ElasticEventlogOption, 0)

	iconObjectOption := getDiscordFileHashOption("emoji_icon_object",
		emoji.ID,
		discordgo.EndpointEmoji(emoji.ID),
		"", "", guildID)

	options = append(options, models.ElasticEventlogOption{
		Key:   "emoji_name",
		Value: emoji.Name,
	})

	options = append(options, models.ElasticEventlogOption{
		Key:   "emoji_managed",
		Value: StoreBoolAsString(emoji.Managed),
	})

	options = append(options, models.ElasticEventlogOption{
		Key:   "emoji_requirecolons",
		Value: StoreBoolAsString(emoji.RequireColons),
	})

	options = append(options, models.ElasticEventlogOption{
		Key:   "emoji_animated",
		Value: StoreBoolAsString(emoji.Animated),
	})

	options = append(options, models.ElasticEventlogOption{
		Key:   "emoji_apiname",
		Value: emoji.APIName(),
	})

	options = append(options, models.ElasticEventlogOption{
		Key:   "emoji_roleids",
		Value: strings.Join(emoji.Roles, ","),
		Type:  models.EventlogTargetTypeRole,
	})

	if iconObjectOption.Value != "" {
		options = append(options, iconObjectOption)
	}

	added, err := EventlogLog(leftAt, guildID, emoji.ID, models.EventlogTargetTypeEmoji, "", models.EventlogTypeEmojiDelete, "", nil, options, true)
	RelaxLog(err)
	if added {
		err := RequestAuditLogBackfill(guildID, models.AuditLogBackfillTypeEmojiDelete)
		RelaxLog(err)
	}
}

func OnEventlogEmojiUpdate(guildID string, oldEmoji, newEmoji *discordgo.Emoji) {
	leftAt := time.Now()

	options := make([]models.ElasticEventlogOption, 0)

	options = append(options, models.ElasticEventlogOption{
		Key:   "emoji_name",
		Value: newEmoji.Name,
	})

	options = append(options, models.ElasticEventlogOption{
		Key:   "emoji_managed",
		Value: StoreBoolAsString(newEmoji.Managed),
	})

	options = append(options, models.ElasticEventlogOption{
		Key:   "emoji_requirecolons",
		Value: StoreBoolAsString(newEmoji.RequireColons),
	})

	options = append(options, models.ElasticEventlogOption{
		Key:   "emoji_animated",
		Value: StoreBoolAsString(newEmoji.Animated),
	})

	options = append(options, models.ElasticEventlogOption{
		Key:   "emoji_apiname",
		Value: newEmoji.APIName(),
	})

	options = append(options, models.ElasticEventlogOption{
		Key:   "emoji_roleids",
		Value: strings.Join(newEmoji.Roles, ","),
		Type:  models.EventlogTargetTypeRole,
	})

	rolesAdded, rolesRemoved := StringSliceDiff(oldEmoji.Roles, newEmoji.Roles)
	if len(rolesAdded) >= 0 {
		options = append(options, models.ElasticEventlogOption{
			Key:   "emoji_roleids_added",
			Value: strings.Join(rolesAdded, ","),
			Type:  models.EventlogTargetTypeRole,
		})
	}
	if len(rolesRemoved) >= 0 {
		options = append(options, models.ElasticEventlogOption{
			Key:   "emoji_roleids_removed",
			Value: strings.Join(rolesRemoved, ","),
			Type:  models.EventlogTargetTypeRole,
		})
	}

	changes := make([]models.ElasticEventlogChange, 0)

	if oldEmoji.Name != newEmoji.Name {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "emoji_name",
			OldValue: oldEmoji.Name,
			NewValue: newEmoji.Name,
		})
	}

	if oldEmoji.Managed != newEmoji.Managed {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "emoji_managed",
			OldValue: StoreBoolAsString(oldEmoji.Managed),
			NewValue: StoreBoolAsString(newEmoji.Managed),
		})
	}

	if oldEmoji.RequireColons != newEmoji.RequireColons {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "emoji_requirecolons",
			OldValue: StoreBoolAsString(oldEmoji.RequireColons),
			NewValue: StoreBoolAsString(newEmoji.RequireColons),
		})
	}

	if oldEmoji.Animated != newEmoji.Animated {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "emoji_animated",
			OldValue: StoreBoolAsString(oldEmoji.Animated),
			NewValue: StoreBoolAsString(newEmoji.Animated),
		})
	}

	if oldEmoji.APIName() != newEmoji.APIName() {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "emoji_apiname",
			OldValue: oldEmoji.APIName(),
			NewValue: newEmoji.APIName(),
		})
	}

	if len(rolesAdded) > 0 || len(rolesRemoved) > 0 {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "emoji_roleids",
			OldValue: strings.Join(oldEmoji.Roles, ","),
			NewValue: strings.Join(newEmoji.Roles, ","),
			Type:     models.EventlogTargetTypeRole,
		})
	}

	added, err := EventlogLog(leftAt, guildID, newEmoji.ID, models.EventlogTargetTypeEmoji, "", models.EventlogTypeEmojiUpdate, "", changes, options, true)
	RelaxLog(err)
	if added {
		err := RequestAuditLogBackfill(guildID, models.AuditLogBackfillTypeEmojiUpdate)
		RelaxLog(err)
	}
}

func getDiscordFileHashOption(key, hash, hashHurl, userID, channelID, guildID string) models.ElasticEventlogOption {
	metadataKey := "discord_" + key

	iconObjectOption := models.ElasticEventlogOption{
		Key:  key,
		Type: models.EventlogTargetTypeRobyulPublicObject,
	}

	// try to get old icon from object storage
	oldObjects, _ := RetrieveFilesByAdditionalObjectMetadata(metadataKey, hash)
	if oldObjects != nil && len(oldObjects) >= 1 {
		iconObjectOption.Value = oldObjects[0]
	} else {
		// try to download old icon if not found in object storage
		oldGuildIconData, err := NetGetUAWithError(hashHurl, DEFAULT_UA)
		RelaxLog(err)
		if err == nil {
			objectName, err := AddFile("", oldGuildIconData, AddFileMetadata{
				UserID:    userID,
				ChannelID: channelID,
				GuildID:   guildID,
				AdditionalMetadata: map[string]string{
					metadataKey: hash,
				},
			}, "eventlog", true)
			RelaxLog(err)
			if err == nil {
				iconObjectOption.Value = objectName
			}
		}
	}

	return iconObjectOption
}

func getDiscordFileHashChange(key, oldHash, newHash, oldHashUrl, newHashUrl, userID, channelID, guildID string) models.ElasticEventlogChange {
	metadataKey := "discord_" + key

	iconObjectChange := models.ElasticEventlogChange{
		Key:  key,
		Type: models.EventlogTargetTypeRobyulPublicObject,
	}

	// try to get old icon from object storage
	oldObjects, _ := RetrieveFilesByAdditionalObjectMetadata(metadataKey, oldHash)
	if oldObjects != nil && len(oldObjects) >= 1 {
		iconObjectChange.OldValue = oldObjects[0]
	} else {
		// try to download old icon if not found in object storage
		oldGuildIconData, err := NetGetUAWithError(oldHashUrl, DEFAULT_UA)
		RelaxLog(err)
		if err == nil {
			objectName, err := AddFile("", oldGuildIconData, AddFileMetadata{
				UserID:    userID,
				ChannelID: channelID,
				GuildID:   guildID,
				AdditionalMetadata: map[string]string{
					metadataKey: oldHash,
				},
			}, "eventlog", true)
			RelaxLog(err)
			if err == nil {
				iconObjectChange.OldValue = objectName
			}
		}
	}
	// try to get new icon from object storage
	newObjects, _ := RetrieveFilesByAdditionalObjectMetadata(metadataKey, newHash)
	if newObjects != nil && len(newObjects) >= 1 {
		iconObjectChange.NewValue = newObjects[0]
	} else {
		// try to download new icon if not found in object storage
		newGuildIconData, err := NetGetUAWithError(newHashUrl, DEFAULT_UA)
		RelaxLog(err)
		if err == nil {
			objectName, err := AddFile("", newGuildIconData, AddFileMetadata{
				UserID:    userID,
				ChannelID: channelID,
				GuildID:   guildID,
				AdditionalMetadata: map[string]string{
					metadataKey: newHash,
				},
			}, "eventlog", true)
			RelaxLog(err)
			iconObjectChange.NewValue = objectName
		}
	}

	return iconObjectChange
}

func OnEventlogGuildUpdate(guildID string, oldGuild, newGuild *discordgo.Guild) {
	leftAt := time.Now()

	changes := make([]models.ElasticEventlogChange, 0)
	if oldGuild.Name != newGuild.Name {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "guild_name",
			OldValue: oldGuild.Name,
			NewValue: newGuild.Name,
		})
	}

	if oldGuild.Icon != newGuild.Icon {
		iconObjectChange := getDiscordFileHashChange("guild_icon_object",
			oldGuild.Icon, newGuild.Icon,
			discordgo.EndpointGuildIcon(guildID, oldGuild.Icon),
			discordgo.EndpointGuildIcon(guildID, newGuild.Icon),
			"", "", newGuild.ID)

		changes = append(changes, models.ElasticEventlogChange{
			Key:      "guild_icon",
			OldValue: oldGuild.Icon,
			NewValue: newGuild.Icon,
		})

		if iconObjectChange.OldValue != "" || iconObjectChange.NewValue != "" {
			changes = append(changes, iconObjectChange)
		}
	}

	if oldGuild.Region != newGuild.Region {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "guild_region",
			OldValue: oldGuild.Region,
			NewValue: newGuild.Region,
		})
	}

	if oldGuild.AfkChannelID != newGuild.AfkChannelID {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "guild_afkchannelid",
			OldValue: oldGuild.AfkChannelID,
			NewValue: newGuild.AfkChannelID,
			Type:     models.EventlogTargetTypeChannel,
		})
	}

	if oldGuild.EmbedChannelID != newGuild.EmbedChannelID {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "guild_embedchannelid",
			OldValue: oldGuild.EmbedChannelID,
			NewValue: newGuild.EmbedChannelID,
			Type:     models.EventlogTargetTypeChannel,
		})
	}

	if oldGuild.OwnerID != newGuild.OwnerID {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "guild_ownerid",
			OldValue: oldGuild.OwnerID,
			NewValue: newGuild.OwnerID,
			Type:     models.EventlogTargetTypeUser,
		})
	}

	if oldGuild.Splash != newGuild.Splash {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "guild_splash",
			OldValue: oldGuild.Splash,
			NewValue: newGuild.Splash,
		})
	}

	if oldGuild.AfkTimeout != newGuild.AfkTimeout {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "guild_afktimeout",
			OldValue: strconv.Itoa(oldGuild.AfkTimeout),
			NewValue: strconv.Itoa(newGuild.AfkTimeout),
		})
	}

	if oldGuild.VerificationLevel != newGuild.VerificationLevel {
		var oldVerificationLevel, newVerificationLevel string
		switch oldGuild.VerificationLevel {
		case discordgo.VerificationLevelNone:
			oldVerificationLevel = "none"
			break
		case discordgo.VerificationLevelLow:
			oldVerificationLevel = "low"
			break
		case discordgo.VerificationLevelMedium:
			oldVerificationLevel = "medium"
			break
		case discordgo.VerificationLevelHigh:
			oldVerificationLevel = "high"
			break
		}
		switch newGuild.VerificationLevel {
		case discordgo.VerificationLevelNone:
			newVerificationLevel = "none"
			break
		case discordgo.VerificationLevelLow:
			newVerificationLevel = "low"
			break
		case discordgo.VerificationLevelMedium:
			newVerificationLevel = "medium"
			break
		case discordgo.VerificationLevelHigh:
			newVerificationLevel = "high"
			break
		}
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "guild_verificationlevel",
			OldValue: oldVerificationLevel,
			NewValue: newVerificationLevel,
		})
	}

	if oldGuild.EmbedEnabled != newGuild.EmbedEnabled {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "guild_embedenabled",
			OldValue: StoreBoolAsString(oldGuild.EmbedEnabled),
			NewValue: StoreBoolAsString(newGuild.EmbedEnabled),
		})
	}

	if oldGuild.DefaultMessageNotifications != newGuild.DefaultMessageNotifications {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "guild_defaultmessagenotifications",
			OldValue: strconv.Itoa(oldGuild.DefaultMessageNotifications),
			NewValue: strconv.Itoa(newGuild.DefaultMessageNotifications),
		})
	}

	added, err := EventlogLog(leftAt, guildID, newGuild.ID, models.EventlogTargetTypeGuild, "", models.EventlogTypeGuildUpdate, "", changes, nil, true)
	RelaxLog(err)
	if added {
		err := RequestAuditLogBackfill(guildID, models.AuditLogBackfillTypeGuildUpdate)
		RelaxLog(err)
	}
}

func OnEventlogChannelUpdate(guildID string, oldChannel, newChannel *discordgo.Channel) {
	leftAt := time.Now()

	var backfill bool

	changes := make([]models.ElasticEventlogChange, 0)

	if oldChannel.Name != newChannel.Name {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "channel_name",
			OldValue: oldChannel.Name,
			NewValue: newChannel.Name,
		})
		backfill = true
	}

	if oldChannel.Topic != newChannel.Topic {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "channel_topic",
			OldValue: oldChannel.Topic,
			NewValue: newChannel.Topic,
		})
		backfill = true
	}

	if oldChannel.NSFW != newChannel.NSFW {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "channel_nsfw",
			OldValue: StoreBoolAsString(oldChannel.NSFW),
			NewValue: StoreBoolAsString(newChannel.NSFW),
		})
		backfill = true
	}

	/*
		if oldChannel.Position != newChannel.Position {
			changes = append(changes, models.ElasticEventlogChange{
				Key:      "channel_position",
				OldValue: strconv.Itoa(oldChannel.Position),
				NewValue: strconv.Itoa(newChannel.Position),
			})
		}
	*/

	if newChannel.Type == discordgo.ChannelTypeGuildVoice && oldChannel.Bitrate != newChannel.Bitrate {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "channel_bitrate",
			OldValue: strconv.Itoa(oldChannel.Bitrate),
			NewValue: strconv.Itoa(newChannel.Bitrate),
		})
	}

	if oldChannel.ParentID != newChannel.ParentID {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "channel_parentid",
			OldValue: oldChannel.ParentID,
			NewValue: newChannel.ParentID,
			Type:     models.EventlogTargetTypeChannel,
		})
	}

	if !ChannelOverwritesMatch(oldChannel.PermissionOverwrites, newChannel.PermissionOverwrites) {
		// TODO: handle permission overwrites
		/*
				changes = append(changes, models.ElasticEventlogChange{
					Key:      "channel_permissionoverwrites",
					OldValue: oldChannel.PermissionOverwrites,
					NewValue: newChannel.PermissionOverwrites,
				})
			backfill = true
		*/
	}

	added, err := EventlogLog(leftAt, guildID, newChannel.ID, models.EventlogTargetTypeChannel, "", models.EventlogTypeChannelUpdate, "", changes, nil, backfill)
	RelaxLog(err)
	if added && backfill {
		err := RequestAuditLogBackfill(guildID, models.AuditLogBackfillTypeChannelUpdate)
		RelaxLog(err)
	}
}

func OnEventlogMemberUpdate(guildID string, oldMember, newMember *discordgo.Member) {
	leftAt := time.Now()

	changes := make([]models.ElasticEventlogChange, 0)

	options := make([]models.ElasticEventlogOption, 0)

	rolesAdded, rolesRemoved := StringSliceDiff(oldMember.Roles, newMember.Roles)

	var memberUpdateBackfill, memberRoleUpdateBackfill bool

	if len(rolesAdded) > 0 || len(rolesRemoved) > 0 {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "member_roles",
			OldValue: strings.Join(oldMember.Roles, ","),
			NewValue: strings.Join(newMember.Roles, ","),
			Type:     models.EventlogTargetTypeRole,
		})

		if len(rolesAdded) > 0 {
			options = append(options, models.ElasticEventlogOption{
				Key:   "member_roles_added",
				Value: strings.Join(rolesAdded, ","),
				Type:  models.EventlogTargetTypeRole,
			})
		}

		if len(rolesRemoved) > 0 {
			options = append(options, models.ElasticEventlogOption{
				Key:   "member_roles_removed",
				Value: strings.Join(rolesRemoved, ","),
				Type:  models.EventlogTargetTypeRole,
			})
		}

		memberRoleUpdateBackfill = true
	}

	if oldMember.User.Username+"#"+oldMember.User.Discriminator != newMember.User.Username+"#"+newMember.User.Discriminator {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "member_username",
			OldValue: oldMember.User.Username + "#" + oldMember.User.Discriminator,
			NewValue: newMember.User.Username + "#" + newMember.User.Discriminator,
		})

		memberUpdateBackfill = true
	}

	if oldMember.Nick != newMember.Nick {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "member_nick",
			OldValue: oldMember.Nick,
			NewValue: newMember.Nick,
		})

		memberUpdateBackfill = true
	}

	// backfill? lots of requests because of bot role changes
	added, err := EventlogLog(leftAt, guildID, newMember.User.ID, models.EventlogTargetTypeUser, "", models.EventlogTypeMemberUpdate, "", changes, options, true)
	RelaxLog(err)
	if added {
		if memberRoleUpdateBackfill {
			err := RequestAuditLogBackfill(guildID, models.AuditlogBackfillTypeMemberRoleUpdate)
			RelaxLog(err)
		}
		if memberUpdateBackfill {
			err := RequestAuditLogBackfill(guildID, models.AuditlogBackfillTypeMemberUpdate)
			RelaxLog(err)
		}
	}
}

func OnEventlogRoleUpdate(guildID string, oldRole, newRole *discordgo.Role) {
	leftAt := time.Now()

	changes := make([]models.ElasticEventlogChange, 0)

	if oldRole.Name != newRole.Name {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "role_name",
			OldValue: oldRole.Name,
			NewValue: newRole.Name,
		})
	}

	if oldRole.Managed != newRole.Managed {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "role_managed",
			OldValue: StoreBoolAsString(oldRole.Managed),
			NewValue: StoreBoolAsString(newRole.Managed),
		})
	}

	if oldRole.Mentionable != newRole.Mentionable {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "role_mentionable",
			OldValue: StoreBoolAsString(oldRole.Mentionable),
			NewValue: StoreBoolAsString(newRole.Mentionable),
		})
	}

	if oldRole.Hoist != newRole.Hoist {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "role_hoist",
			OldValue: StoreBoolAsString(oldRole.Hoist),
			NewValue: StoreBoolAsString(newRole.Hoist),
		})
	}

	if oldRole.Color != newRole.Color {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "role_color",
			OldValue: GetHexFromDiscordColor(oldRole.Color),
			NewValue: GetHexFromDiscordColor(newRole.Color),
		})
	}

	/*
		if oldRole.Position != newRole.Position {
			changes = append(changes, models.ElasticEventlogChange{
				Key:      "role_position",
				OldValue: strconv.Itoa(oldRole.Position),
				NewValue: strconv.Itoa(newRole.Position),
			})
		}
	*/

	if oldRole.Permissions != newRole.Permissions {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "role_permissions",
			OldValue: strconv.Itoa(oldRole.Permissions),
			NewValue: strconv.Itoa(newRole.Permissions),
		})
	}

	added, err := EventlogLog(leftAt, guildID, newRole.ID, models.EventlogTargetTypeRole, "", models.EventlogTypeRoleUpdate, "", changes, nil, true)
	RelaxLog(err)
	if added {
		err := RequestAuditLogBackfill(guildID, models.AuditLogBackfillTypeRoleUpdate)
		RelaxLog(err)
	}
}

func StoreBoolAsString(input bool) (output string) {
	if input {
		return "yes"
	} else {
		return "no"
	}
}

func cleanChanges(oldChanges []models.ElasticEventlogChange) (newChanges []models.ElasticEventlogChange) {
	newChanges = make([]models.ElasticEventlogChange, 0)
	if oldChanges == nil {
		return
	}

	for _, oldChange := range oldChanges {
		if oldChange.Key == "" {
			continue
		}
		if oldChange.NewValue == "" && oldChange.OldValue == "" {
			continue
		}
		if oldChange.NewValue == oldChange.OldValue {
			continue
		}
		newChanges = append(newChanges, oldChange)
	}

	return
}

func cleanOptions(oldOptions []models.ElasticEventlogOption) (newOptions []models.ElasticEventlogOption) {
	newOptions = make([]models.ElasticEventlogOption, 0)
	if oldOptions == nil {
		return
	}

	for _, oldOption := range oldOptions {
		if oldOption.Key == "" {
			continue
		}
		if oldOption.Value == "" {
			continue
		}
		newOptions = append(newOptions, oldOption)
	}

	return
}
