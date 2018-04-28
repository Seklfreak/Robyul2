package migrations

import (
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m89_migration_table_starboard_entries() {
	if !TableExists("starboard_entries") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving starboard_entries to mongodb")

	cursor, err := gorethink.Table("starboard_entries").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("starboard_entries").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID                        string    `rethink:"id,omitempty"`
		GuildID                   string    `rethink:"guild_id"`
		MessageID                 string    `rethink:"message_id"`
		ChannelID                 string    `rethink:"channel_id"`
		AuthorID                  string    `rethink:"author_id"`
		MessageContent            string    `rethink:"message_content"`
		MessageAttachmentURLs     []string  `rethink:"message_attachment_urls"`
		MessageEmbedImageURL      string    `rethink:"message_embed_image_url"`
		StarboardMessageID        string    `rethink:"starboard_message_id"`
		StarboardMessageChannelID string    `rethink:"starboard_message_channel_id"`
		StarUserIDs               []string  `rethink:"star_user_ids"`
		Stars                     int       `rethink:"stars"`
		FirstStarred              time.Time `rethink:"first_starred"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		_, err = helpers.MDbInsertWithoutLogging(
			models.StarboardEntriesTable,
			models.StarboardEntry{
				GuildID:                   rethinkdbEntry.GuildID,
				MessageID:                 rethinkdbEntry.MessageID,
				ChannelID:                 rethinkdbEntry.ChannelID,
				AuthorID:                  rethinkdbEntry.AuthorID,
				MessageContent:            rethinkdbEntry.MessageContent,
				MessageAttachmentURLs:     rethinkdbEntry.MessageAttachmentURLs,
				MessageEmbedImageURL:      rethinkdbEntry.MessageEmbedImageURL,
				StarboardMessageID:        rethinkdbEntry.StarboardMessageID,
				StarboardMessageChannelID: rethinkdbEntry.StarboardMessageChannelID,
				StarUserIDs:               rethinkdbEntry.StarUserIDs,
				Stars:                     rethinkdbEntry.Stars,
				FirstStarred:              rethinkdbEntry.FirstStarred,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb starboard_entries")
	_, err = gorethink.TableDrop("starboard_entries").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
