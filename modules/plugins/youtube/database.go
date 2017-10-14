package youtube

import (
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	rethink "github.com/gorethink/gorethink"
)

// RethinkDB CRUD wrapper functions.

func createEntry(entry models.YoutubeChannelEntry) (id string, err error) {
	query := rethink.Table(models.YoutubeChannelTable).Insert(entry)

	res, err := query.RunWrite(helpers.GetDB())
	if err != nil {
		return "", err
	}

	return res.GeneratedKeys[0], nil
}

func readEntries(filter interface{}) (entry []models.YoutubeChannelEntry, err error) {
	query := rethink.Table(models.YoutubeChannelTable).Filter(filter)

	cursor, err := query.Run(helpers.GetDB())
	if err != nil {
		return entry, err
	}
	defer cursor.Close()

	err = cursor.All(&entry)
	return
}

func updateEntry(entry models.YoutubeChannelEntry) (err error) {
	query := rethink.Table(models.YoutubeChannelTable).Update(entry)

	_, err = query.Run(helpers.GetDB())
	return
}

func deleteEntry(id string) (n int, err error) {
	query := rethink.Table(models.YoutubeChannelTable).Filter(rethink.Row.Field("id").Eq(id)).Delete()

	r, err := query.RunWrite(helpers.GetDB())
	if err == nil {
		n = r.Deleted
	}

	return
}
