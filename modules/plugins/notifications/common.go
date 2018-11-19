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

func keywordMatches(message, keyword string) bool {
	// early match if message is keyword
	if message == keyword {
		return true
	}

	// early skip if word is not found
	if !strings.Contains(message, keyword) {
		return false
	}

	var lookup strings.Builder

	// check common cases first
	// spaces around word
	lookup.WriteString(" ")
	lookup.WriteString(keyword)
	lookup.WriteString(" ")
	if strings.Contains(message, lookup.String()) {
		return true
	}

	// spaces after word
	lookup.Reset()
	lookup.WriteString(keyword)
	lookup.WriteString(" ")
	if strings.HasPrefix(message, lookup.String()) {
		return true
	}

	// check delimiter around word
	for _, combination := range generatedDelimiterCombinations {
		lookup.Reset()
		lookup.WriteString(combination.Start)
		lookup.WriteString(keyword)
		lookup.WriteString(combination.End)

		if strings.Contains(message, lookup.String()) {
			return true
		}
	}
	// check delimiter after word
	for _, delimiter := range ValidTextDelimiters {
		lookup.Reset()
		lookup.WriteString(keyword)
		lookup.WriteString(delimiter)

		if strings.HasPrefix(message, lookup.String()) {
			return true
		}
	}
	// check delimiter before word
	for _, delimiter := range ValidTextDelimiters {
		lookup.Reset()
		lookup.WriteString(delimiter)
		lookup.WriteString(keyword)

		if strings.HasSuffix(message, lookup.String()) {
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
