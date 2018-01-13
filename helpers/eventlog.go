package helpers

import (
	"time"

	"sync"

	"strconv"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
)

var (
	AuditLogBackfillRequestsLock = sync.Mutex{}
)

func EventlogLog(createdAt time.Time, guildID, targetID, targetType, userID, actionType, reason string,
	changes []models.ElasticEventlogChange, options []models.ElasticEventlogOption, waitingForAuditLogBackfill bool) (err error) {
	if guildID == "" {
		return nil
	}

	if IsBlacklistedGuild(guildID) {
		return nil
	}

	if IsLimitedGuild(guildID) {
		return nil
	}

	if GuildSettingsGetCached(guildID).EventlogDisabled {
		return nil
	}

	if changes == nil {
		changes = make([]models.ElasticEventlogChange, 0)
	}

	if options == nil {
		options = make([]models.ElasticEventlogOption, 0)
	}

	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	// TODO: remove me
	cache.GetLogger().WithField("module", "helpers/eventlog").Debugf(
		"adding to eventlog time %s guildID %s targetID %s userID %s actionType %s reason %s changes %+v options %+v",
		createdAt.Format(time.RFC3339), guildID, targetID, userID, actionType, reason, changes, options,
	)
	ElasticAddEventlog(createdAt, guildID, targetID, targetType, userID, actionType, reason, changes, options, waitingForAuditLogBackfill)

	return
}

type AuditLogBackfillType int

const (
	AuditLogBackfillTypeChannelCreate AuditLogBackfillType = 1 << iota
	AuditLogBackfillTypeChannelDelete
	AuditLogBackfillTypeRoleCreate
	AuditLogBackfillTypeRoleDelete
	AuditLogBackfillTypeBanAdd
	AuditLogBackfillTypeBanRemove
	AuditLogBackfillTypeMemberRemove
	AuditLogBackfillTypeEmojiCreate
	AuditLogBackfillTypeEmojiDelete
	AuditLogBackfillTypeEmojiUpdate
	AuditLogBackfillTypeGuildUpdate
)

func RequestAuditLogBackfill(guildID string, backfillType AuditLogBackfillType) (err error) {
	AuditLogBackfillRequestsLock.Lock()
	defer AuditLogBackfillRequestsLock.Unlock()

	redis := cache.GetRedisClient()

	switch backfillType {
	case AuditLogBackfillTypeChannelCreate:
		cache.GetLogger().Infof("requested backfill for %s: %s", guildID, "channel create")
		_, err := redis.SAdd(models.AuditLogBackfillTypeChannelCreateRedisSet, guildID).Result()
		return err
	case AuditLogBackfillTypeChannelDelete:
		cache.GetLogger().Infof("requested backfill for %s: %s", guildID, "channel delete")
		_, err := redis.SAdd(models.AuditLogBackfillTypeChannelDeleteRedisSet, guildID).Result()
		return err
	case AuditLogBackfillTypeRoleCreate:
		cache.GetLogger().Infof("requested backfill for %s: %s", guildID, "role create")
		_, err := redis.SAdd(models.AuditLogBackfillTypeRoleCreateRedisSet, guildID).Result()
		return err
	case AuditLogBackfillTypeRoleDelete:
		cache.GetLogger().Infof("requested backfill for %s: %s", guildID, "role delete")
		_, err := redis.SAdd(models.AuditLogBackfillTypeRoleDeleteRedisSet, guildID).Result()
		return err
	case AuditLogBackfillTypeBanAdd:
		cache.GetLogger().Infof("requested backfill for %s: %s", guildID, "ban add")
		_, err := redis.SAdd(models.AuditLogBackfillTypeBanAddRedisSet, guildID).Result()
		return err
	case AuditLogBackfillTypeBanRemove:
		cache.GetLogger().Infof("requested backfill for %s: %s", guildID, "ban remove")
		_, err := redis.SAdd(models.AuditLogBackfillTypeBanRemoveRedisSet, guildID).Result()
		return err
	case AuditLogBackfillTypeMemberRemove:
		cache.GetLogger().Infof("requested backfill for %s: %s", guildID, "member remove")
		_, err := redis.SAdd(models.AuditLogBackfillTypeMemberRemoveRedisSet, guildID).Result()
		return err
	case AuditLogBackfillTypeEmojiCreate:
		cache.GetLogger().Infof("requested backfill for %s: %s", guildID, "emoji create")
		_, err := redis.SAdd(models.AuditLogBackfillTypeEmojiCreateRedisSet, guildID).Result()
		return err
	case AuditLogBackfillTypeEmojiDelete:
		cache.GetLogger().Infof("requested backfill for %s: %s", guildID, "emoji delete")
		_, err := redis.SAdd(models.AuditLogBackfillTypeEmojiDeleteRedisSet, guildID).Result()
		return err
	case AuditLogBackfillTypeEmojiUpdate:
		cache.GetLogger().Infof("requested backfill for %s: %s", guildID, "emoji update")
		_, err := redis.SAdd(models.AuditLogBackfillTypeEmojiUpdateRedisSet, guildID).Result()
		return err
	case AuditLogBackfillTypeGuildUpdate:
		cache.GetLogger().Infof("requested backfill for %s: %s", guildID, "guild update")
		_, err := redis.SAdd(models.AuditLogBackfillTypeGuildUpdateRedisSet, guildID).Result()
		return err
	}
	return errors.New("unknown backfill type")
}

