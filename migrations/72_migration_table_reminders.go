package migrations

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/globalsign/mgo/bson"
	"github.com/gorethink/gorethink"
)

func m72_migration_table_reminders() {
	if !TableExists("reminders") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving reminders to mongodb")

	cursor, err := gorethink.Table("reminders").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("reminders").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	type rethinkdbSubEntry struct {
		Message   string `gorethink:"message"`
		ChannelID string `gorethink:"channelID"`
		GuildID   string `gorethink:"guildID"`
		Timestamp int64  `gorethink:"timestamp"`
	}
	var rethinkdbEntry struct {
		Id        string              `gorethink:"id,omitempty"`
		UserID    string              `gorethink:"userid"`
		Reminders []rethinkdbSubEntry `gorethink:"reminders"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		reminders := make([]models.RemindersReminderEntry, 0)
		for _, oldReminder := range rethinkdbEntry.Reminders {
			reminders = append(reminders, models.RemindersReminderEntry{
				Message:   oldReminder.Message,
				ChannelID: oldReminder.ChannelID,
				GuildID:   oldReminder.GuildID,
				Timestamp: oldReminder.Timestamp,
			})
		}

		err = helpers.MDbUpsertWithoutLogging(
			models.RemindersTable,
			bson.M{"userid": rethinkdbEntry.UserID},
			models.RemindersEntry{
				UserID:    rethinkdbEntry.UserID,
				Reminders: reminders,
			},
		)
		if err != nil {
			panic(err)
		}

		bar.Increment()
	}

	if cursor.Err() != nil {
		panic(err)
	}
	bar.Finish()

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb reminders")
	_, err = gorethink.TableDrop("reminders").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
