package instagram

import (
	"sync"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
)

type Instagram_User struct {
	Biography      string `json:"biography"`
	ExternalURL    string `json:"external_url"`
	FollowerCount  int    `json:"follower_count"`
	FollowingCount int    `json:"following_count"`
	FullName       string `json:"full_name"`
	ProfilePic     struct {
		URL string `json:"url"`
	} `json:"hd_profile_pic_url_info"`
	IsBusiness bool                  `json:"is_business"`
	IsFavorite bool                  `json:"is_favorite"`
	IsPrivate  bool                  `json:"is_private"`
	IsVerified bool                  `json:"is_verified"`
	MediaCount int                   `json:"media_count"`
	Pk         int                   `json:"pk"`
	Username   string                `json:"username"`
	Posts      []Instagram_Post      `json:"-"`
	ReelMedias []Instagram_ReelMedia `json:"-"`
	Broadcast  Instagram_Broadcast   `json:"-"`
}

type Instagram_Post struct {
	Caption struct {
		Text      string `json:"text"`
		CreatedAt int    `json:"created_at"`
	} `json:"caption"`
	ID             string `json:"id"`
	ImageVersions2 struct {
		Candidates []struct {
			Height int    `json:"height"`
			URL    string `json:"url"`
			Width  int    `json:"width"`
		} `json:"candidates"`
	} `json:"image_versions2"`
	MediaType     int    `json:"media_type"`
	Code          string `json:"code"`
	CarouselMedia []struct {
		CarouselParentID string `json:"carousel_parent_id"`
		ID               string `json:"id"`
		ImageVersions2   struct {
			Candidates []struct {
				Height int    `json:"height"`
				URL    string `json:"url"`
				Width  int    `json:"width"`
			} `json:"candidates"`
		} `json:"image_versions2"`
		MediaType      int   `json:"media_type"`
		OriginalHeight int   `json:"original_height"`
		OriginalWidth  int   `json:"original_width"`
		Pk             int64 `json:"pk"`
	} `json:"carousel_media"`
}

type Instagram_ReelMedia struct {
	CanViewerSave       bool   `json:"can_viewer_save"`
	Caption             string `json:"caption"`
	CaptionIsEdited     bool   `json:"caption_is_edited"`
	CaptionPosition     int    `json:"caption_position"`
	ClientCacheKey      string `json:"client_cache_key"`
	Code                string `json:"code"`
	CommentCount        int    `json:"comment_count"`
	CommentLikesEnabled bool   `json:"comment_likes_enabled"`
	DeviceTimestamp     int    `json:"device_timestamp"`
	ExpiringAt          int    `json:"expiring_at"`
	FilterType          int    `json:"filter_type"`
	HasAudio            bool   `json:"has_audio"`
	HasLiked            bool   `json:"has_liked"`
	HasMoreComments     bool   `json:"has_more_comments"`
	ID                  string `json:"id"`
	ImageVersions2      struct {
		Candidates []struct {
			Height int    `json:"height"`
			URL    string `json:"url"`
			Width  int    `json:"width"`
		} `json:"candidates"`
	} `json:"image_versions2"`
	IsReelMedia                  bool          `json:"is_reel_media"`
	LikeCount                    int           `json:"like_count"`
	Likers                       []interface{} `json:"likers"`
	MaxNumVisiblePreviewComments int           `json:"max_num_visible_preview_comments"`
	MediaType                    int           `json:"media_type"`
	OrganicTrackingToken         string        `json:"organic_tracking_token"`
	OriginalHeight               int           `json:"original_height"`
	OriginalWidth                int           `json:"original_width"`
	PhotoOfYou                   bool          `json:"photo_of_you"`
	Pk                           int64         `json:"pk"`
	PreviewComments              []interface{} `json:"preview_comments"`
	ReelMentions                 []interface{} `json:"reel_mentions"`
	StoryLocations               []interface{} `json:"story_locations"`
	TakenAt                      int           `json:"taken_at"`
	User                         struct {
		FullName                   string `json:"full_name"`
		HasAnonymousProfilePicture bool   `json:"has_anonymous_profile_picture"`
		IsFavorite                 bool   `json:"is_favorite"`
		IsPrivate                  bool   `json:"is_private"`
		IsUnpublished              bool   `json:"is_unpublished"`
		IsVerified                 bool   `json:"is_verified"`
		Pk                         int    `json:"pk"`
		ProfilePicID               string `json:"profile_pic_id"`
		ProfilePicURL              string `json:"profile_pic_url"`
		Username                   string `json:"username"`
	} `json:"user"`
	VideoDuration float64 `json:"video_duration"`
	VideoVersions []struct {
		Height int    `json:"height"`
		Type   int    `json:"type"`
		URL    string `json:"url"`
		Width  int    `json:"width"`
	} `json:"video_versions"`
}

