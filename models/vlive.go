package models

import "github.com/globalsign/mgo/bson"

const (
	VliveTable MongoDbCollection = "vlive"
)

type VliveEntry struct {
	ID             bson.ObjectId `bson:"_id,omitempty"`
	GuildID        string        // renamed from server ID
	ChannelID      string
	VLiveChannel   VliveChannelInfo
	PostedUpcoming []VliveVideoInfo
	PostedLive     []VliveVideoInfo
	PostedVOD      []VliveVideoInfo
	PostedNotices  []VliveNoticeInfo
	PostedCelebs   []VliveCelebInfo
	MentionRoleID  string
}

type VliveChannelInfo struct {
	Seq           int64  `json:"channel_seq"`
	Code          string `json:"channel_code"`
	Type          string `json:"type"`
	Name          string `json:"channel_name"`
	Followers     int64  `json:"fan_count"`
	CoverImgUrl   string `json:"channel_cover_img"`
	ProfileImgUrl string `json:"channel_profile_img"`
	Color         string `json:"representative_color"`
	TotalVideos   int64  `json:"-"`
	CelebBoard    struct {
		BoardID int64 `json:"board_id"`
	} `json:"celeb_board"`
	Upcoming []VliveVideoInfo  `json:"-"`
	Live     []VliveVideoInfo  `json:"-"`
	VOD      []VliveVideoInfo  `json:"-"`
	Notices  []VliveNoticeInfo `json:"-"`
	Celebs   []VliveCelebInfo  `json:"-"`
	Url      string            `json:"-"`
}

type VliveVideoInfo struct {
	Seq       int64  `json:"videoSeq"`
	Title     string `json:"title"`
	Plays     int64  `json:"playCount"`
	Likes     int64  `json:"likeCount"`
	Comments  int64  `json:"commentCount"`
	Thumbnail string `json:"thumbnail"`
	Date      string `json:"onAirStartAt"`
	Playtime  int64  `json:"playTime"`
	Type      string `json:"videoType"`
	Url       string `json:"-"`
}

type VliveNoticeInfo struct {
	Number   int64  `json:"noticeNo"`
	Title    string `json:"title"`
	ImageUrl string `json:"listImageUrl"`
	Summary  string `json:"summary"`
	Url      string `json:"-"`
}

type VliveCelebInfo struct {
	ID      string `json:"post_id"`
	Summary string `json:"body_summary"`
	Url     string `json:"-"`
}
