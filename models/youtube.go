package models

const (
	YoutubeChannelTable  = "youtube_channels"
	YoutubeQuotaRedisKey = "robyul2-discord:youtube:quota"
)

type YoutubeChannelEntry struct {
	// Discord related fields.
	ID                      string `gorethink:"id,omitempty"`
	ServerID                string `gorethink:"server_id"`
	ChannelID               string `gorethink:"channel_id"`
	NextCheckTime           int64  `gorethink:"next_check_time"`
	LastSuccessfulCheckTime int64  `gorethink:"last_successful_check_time"`

	// Youtube channel specific fields.
	YoutubeChannelID    string   `gorethink:"youtube_channel_id"`
	YoutubeChannelName  string   `gorethink:"youtube_channel_name"`
	YoutubePostedVideos []string `gorethink:"youtube_posted_videos"`
}

type YoutubeQuota struct {
	Daily     int64
	Left      int64
	ResetTime int64
}
