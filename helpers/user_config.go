package helpers

import (
	"errors"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
	rethink "github.com/gorethink/gorethink"
	"github.com/vmihailenco/msgpack"
)

func getUserConfigEntry(userID, key string) (entry models.UserConfigEntry, err error) {
	listCursor, err := rethink.Table(models.UserConfigTable).Filter(
		rethink.Row.Field("user_id").Eq(userID),
	).Filter(
		rethink.Row.Field("key").Eq(key),
	).Run(GetDB())
	if err != nil {
		return entry, err
	}

	defer listCursor.Close()
	err = listCursor.One(&entry)
	return entry, err
}

func createUserConfigEntry(userID, key string, value []byte) (err error) {
	insert := rethink.Table(models.UserConfigTable).Insert(models.UserConfigEntry{
		UserID:      userID,
		Key:         key,
		Value:       value,
		LastChanged: time.Now(),
	})
	_, err = insert.RunWrite(GetDB())
	return err
}

func updateUserConfigEntry(entry models.UserConfigEntry) (err error) {
	if entry.ID != "" {
		_, err = rethink.Table(models.UserConfigTable).Get(entry.ID).Update(entry).RunWrite(GetDB())
	}
	return err
}

func SetUserConfig(userID, key string, value interface{}) (err error) {
	if len(key) <= 0 {
		return errors.New("invalid key")
	}

	b, err := msgpack.Marshal(value)
	if err != nil {
		return err
	}

	entry, err := getUserConfigEntry(userID, key)
	if err != nil {
		cache.GetLogger().WithField("Module", "user_config").Infof("Creating User Config Entry: %s User: %s Value: %#v", key, userID, value)
		return createUserConfigEntry(userID, key, b)
	}
	entry.Value = b
	entry.LastChanged = time.Now()
	cache.GetLogger().WithField("Module", "user_config").Infof("Updating User Config Entry: %s User: %s Value: %#v", key, userID, value)
	return updateUserConfigEntry(entry)
}

// TODO: cache
func GetUserConfig(userID, key string, value interface{}) (err error) {
	entry, err := getUserConfigEntry(userID, key)
	if err != nil {
		return err
	}

	err = msgpack.Unmarshal(entry.Value, &value)
	return err
}

func SetUserConfigString(userID, key, value string) (err error) {
	return SetUserConfig(userID, key, value)
}

func GetUserConfigString(userID, key, placeholder string) (value string) {
	err := GetUserConfig(userID, key, &value)
	if err != nil {
		return placeholder
	}
	return value
}

func SetUserConfigInt(userID, key string, value int) (err error) {
	return SetUserConfig(userID, key, value)
}

func GetUserConfigInt(userID, key string, placeholder int) (value int) {
	err := GetUserConfig(userID, key, &value)
	if err != nil {
		return placeholder
	}
	return value
}
