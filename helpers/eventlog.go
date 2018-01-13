package helpers

import (
	"time"

	"sync"

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
	}
	return errors.New("unknown backfill type")
}

func OnEmojiCreate(guildID string, emoji *discordgo.Emoji) {
	go func() {
		defer Recover()

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
	}()
}

func OnEmojiDelete(guildID string, emoji *discordgo.Emoji) {
	go func() {
		defer Recover()

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
	}()
}

func OnEmojiUpdate(guildID string, oldEmoji, newEmoji *discordgo.Emoji) {
	go func() {
		defer Recover()

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
	}()
}

func StoreBoolAsString(input bool) (output string) {
	if input {
		return "yes"
	} else {
		return "no"
	}
}
