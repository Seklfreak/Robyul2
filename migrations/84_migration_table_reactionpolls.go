package migrations

import (
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m84_migration_table_reactionpolls() {
	if !TableExists("reactionpolls") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving reactionpolls to mongodb")

	cursor, err := gorethink.Table("reactionpolls").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("reactionpolls").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID              string    `gorethink:"id,omitempty"`
		Text            string    `gorethink:"text"`
		MessageID       string    `gorethink:"messageid"`
		ChannelID       string    `gorethink:"channelid"`
		GuildID         string    `gorethink:"guildid"`
		CreatedByUserID string    `gorethink:"createdby_userid"`
		CreatedAt       time.Time `gorethink:"createdat"`
		Active          bool      `gorethinK:"active"`
		AllowedEmotes   []string  `gorethink:"allowedemotes"`
		MaxAllowedVotes int       `gorethink:"maxallowedemotes"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		_, err = helpers.MDbInsertWithoutLogging(
			models.ReactionpollsTable,
			models.ReactionpollsEntry{
				Text:            rethinkdbEntry.Text,
				MessageID:       rethinkdbEntry.MessageID,
				ChannelID:       rethinkdbEntry.ChannelID,
				GuildID:         rethinkdbEntry.GuildID,
				CreatedByUserID: rethinkdbEntry.CreatedByUserID,
				CreatedAt:       rethinkdbEntry.CreatedAt,
				Active:          rethinkdbEntry.Active,
				AllowedEmotes:   rethinkdbEntry.AllowedEmotes,
				MaxAllowedVotes: rethinkdbEntry.MaxAllowedVotes,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb reactionpolls")
	_, err = gorethink.TableDrop("reactionpolls").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
