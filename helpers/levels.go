package helpers

func GetMaxBadgesForGuild(guildID string) (maxBadges int) {
	maxBadges = GuildSettingsGetCached(guildID).LevelsMaxBadges
	if maxBadges == 0 {
		maxBadges = 100
	}
	if maxBadges < 0 {
		maxBadges = 0
	}
	return maxBadges
}
