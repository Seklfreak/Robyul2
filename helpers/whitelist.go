package helpers

import (
	"github.com/Seklfreak/Robyul2/models"
)

var whitelistCache map[string]bool

func UpdateWhitelistCache() error {
	var entryBucket []models.AutoleaverWhitelistEntry
	err := MDbIter(MdbCollection(models.AutoleaverWhitelistTable).Find(nil)).All(&entryBucket)
	if err != nil {
		return err
	}

	newCache := make(map[string]bool)
	for _, whitelistEntry := range entryBucket {
		newCache[whitelistEntry.GuildID] = true
	}

	whitelistCache = newCache
	return nil
}

func GuildIsOnWhitelist(guildID string) bool {
	if whitelistCache == nil {
		return true
	}

	return whitelistCache[guildID]
}
