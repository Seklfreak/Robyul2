package helpers

import (
	"encoding/json"
	"time"

	"sync"

	"strconv"

	"strings"

	"errors"

	"fmt"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/globalsign/mgo/bson"
	"github.com/json-iterator/go"
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

	eventlogID, err := ElasticAddEventlog(createdAt, guildID, targetID, targetType, userID, actionType, reason, changes, options, waitingForAuditLogBackfill, nil)
	if err != nil {
		return false, err
	}

	messageIDs := make([]string, 0)
	eventlogChannelIDs := GuildSettingsGetCached(guildID).EventlogChannelIDs
	for _, eventlogChannelID := range eventlogChannelIDs {
		messages, _ := SendEmbed(eventlogChannelID, getEventlogEmbed(eventlogID, createdAt, guildID, targetID, targetType, userID,
			actionType, reason, cleanChanges(changes), cleanOptions(options), waitingForAuditLogBackfill))
		if messages != nil && len(messages) >= 1 {
			messageIDs = append(messageIDs, eventlogChannelID+"|"+messages[0].ID)
		}
	}

	eventlogItem, err := ElasticUpdateEventLog(eventlogID, "", nil, nil, "", false, false, messageIDs)
	if err != nil {
		return true, err
	}

	if len(messageIDs) > 0 && CanRevert(*eventlogItem) {
		// add reactions
		for _, messageID := range messageIDs {
			messageIDParts := strings.SplitN(messageID, "|", 2)
			cache.GetSession().MessageReactionAdd(messageIDParts[0], messageIDParts[1], "↩")
		}
	}

	return true, nil
}

func EventlogLogUpdate(elasticID string, UserID string,
	options []models.ElasticEventlogOption, changes []models.ElasticEventlogChange,
	reason string, auditLogBackfilled, reverted bool) (err error) {
	eventlogItem, err := ElasticUpdateEventLog(elasticID, UserID, cleanOptions(options), cleanChanges(changes), reason,
		auditLogBackfilled, reverted, nil)
	if err != nil {
		return
	}

	if eventlogItem != nil && eventlogItem.EventlogMessages != nil && len(eventlogItem.EventlogMessages) > 0 {
		embed := getEventlogEmbed(elasticID, eventlogItem.CreatedAt, eventlogItem.GuildID, eventlogItem.TargetID,
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
	ids := strings.Split(idsText, ";")
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
		case models.EventlogTargetTypeRobyulEventlogItem:
			eventlogItem, err := ElasticGetEventlog(id)
			if err == nil {
				targetName = eventlogItem.ActionType
			}
		case models.EventlogTargetTypeRolePermissions:
			idNum, err := strconv.Atoi(id)
			if err == nil {
				targetName = GetPermissionsText(idNum)
			}
		case models.EventlogTargetTypePermissionOverwrite:
			var channelOverwrite discordgo.PermissionOverwrite
			err := jsoniter.UnmarshalFromString(id, &channelOverwrite)
			if err == nil {
				if channelOverwrite.Allow == 0 && channelOverwrite.Deny == 0 {
					continue
				}
				switch channelOverwrite.Type {
				case "member":
					targetUser, err := GetUserWithoutAPI(channelOverwrite.ID)
					if err == nil {
						targetName = targetUser.Username + "#" + targetUser.Discriminator + ": "
					} else {
						targetName = "User #" + channelOverwrite.ID
					}
				case "role":
					targetRole, err := cache.GetSession().State.Role(guildID, channelOverwrite.ID)
					if err == nil {
						targetName = "@" + targetRole.Name + ": "
					} else {
						targetName = "Role #" + channelOverwrite.ID
					}
				}
				if channelOverwrite.Allow > 0 {
					targetName += "Allow " + GetPermissionsText(channelOverwrite.Allow)
				}
				if channelOverwrite.Deny > 0 {
					targetName += "Deny " + GetPermissionsText(channelOverwrite.Deny)
				}
			}
		}
		names = append(names, targetName)
	}
	return names
}

