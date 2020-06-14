package helpers

import (
	"sync"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
	raven "github.com/getsentry/raven-go"
	"github.com/globalsign/mgo/bson"
)

var (
	guildSettingsCache = make(map[string]models.Config)
	cacheMutex         sync.RWMutex
)

// GuildSettingsSet writes all $config into the db
func GuildSettingsSet(guild string, config models.Config) error {
	// Check if an config object exists
	var settings models.Config

	err := MdbOneWithoutLogging(
		MdbCollection(models.GuildConfigTable).Find(bson.M{"guildid": guild}),
		&settings,
	)

	if IsMdbNotFound(err) {
		_, err = MDbInsert(
			models.GuildConfigTable,
			config,
		)
	} else if err != nil {
		return err
	} else {
		err = MDbUpdate(models.GuildConfigTable, config.ID, config)
	}
	if err != nil {
		return err
	}

	// Update cache
	cacheMutex.Lock()
	guildSettingsCache[guild] = config
	cacheMutex.Unlock()

	return err
}

// GuildSettingsGet returns all config values for the guild or a default object
func GuildSettingsGet(guild string) (models.Config, error) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	var settings models.Config
	var err error

	err = MdbOneWithoutLogging(
		MdbCollection(models.GuildConfigTable).Find(bson.M{"guildid": guild}),
		&settings,
	)

	if IsMdbNotFound(err) {
		settings = models.Config{}.Default(guild)
		settings.GuildID = guild
		guildSettingsCache[guild] = settings
		return settings, nil
	}

	guildSettingsCache[guild] = settings
	return settings, err
}

func GuildSettingsGetCached(id string) models.Config {
	cacheMutex.RLock()
	settings := guildSettingsCache[id]
	cacheMutex.RUnlock()

	if settings.GuildID == "" {
		settings, _ = GuildSettingsGet(id)
	}
	return settings
}

// GetPrefixForServer gets the prefix for $guild
func GetPrefixForServer(guildID string) string {
	return GuildSettingsGetCached(guildID).Prefix
}

// SetPrefixForServer sets the prefix for $guild to $prefix
func SetPrefixForServer(guild string, prefix string) error {
	settings := GuildSettingsGetCached(guild)

	settings.Prefix = prefix

	return GuildSettingsSet(guild, settings)
}

func GuildSettingsUpdater() {
	for {
		for _, shard := range cache.GetSession().Sessions {
			for _, guild := range shard.State.Guilds {
				_, e := GuildSettingsGet(guild.ID)
				if e != nil {
					raven.CaptureError(e, map[string]string{})
					continue
				}

				time.Sleep(100 * time.Millisecond)
			}
		}

		time.Sleep(15 * time.Second)
	}
}
