package helpers

import (
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
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
