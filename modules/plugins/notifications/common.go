package notifications

import (
	"strings"

	"github.com/bwmarrin/discordgo"

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

func keywordMatches(message string, keyword []byte) bool {
	var lookup strings.Builder
	lookup.Write(keyword)

	if message == lookup.String() {
		return true
	}

	for _, combination := range generatedDelimiterCombinations {
		lookup.Reset()
		lookup.Write(combination.Start)
		lookup.Write(keyword)
		lookup.Write(combination.End)

		if strings.Contains(message, lookup.String()) {
			return true
		}
	}
	for _, delimiter := range ValidTextDelimiters {
		lookup.Reset()
		lookup.Write(keyword)
		lookup.Write(delimiter)

		if strings.HasPrefix(message, lookup.String()) {
			return true
		}
	}
	for _, delimiter := range ValidTextDelimiters {
		lookup.Reset()
		lookup.Write(delimiter)
		lookup.Write(keyword)

		if strings.HasSuffix(message, lookup.String()) {
			return true
		}
	}

	return false
}

func refreshNotificationSettingsCache() error {
	var bucket []*models.NotificationsEntry
	err := helpers.MDbIter(helpers.MdbCollection(models.NotificationsTable).Find(nil)).All(&bucket)
	if err != nil {
		return err
	}
	temporaryNotificationSettingsCache := make([]*entryWithBytes, len(bucket))
	for i := range bucket {
		temporaryNotificationSettingsCache[i] = &entryWithBytes{}
		temporaryNotificationSettingsCache[i].NotificationsEntry = bucket[i]
		temporaryNotificationSettingsCache[i].Keyword = strings.ToLower(
			temporaryNotificationSettingsCache[i].Keyword,
		)
		temporaryNotificationSettingsCache[i].KeywordBytes = []byte(
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

func asyncRefresh() {
	go func() {
		defer helpers.Recover()

		err := refreshNotificationSettingsCache()
		helpers.RelaxLog(err)
	}()
}

func isIgnored(entry *models.NotificationsEntry, msg *discordgo.Message) bool {
	// ignore messages by the notification setting author
	if entry.UserID == msg.Author.ID {
		return true
	}

	// ignore message if in ignored guilds for global keyword
	if len(entry.IgnoredGuildIDs) > 0 {
		for _, ignoredGuildID := range entry.IgnoredGuildIDs {
			if ignoredGuildID == msg.GuildID {
				return true
			}
		}
	}

	// ignore message if in ignored channels for keyword
	if len(entry.IgnoredChannelIDs) > 0 {
		for _, ignoredChannelID := range entry.IgnoredChannelIDs {
			if ignoredChannelID == msg.ChannelID {
				return true
			}
		}
	}

	return false
}
