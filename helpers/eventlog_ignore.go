package helpers

import (
	"time"

	"strings"

	"github.com/Seklfreak/Robyul2/models"
)

func eventlogEventIsIgnored(createdAt time.Time, guildID, targetID, targetType, userID, actionType, reason string,
	changes []models.ElasticEventlogChange, options []models.ElasticEventlogOption, waitingForAuditLogBackfill bool) bool {
	// ignore music bot channel description update
	if actionType == models.EventlogTypeChannelUpdate &&
		len(options) == 0 &&
		len(changes) == 1 &&
		changes[0].Key == "channel_topic" &&
		((strings.Contains(changes[0].NewValue, "▶") || strings.Contains(changes[0].NewValue, "⏹")) &&
			(strings.Contains(changes[0].NewValue, "🔇") ||
				strings.Contains(changes[0].NewValue, "🔈") ||
				strings.Contains(changes[0].NewValue, "🔉") ||
				strings.Contains(changes[0].NewValue, "🔊"))) {
		return true
	}

	return false
}
