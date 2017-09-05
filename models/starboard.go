package models

import "time"

// StarEntry struct
type StarEntry struct {
	ID                        string    `rethink:"id,omitempty"`
	GuildID                   string    `rethink:"guild_id"`
	MessageID                 string    `rethink:"message_id"`
	ChannelID                 string    `rethink:"channel_id"`
	AuthorID                  string    `rethink:"author_id"`
	MessageContent            string    `rethink:"message_content"`
	MessageAttachmentURLs     []string  `rethink:"message_attachment_urls"`
	StarboardMessageID        string    `rethinK:"starboard_message_id"`
	StarboardMessageChannelID string    `rethinK:"starboard_message_channel_id"`
	StarUserIDs               []string  `rethink:"star_user_ids"`
	Stars                     int       `rethin:"stars"`
	FirstStarred              time.Time `rethink:"first_starred"`
}