func OnEmojiCreate(guildID string, emoji *discordgo.Emoji) {
	leftAt := time.Now()

	options := make([]models.ElasticEventlogOption, 0)

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

	EventlogLog(leftAt, guildID, emoji.ID, models.EventlogTargetTypeEmoji, "", models.EventlogTypeEmojiCreate, "", nil, options, true)

	err := RequestAuditLogBackfill(guildID, AuditLogBackfillTypeEmojiCreate)
	RelaxLog(err)
}

func OnEmojiDelete(guildID string, emoji *discordgo.Emoji) {
	leftAt := time.Now()

	options := make([]models.ElasticEventlogOption, 0)

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

	EventlogLog(leftAt, guildID, emoji.ID, models.EventlogTargetTypeEmoji, "", models.EventlogTypeEmojiDelete, "", nil, options, true)

	err := RequestAuditLogBackfill(guildID, AuditLogBackfillTypeEmojiDelete)
	RelaxLog(err)
}

func OnEmojiUpdate(guildID string, oldEmoji, newEmoji *discordgo.Emoji) {
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

	EventlogLog(leftAt, guildID, newEmoji.ID, models.EventlogTargetTypeEmoji, "", models.EventlogTypeEmojiUpdate, "", changes, options, true)

	err := RequestAuditLogBackfill(guildID, AuditLogBackfillTypeEmojiUpdate)
	RelaxLog(err)
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
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "guild_icon",
			OldValue: oldGuild.Icon,
			NewValue: newGuild.Icon,
		})
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
		})
	}

	if oldGuild.EmbedChannelID != newGuild.EmbedChannelID {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "guild_embedchannelid",
			OldValue: oldGuild.EmbedChannelID,
			NewValue: newGuild.EmbedChannelID,
		})
	}

	if oldGuild.OwnerID != newGuild.OwnerID {
		changes = append(changes, models.ElasticEventlogChange{
			Key:      "guild_ownerid",
			OldValue: oldGuild.OwnerID,
			NewValue: newGuild.OwnerID,
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

	EventlogLog(leftAt, guildID, newGuild.ID, models.EventlogTargetTypeGuild, "", models.EventlogTypeGuildUpdate, "", changes, nil, true)

	err := RequestAuditLogBackfill(guildID, AuditLogBackfillTypeGuildUpdate)
	RelaxLog(err)
}

func StoreBoolAsString(input bool) (output string) {
	if input {
		return "yes"
	} else {
		return "no"
	}
}