type Instagram_Broadcast struct {
	BroadcastMessage string `json:"broadcast_message"`
	BroadcastOwner   struct {
		FriendshipStatus struct {
			Blocking        bool `json:"blocking"`
			FollowedBy      bool `json:"followed_by"`
			Following       bool `json:"following"`
			IncomingRequest bool `json:"incoming_request"`
			IsPrivate       bool `json:"is_private"`
			OutgoingRequest bool `json:"outgoing_request"`
		} `json:"friendship_status"`
		FullName      string `json:"full_name"`
		IsPrivate     bool   `json:"is_private"`
		IsVerified    bool   `json:"is_verified"`
		Pk            int    `json:"pk"`
		ProfilePicID  string `json:"profile_pic_id"`
		ProfilePicURL string `json:"profile_pic_url"`
		Username      string `json:"username"`
	} `json:"broadcast_owner"`
	BroadcastStatus      string `json:"broadcast_status"`
	CoverFrameURL        string `json:"cover_frame_url"`
	DashAbrPlaybackURL   string `json:"dash_abr_playback_url"`
	DashPlaybackURL      string `json:"dash_playback_url"`
	ID                   int64  `json:"id"`
	MediaID              string `json:"media_id"`
	OrganicTrackingToken string `json:"organic_tracking_token"`
	PublishedTime        int    `json:"published_time"`
	RtmpPlaybackURL      string `json:"rtmp_playback_url"`
	ViewerCount          int    `json:"viewer_count"`
}

type Instagram_Safe_Entries struct {
	entries []models.InstagramEntry
	mux     sync.Mutex
}

type Instagram_GraphQl_User_Feed struct {
	Data struct {
		User struct {
			EdgeOwnerToTimelineMedia struct {
				Count    int `json:"count"`
				PageInfo struct {
					HasNextPage bool   `json:"has_next_page"`
					EndCursor   string `json:"end_cursor"`
				} `json:"page_info"`
				Edges []struct {
					Node struct {
						ID                 string `json:"id"`
						Typename           string `json:"__typename"`
						EdgeMediaToCaption struct {
							Edges []struct {
								Node struct {
									Text string `json:"text"`
								} `json:"node"`
							} `json:"edges"`
						} `json:"edge_media_to_caption"`
						Shortcode          string `json:"shortcode"`
						EdgeMediaToComment struct {
							Count int `json:"count"`
						} `json:"edge_media_to_comment"`
						CommentsDisabled bool `json:"comments_disabled"`
						TakenAtTimestamp int  `json:"taken_at_timestamp"`
						Dimensions       struct {
							Height int `json:"height"`
							Width  int `json:"width"`
						} `json:"dimensions"`
						DisplayURL           string `json:"display_url"`
						EdgeMediaPreviewLike struct {
							Count int `json:"count"`
						} `json:"edge_media_preview_like"`
						Owner struct {
							ID string `json:"id"`
						} `json:"owner"`
						ThumbnailSrc       string `json:"thumbnail_src"`
						ThumbnailResources []struct {
							Src          string `json:"src"`
							ConfigWidth  int    `json:"config_width"`
							ConfigHeight int    `json:"config_height"`
						} `json:"thumbnail_resources"`
						IsVideo bool `json:"is_video"`
					} `json:"node"`
				} `json:"edges"`
			} `json:"edge_owner_to_timeline_media"`
		} `json:"user"`
	} `json:"data"`
	Status string `json:"status"`
}

func (m *Handler) getBundledEntries() (bundledEntries map[int64][]models.InstagramEntry, entriesCount int, err error) {
	var entries []models.InstagramEntry

	err = helpers.MDbIter(helpers.MdbCollection(models.InstagramTable).Find(nil)).All(&entries)

	bundledEntries = make(map[int64][]models.InstagramEntry, 0)

	for _, entry := range entries {
		if entry.InstagramUserID == 0 {
			continue
		}

		channel, err := helpers.GetChannelWithoutApi(entry.ChannelID)
		if err != nil || channel == nil || channel.ID == "" {
			//cache.GetLogger().WithField("module", "instagram").Infof("skipped instagram @%s for Channel #%s on Guild #%s: channel not found!",
			//	entry.Username, entry.ChannelID, entry.ServerID)
			continue
		}

		if _, ok := bundledEntries[entry.InstagramUserID]; ok {
			bundledEntries[entry.InstagramUserID] = append(bundledEntries[entry.InstagramUserID], entry)
		} else {
			bundledEntries[entry.InstagramUserID] = []models.InstagramEntry{entry}
		}
	}

	return bundledEntries, len(entries), nil
}
