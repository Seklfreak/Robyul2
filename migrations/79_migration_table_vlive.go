package migrations

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/gorethink/gorethink"
)

func m79_migration_table_vlive() {
	if !TableExists("vlive") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving vlive to mongodb")

	cursor, err := gorethink.Table("vlive").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("vlive").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	type DB_VLive_Video struct {
		Seq       int64  `gorethink:"seq,omitempty" json:"videoSeq"`
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

	type DB_VLive_Notice struct {
		Number   int64  `gorethink:"number,omitempty" json:"noticeNo"`
		Title    string `json:"title"`
		ImageUrl string `json:"listImageUrl"`
		Summary  string `json:"summary"`
		Url      string `json:"-"`
	}

	type DB_VLive_Celeb struct {
		ID      string `gorethink:"id,omitempty" json:"post_id"`
		Summary string `json:"body_summary"`
		Url     string `json:"-"`
	}

	type DB_VLive_Channel struct {
		Seq           int64  `gorethink:"seq,omitempty" json:"channel_seq"`
		Code          string `gorethink:"code,omitempty" json:"channel_code"`
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
		Upcoming []DB_VLive_Video  `gorethink:"upcoming" json:"-"`
		Live     []DB_VLive_Video  `gorethink:"live" json:"-"`
		VOD      []DB_VLive_Video  `gorethink:"vod" json:"-"`
		Notices  []DB_VLive_Notice `gorethink:"notices" json:"-"`
		Celebs   []DB_VLive_Celeb  `gorethink:"celebs" json:"-"`
		Url      string            `json:"-"`
	}

	var rethinkdbEntry struct {
		ID             string            `gorethink:"id,omitempty"`
		ServerID       string            `gorethink:"serverid"`
		ChannelID      string            `gorethink:"channelid"`
		VLiveChannel   DB_VLive_Channel  `gorethink:"vlivechannel"`
		PostedUpcoming []DB_VLive_Video  `gorethink:"posted_upcoming"`
		PostedLive     []DB_VLive_Video  `gorethink:"posted_live"`
		PostedVOD      []DB_VLive_Video  `gorethink:"posted_vod"`
		PostedNotices  []DB_VLive_Notice `gorethink:"posted_notices"`
		PostedCelebs   []DB_VLive_Celeb  `gorethink:"posted_celebs"`
		MentionRoleID  string            `gorethink:"mention_role_id"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		channelInfo := models.VliveChannelInfo{
			Seq:           rethinkdbEntry.VLiveChannel.Seq,
			Code:          rethinkdbEntry.VLiveChannel.Code,
			Type:          rethinkdbEntry.VLiveChannel.Type,
			Name:          rethinkdbEntry.VLiveChannel.Name,
			Followers:     rethinkdbEntry.VLiveChannel.Followers,
			CoverImgUrl:   rethinkdbEntry.VLiveChannel.CoverImgUrl,
			ProfileImgUrl: rethinkdbEntry.VLiveChannel.ProfileImgUrl,
			Color:         rethinkdbEntry.VLiveChannel.Color,
			TotalVideos:   rethinkdbEntry.VLiveChannel.TotalVideos,
			CelebBoard:    rethinkdbEntry.VLiveChannel.CelebBoard,
			Upcoming:      nil,
			Live:          nil,
			VOD:           nil,
			Notices:       nil,
			Celebs:        nil,
			Url:           rethinkdbEntry.VLiveChannel.Url,
		}
		postedUpcoming := make([]models.VliveVideoInfo, 0)
		for _, upcoming := range rethinkdbEntry.PostedUpcoming {
			postedUpcoming = append(postedUpcoming, models.VliveVideoInfo{
				Seq:       upcoming.Seq,
				Title:     upcoming.Title,
				Plays:     upcoming.Plays,
				Likes:     upcoming.Likes,
				Comments:  upcoming.Comments,
				Thumbnail: upcoming.Thumbnail,
				Date:      upcoming.Date,
				Playtime:  upcoming.Playtime,
				Type:      upcoming.Type,
				Url:       upcoming.Url,
			})
		}
		postedLive := make([]models.VliveVideoInfo, 0)
		for _, live := range rethinkdbEntry.PostedLive {
			postedLive = append(postedLive, models.VliveVideoInfo{
				Seq:       live.Seq,
				Title:     live.Title,
				Plays:     live.Plays,
				Likes:     live.Likes,
				Comments:  live.Comments,
				Thumbnail: live.Thumbnail,
				Date:      live.Date,
				Playtime:  live.Playtime,
				Type:      live.Type,
				Url:       live.Url,
			})
		}
		postedVod := make([]models.VliveVideoInfo, 0)
		for _, vod := range rethinkdbEntry.PostedVOD {
			postedVod = append(postedVod, models.VliveVideoInfo{
				Seq:       vod.Seq,
				Title:     vod.Title,
				Plays:     vod.Plays,
				Likes:     vod.Likes,
				Comments:  vod.Comments,
				Thumbnail: vod.Thumbnail,
				Date:      vod.Date,
				Playtime:  vod.Playtime,
				Type:      vod.Type,
				Url:       vod.Url,
			})
		}
		postedNotices := make([]models.VliveNoticeInfo, 0)
		for _, notice := range rethinkdbEntry.PostedNotices {
			postedNotices = append(postedNotices, models.VliveNoticeInfo{
				Number:   notice.Number,
				Title:    notice.Title,
				ImageUrl: notice.ImageUrl,
				Summary:  notice.Summary,
				Url:      notice.Url,
			})
		}
		postedCelebs := make([]models.VliveCelebInfo, 0)
		for _, celeb := range rethinkdbEntry.PostedCelebs {
			postedCelebs = append(postedCelebs, models.VliveCelebInfo{
				ID:      celeb.ID,
				Summary: celeb.Summary,
				Url:     celeb.Url,
			})
		}

		_, err = helpers.MDbInsertWithoutLogging(
			models.VliveTable,
			models.VliveEntry{
				GuildID:        rethinkdbEntry.ServerID,
				ChannelID:      rethinkdbEntry.ChannelID,
				VLiveChannel:   channelInfo,
				PostedUpcoming: postedUpcoming,
				PostedLive:     postedLive,
				PostedVOD:      postedVod,
				PostedNotices:  postedNotices,
				PostedCelebs:   postedCelebs,
				MentionRoleID:  rethinkdbEntry.MentionRoleID,
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

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb vlive")
	_, err = gorethink.TableDrop("vlive").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
