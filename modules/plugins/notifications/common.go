package notifications

import (
	"strings"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
)

func getAllDelimiterCombinations() []delimiterCombination {
	var result []delimiterCombination
	for _, delimiterStart := range ValidTextDelimiters {
		for _, delimiterEnd := range ValidTextDelimiters {
			result = append(result, delimiterCombination{
				Start: delimiterStart,
				End:   delimiterEnd,
			})
		}
	}
	return result
}

func keywordMatches(message, keyword string) bool {
	message = strings.ToLower(strings.TrimSpace(message))

	if message == keyword {
		return true
	}
	for _, combination := range generatedDelimiterCombinations {
		if strings.Contains(message, combination.Start+keyword+combination.End) {
			return true
		}
	}
	for _, delimiter := range ValidTextDelimiters {
		if strings.HasPrefix(message, keyword+delimiter) {
			return true
		}
	}
	for _, delimiter := range ValidTextDelimiters {
		if strings.HasSuffix(message, delimiter+keyword) {
			return true
		}
	}

	return false
}

func refreshNotificationSettingsCache() (err error) {
	var temporaryNotificationSettingsCache []*models.NotificationsEntry
	err = helpers.MDbIter(helpers.MdbCollection(models.NotificationsTable).Find(nil)).All(&temporaryNotificationSettingsCache)
	if err != nil {
		return err
	}
	for i := range temporaryNotificationSettingsCache {
		temporaryNotificationSettingsCache[i].Keyword = strings.ToLower(
			temporaryNotificationSettingsCache[i].Keyword,
		)
	}
	notificationSettingsCache = temporaryNotificationSettingsCache

	err = helpers.MDbIter(helpers.MdbCollection(models.NotificationsIgnoredChannelsTable).Find(nil)).All(&ignoredChannelsCache)
	if err != nil {
		return err
	}

	return nil
}
