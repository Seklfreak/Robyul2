package nugugame

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"
)

// log is just a small helper function for logging in this module
func log() *logrus.Entry {
	return cache.GetLogger().WithField("module", "nugugame")
}

// getIdolCache for easily getting redis cache specific to idols
func getModuleCache(key string, data interface{}) error {
	// get cache with given key
	cacheResult, err := cache.GetRedisClient().Get(fmt.Sprintf("robyul2-discord:idols:%s", key)).Bytes()
	if err != nil || err == redis.Nil {
		return err
	}

	// if the datas type is already []byte then set it to cache instead of unmarshal
	switch data.(type) {
	case []byte:
		data = cacheResult
		return nil
	}

	err = json.Unmarshal(cacheResult, data)
	return err
}

// setIdolsCache for easily setting redis cache specific to idols
func setModuleCache(key string, data interface{}, time time.Duration) error {
	marshaledData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = cache.GetRedisClient().Set(fmt.Sprintf("robyul2-discord:idols:%s", key), marshaledData, time).Result()
	return err
}

// checks if the error is a permissions error and notifies the user
func checkPermissionError(err error, channelID string) bool {
	if err == nil {
		return false
	}

	// check if error is a permissions error
	if err, ok := err.(*discordgo.RESTError); ok && err.Message != nil {
		if err.Message.Code == discordgo.ErrCodeMissingPermissions {
			return true
		}
	}
	return false
}