func getEventlogEmbed(eventlogID string, createdAt time.Time, guildID, targetID, targetType, userID, actionType, reason string,
	changes []models.ElasticEventlogChange, options []models.ElasticEventlogOption, waitingForAuditLogBackfill bool) (embed *discordgo.MessageEmbed) {

	// get target text
	targetNames := strings.Join(eventlogTargetsToText(guildID, targetType, targetID), ", ")
	if targetNames == targetID {
		targetNames = ""
	} else {
		targetNames += ", "
	}

	// create embed
	embed = &discordgo.MessageEmbed{
		URL:       "",
		Type:      "",
		Title:     actionType + ": #" + targetID + " (" + targetNames + targetType + ")", // title: action type + targets
		Timestamp: createdAt.Format(time.RFC3339),
		Fields: []*discordgo.MessageEmbedField{{
			Name:  "Reason", // reason field
			Value: reason,
		},
		},
		Footer: &discordgo.MessageEmbedFooter{Text: "#" + eventlogID + " • Robyul Eventlog is currently in Beta"}, // #id + beta disclaimer
		Color:  GetDiscordColorFromHex("#73d016"),                                                                 // lime gree
	}

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

	// display changes as fields
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

	// display options as fields
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

	embed.Description += "\n"

	// add author information if issuer is given
	if userID != "" {
		user, err := GetUserWithoutAPI(userID)
		if err != nil {
			user = new(discordgo.User)
			user.Username = "N/A"
			user.Discriminator = "N/A"
		}
		embed.Author = &discordgo.MessageEmbedAuthor{
			Name:    user.Username + "#" + user.Discriminator + " #" + userID,
			IconURL: user.AvatarURL("64"),
		}
		embed.Description += "By <@" + userID + "> "
	}

	// add target icon and description text if possible
	targetIDs := strings.Split(targetID, ",")
	if len(targetIDs) >= 1 {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{}

		switch targetType {
		case models.EventlogTargetTypeRole:
			embed.Description += "On <@&" + strings.Join(targetIDs, ">, <@&") + ">"
		case models.EventlogTargetTypeChannel:
			embed.Description += "On <#" + strings.Join(targetIDs, ">, <#") + ">"
		case models.EventlogTargetTypeUser:
			user, err := GetUserWithoutAPI(targetIDs[0])
			if err == nil {
				embed.Thumbnail.URL = user.AvatarURL("256")
			}
			embed.Description += "On <@" + strings.Join(targetIDs, ">, <@") + ">"
		case models.EventlogTargetTypeGuild:
			guild, err := GetGuildWithoutApi(targetIDs[0])
			if err == nil && guild.Icon != "" {
				embed.Thumbnail.URL = discordgo.EndpointGuildIcon(guild.ID, guild.Icon) + "?size=256"
			}
		case models.EventlogTargetTypeEmoji:
			objectName, err := getDiscordFileHashObject("emoji_icon_object",
				targetIDs[0],
				discordgo.EndpointEmoji(targetIDs[0]),
				"", "", guildID)
			if err == nil {
				fileLink, err := GetFileLink(objectName)
				if err == nil {
					fmt.Println(fileLink)
					embed.Thumbnail.URL = fileLink
				}
			}
		case models.EventlogTargetTypeRobyulBadge:
			var entryBucket models.ProfileBadgeEntry
			err := MdbOne(
				MdbCollection(models.ProfileBadgesTable).Find(bson.M{"guildid": guildID, "_id": HumanToMdbId(targetIDs[0])}),
				&entryBucket,
			)
			if err == nil && entryBucket.ObjectName != "" {
				fileLink, err := GetFileLink(entryBucket.ObjectName)
				if err == nil {
					embed.Thumbnail.URL = fileLink
				}
			}
		}
	}

	return embed
}

