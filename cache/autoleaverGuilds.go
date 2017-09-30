package cache

import (
	"sync"
)

var (
	guildIDs      []string
	guildIDsMutex sync.RWMutex
)

func AddAutoleaverGuildID(s string) bool {
	guildIDsMutex.Lock()
	defer guildIDsMutex.Unlock()
	for _, guildID := range guildIDs {
		if guildID == s {
			return false
		}
	}

	guildIDs = append(guildIDs, s)
	return true
}

func RemoveAutoleaverGuildID(s string) bool {
	guildIDsMutex.Lock()
	defer guildIDsMutex.Unlock()
	otherGuilds := make([]string, 0)
	guildRemoved := false
	for _, guildID := range guildIDs {
		if guildID != s {
			otherGuilds = append(otherGuilds, guildID)
		} else {
			guildRemoved = true
		}
	}

	guildIDs = otherGuilds
	return guildRemoved
}

func GetAutoleaverGuildIDs() []string {
	guildIDsMutex.RLock()
	defer guildIDsMutex.RUnlock()

	return guildIDs
}
