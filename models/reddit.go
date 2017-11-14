package models

import "time"

const (
	RedditSubredditsTable = "reddit_subreddits"
)

type RedditSubredditEntry struct {
	ID              string    `rethink:"id,omitempty"`
	SubredditName   string    `rethink:"subreddit_name"`
	LastChecked     time.Time `rethink:"last_checked"`
	GuildID         string    `rethink:"guild_id"`
	ChannelID       string    `rethink:"channel_id"`
	AddedByUserID   string    `rethink:"addedby_user_id"`
	AddedAt         time.Time `rethink:"addedat"`
	PostDelay       int       `rethink:"post_delay"`
	PostDirectLinks bool      `rethink:"post_direct_links"`
}