// requests an audit log backfill
// guildID and backfillType fields are required
func RequestAuditLogBackfill(guildID string, backfillType models.AuditLogBackfillType, userID string) (err error) {
	if guildID == "" || backfillType < 0 {
		return errors.New("invalid backfill request")
	}

	AuditLogBackfillRequestsLock.Lock()
	defer AuditLogBackfillRequestsLock.Unlock()

	marshaledData, err := json.Marshal(models.AuditLogBackfillRequest{
		GuildID: guildID,
		Type:    backfillType,
		UserID:  userID,
		Count:   1,
	})
	if err != nil {
		return err
	}

	redis := cache.GetRedisClient()

	_, err = redis.LPush(models.AuditLogBackfillRedisList, marshaledData).Result()
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
		Value: strings.Join(emoji.Roles, ";"),
		Type:  models.EventlogTargetTypeRole,
	})

	if iconObjectOption.Value != "" {
		options = append(options, iconObjectOption)
	}

	added, err := EventlogLog(leftAt, guildID, emoji.ID, models.EventlogTargetTypeEmoji, "", models.EventlogTypeEmojiCreate, "", nil, options, true)
	RelaxLog(err)
	if added {
		err := RequestAuditLogBackfill(guildID, models.AuditLogBackfillTypeEmojiCreate, "")
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
		Value: strings.Join(emoji.Roles, ";"),
		Type:  models.EventlogTargetTypeRole,
	})

	if iconObjectOption.Value != "" {
		options = append(options, iconObjectOption)
	}

	added, err := EventlogLog(leftAt, guildID, emoji.ID, models.EventlogTargetTypeEmoji, "", models.EventlogTypeEmojiDelete, "", nil, options, true)
	RelaxLog(err)
	if added {
		err := RequestAuditLogBackfill(guildID, models.AuditLogBackfillTypeEmojiDelete, "")
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
		Value: strings.Join(newEmoji.Roles, ";"),
		Type:  models.EventlogTargetTypeRole,
	})

	rolesAdded, rolesRemoved := StringSliceDiff(oldEmoji.Roles, newEmoji.Roles)
	if len(rolesAdded) >= 0 {
		options = append(options, models.ElasticEventlogOption{
			Key:   "emoji_roleids_added",
			Value: strings.Join(rolesAdded, ";"),
			Type:  models.EventlogTargetTypeRole,
		})
	}
	if len(rolesRemoved) >= 0 {
		options = append(options, models.ElasticEventlogOption{
			Key:   "emoji_roleids_removed",
			Value: strings.Join(rolesRemoved, ";"),
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
			OldValue: strings.Join(oldEmoji.Roles, ";"),
			NewValue: strings.Join(newEmoji.Roles, ";"),
			Type:     models.EventlogTargetTypeRole,
		})
	}

	added, err := EventlogLog(leftAt, guildID, newEmoji.ID, models.EventlogTargetTypeEmoji, "", models.EventlogTypeEmojiUpdate, "", changes, options, true)
	RelaxLog(err)
	if added {
		err := RequestAuditLogBackfill(guildID, models.AuditLogBackfillTypeEmojiUpdate, "")
		RelaxLog(err)
	}
}

func getDiscordFileHashOption(key, hash, hashURL, userID, channelID, guildID string) models.ElasticEventlogOption {
	iconObjectOption := models.ElasticEventlogOption{
		Key:  key,
		Type: models.EventlogTargetTypeRobyulPublicObject,
	}

	// get icon object
	iconObjectOption.Value, _ = getDiscordFileHashObject(key, hash, hashURL, userID, channelID, guildID)

	return iconObjectOption
}

func getDiscordFileHashObject(key, hash, hashURL, userID, channelID, guildID string) (objectName string, err error) {
	metadataKey := "discord_" + key
	// try to get icon from object storage
	oldObjects, _ := RetrieveFilesByAdditionalObjectMetadata(metadataKey, hash)
	if oldObjects != nil && len(oldObjects) >= 1 {
		return oldObjects[0], nil
	} else {
		// try to download old icon if not found in object storage
		oldGuildIconData, err := NetGetUAWithError(hashURL, DEFAULT_UA)
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
				return objectName, nil
			}
		}
	}

	return "", errors.New("unable to get object for file hash")
}

