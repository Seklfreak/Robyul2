package notifications

import "github.com/Seklfreak/Robyul2/models"

var (
	notificationSettingsCache []*models.NotificationsEntry
	ignoredChannelsCache      []models.NotificationsIgnoredChannelsEntry
	ValidTextDelimiters       = []string{" ", ".", ",", "?", "!", ";", "(", ")", "=", "\"", "'", "`", "´", "_", "~", "+", "-", "/", ":", "*", "\n", "…", "’", "“", "‘"}
	WhitelistedBotIDs         = []string{
		"430101373397368842", // Test Webhook (Sekl)

		"178215222614556673", // Fiscord-IRC (Kakkela)
		"232927528325611521", // TrelleIRC (Kakkela)

		"309026207104761858", // Fiscord (Kakkela, Webhook)
		"426398711896080384", // Fiscord (Kakkela, Webhook)
		"308942631696859148", // TrelleIRC (Kakkela, Webhook)
		"426685461793341451", // TrelleIRC (Kakkela, Webhook)
		"308942526570561536", // TrelleIRC (Kakkela, Webhook)
		"430089364417150976", // TrelleIRC (Kakkela, Webhook)
	}
	generatedDelimiterCombinations = getAllDelimiterCombinations()
)

const (
	UserConfigNotificationsLayoutModeKey = "notifications:layout-mode"
)
