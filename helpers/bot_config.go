package helpers

import (
	"errors"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/globalsign/mgo/bson"
	"github.com/vmihailenco/msgpack"
)

func getBotConfigEntry(key string) (entry models.BotConfigEntry, err error) {
	err = MdbOne(
		MdbCollection(models.BotConfigTable).Find(bson.M{"key": key}),
		&entry,
	)
	return entry, err
}

func createBotConfigEntry(key string, value []byte) (err error) {
	_, err = MDbInsert(
		models.BotConfigTable,
		models.BotConfigEntry{
			Key:         key,
			Value:       value,
			LastChanged: time.Now(),
		},
	)
	return err
}

func updateBotConfigEntry(entry models.BotConfigEntry) (err error) {
	if entry.ID.Valid() {
		err = MDbUpdate(models.BotConfigTable, entry.ID, entry)
	}
	return err
}

func SetBotConfig(key string, value interface{}) (err error) {
	if len(key) <= 0 {
		return errors.New("invalid key")
	}

	b, err := msgpack.Marshal(value)
	if err != nil {
		return err
	}

	entry, err := getBotConfigEntry(key)
	if err != nil {
		cache.GetLogger().WithField("Module", "bot_config").Infof("Creating Bot Config Entry: %s Value: %#v", key, value)
		return createBotConfigEntry(key, b)
	}
	entry.Value = b
	entry.LastChanged = time.Now()
	cache.GetLogger().WithField("Module", "bot_config").Infof("Updating Bot Config Entry: %s Value: %#v", key, value)
	return updateBotConfigEntry(entry)
}

// TODO: cache
func GetBotConfig(key string, value interface{}) (err error) {
	entry, err := getBotConfigEntry(key)
	if err != nil {
		return err
	}

	err = msgpack.Unmarshal(entry.Value, &value)
	return err
}

func SetBotConfigString(key, value string) (err error) {
	return SetBotConfig(key, value)
}

func GetBotConfigString(key string) (value string, err error) {
	err = GetBotConfig(key, &value)
	return value, err
}