func getDiscordFileHashChange(key, oldHash, newHash, oldHashUrl, newHashUrl, userID, channelID, guildID string) models.ElasticEventlogChange {
	iconObjectChange := models.ElasticEventlogChange{
		Key:  key,
		Type: models.EventlogTargetTypeRobyulPublicObject,
	}

	// try to get old icon
	iconObjectChange.OldValue, _ = getDiscordFileHashObject(key, oldHash, oldHashUrl, userID, channelID, guildID)

	// try to get new icon
	iconObjectChange.NewValue, _ = getDiscordFileHashObject(key, newHash, newHashUrl, userID, channelID, guildID)

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

	/*
		sent with every first update
		if oldGuild.EmbedChannelID != newGuild.EmbedChannelID {
			changes = append(changes, models.ElasticEventlogChange{
				Key:      "guild_embedchannelid",
				OldValue: oldGuild.EmbedChannelID,
				NewValue: newGuild.EmbedChannelID,
				Type:     models.EventlogTargetTypeChannel,
			})
		}
	*/

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

	/*
		sent with every first update
		if oldGuild.EmbedEnabled != newGuild.EmbedEnabled {
			changes = append(changes, models.ElasticEventlogChange{
				Key:      "guild_embedenabled",
				OldValue: StoreBoolAsString(oldGuild.EmbedEnabled),
				NewValue: StoreBoolAsString(newGuild.EmbedEnabled),
			})
		}
	*/

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
		err := RequestAuditLogBackfill(guildID, models.AuditLogBackfillTypeGuildUpdate, "")
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
		var oldOverwrites, newOverwrites string
		for _, oldOverwrite := range oldChannel.PermissionOverwrites {
			oldOverwriteText, err := jsoniter.MarshalToString(oldOverwrite)
			RelaxLog(err)
			if err == nil {
				oldOverwrites += oldOverwriteText + ";"
			}
		}
		oldOverwrites = strings.TrimRight(oldOverwrites, ";")
		for _, newOverwrite := range newChannel.PermissionOverwrites {
			newOverwriteText, err := jsoniter.MarshalToString(newOverwrite)
			RelaxLog(err)
			if err == nil {
				newOverwrites += newOverwriteText + ";"
			}
		}
		newOverwrites = strings.TrimRight(newOverwrites, ";")
		if oldOverwrites != "" && newOverwrites != "" {
			changes = append(changes, models.ElasticEventlogChange{
				Key:      "channel_permissionoverwrites",
				OldValue: oldOverwrites,
				NewValue: newOverwrites,
				Type:     models.EventlogTargetTypePermissionOverwrite,
			})
			backfill = true
		}
	}

	added, err := EventlogLog(leftAt, guildID, newChannel.ID, models.EventlogTargetTypeChannel, "", models.EventlogTypeChannelUpdate, "", changes, nil, backfill)
	RelaxLog(err)
	if added && backfill {
		err := RequestAuditLogBackfill(guildID, models.AuditLogBackfillTypeChannelUpdate, "")
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
			OldValue: strings.Join(oldMember.Roles, ";"),
			NewValue: strings.Join(newMember.Roles, ";"),
			Type:     models.EventlogTargetTypeRole,
		})

		if len(rolesAdded) > 0 {
			options = append(options, models.ElasticEventlogOption{
				Key:   "member_roles_added",
				Value: strings.Join(rolesAdded, ";"),
				Type:  models.EventlogTargetTypeRole,
			})
		}

		if len(rolesRemoved) > 0 {
			options = append(options, models.ElasticEventlogOption{
				Key:   "member_roles_removed",
				Value: strings.Join(rolesRemoved, ";"),
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
			err := RequestAuditLogBackfill(guildID, models.AuditlogBackfillTypeMemberRoleUpdate, "")
			RelaxLog(err)
		}
		if memberUpdateBackfill {
			err := RequestAuditLogBackfill(guildID, models.AuditlogBackfillTypeMemberUpdate, "")
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
			Type:     models.EventlogTargetTypeRolePermissions,
		})
	}

	added, err := EventlogLog(leftAt, guildID, newRole.ID, models.EventlogTargetTypeRole, "", models.EventlogTypeRoleUpdate, "", changes, nil, true)
	RelaxLog(err)
	if added {
		err := RequestAuditLogBackfill(guildID, models.AuditLogBackfillTypeRoleUpdate, "")
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

func GetStringAsBool(input string) bool {
	if input == "yes" {
		return true
	}
	return false
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
