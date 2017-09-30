package plugins

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Jeffail/gabs"
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/emojis"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	rethink "github.com/gorethink/gorethink"
	"github.com/satori/go.uuid"
)

type Instagram struct{}

type DB_Instagram_Entry struct {
	ID               string                   `gorethink:"id,omitempty"`
	ServerID         string                   `gorethink:"serverid"`
	ChannelID        string                   `gorethink:"channelid"`
	Username         string                   `gorethink:"username"`
	PostedPosts      []DB_Instagram_Post      `gorethink:"posted_posts"`
	PostedReelMedias []DB_Instagram_ReelMedia `gorethink:"posted_reelmedias"`
	IsLive           bool                     `gorethink:"islive"`
	PostDirectLinks  bool                     `gorethink:"post_direct_links"`
}

type DB_Instagram_Post struct {
	ID        string `gorethink:"id,omitempty"`
	CreatedAt int    `gorethink:"createdat"`
}

type DB_Instagram_ReelMedia struct {
	ID        string `gorethink:"id,omitempty"`
	CreatedAt int    `gorethink:"createdat"`
}

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
	entries []DB_Instagram_Entry
	mux     sync.Mutex
}

var (
	usedUuid             string
	sessionId            string
	rankToken            string
	httpClient           *http.Client
	instagramPicUrlRegex *regexp.Regexp
)

const (
	hexColor                 string = "#fcaf45"
	apiBaseUrl               string = "https://i.instagram.com/api/v1/%s"
	apiUserAgent             string = "Instagram 10.8.0 Android (18/4.3; 320dpi; 720x1280; Xiaomi; HM 1SW; armani; qcom; en_US)"
	instagramSignKey         string = "68a04945eb02970e2e8d15266fc256f7295da123e123f44b88f09d594a5902df"
	deviceId                 string = "android-3deeb2d04b2ab0ee" // TODO: generate a random device id
	instagramFriendlyUser    string = "https://www.instagram.com/%s/"
	instagramFriendlyPost    string = "https://www.instagram.com/p/%s/"
	instagramPicUrlRegexText string = `(http(s)?\:\/\/[^\/]+\/[^\/]+\/)([a-z0-9]+x[a-z0-9]+\/)?([a-z0-9\.]+\/)?(([a-z0-9]+\/)?.+\.jpg)`
)

func (m *Instagram) Commands() []string {
	return []string{
		"instagram",
	}
}

func (m *Instagram) Init(session *discordgo.Session) {
	m.login()
	var err error

	instagramPicUrlRegex, err = regexp.Compile(instagramPicUrlRegexText)
	helpers.Relax(err)

	go m.checkInstagramFeedsLoop()
	cache.GetLogger().WithField("module", "instagram").Info("Started Instagram loop (10m)")
}

func (m *Instagram) checkInstagramFeedsLoop() {
	var safeEntries Instagram_Safe_Entries
	log := cache.GetLogger()

	defer helpers.Recover()
	defer func() {
		go func() {
			log.WithField("module", "instagram").Error("The checkInstagramFeedsLoop died. Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			m.checkInstagramFeedsLoop()
		}()
	}()

	for {
		cursor, err := rethink.Table("instagram").Run(helpers.GetDB())
		helpers.Relax(err)

		err = cursor.All(&safeEntries.entries)
		helpers.Relax(err)

		// TODO: Check multiple entries at once
		for _, entry := range safeEntries.entries {
			safeEntries.mux.Lock()
			changes := false
			log.WithField("module", "instagram").Debug(fmt.Sprintf("checking Instagram Account @%s", entry.Username))

			instagramUser, err := m.lookupInstagramUser(entry.Username)
			if err != nil || instagramUser.Username == "" {
				log.WithField("module", "instagram").Error(fmt.Sprintf("updating instagram account @%s failed: %s", entry.Username, err))
				safeEntries.mux.Unlock()
				continue
			}

			// https://github.com/golang/go/wiki/SliceTricks#reversing
			for i := len(instagramUser.Posts)/2 - 1; i >= 0; i-- {
				opp := len(instagramUser.Posts) - 1 - i
				instagramUser.Posts[i], instagramUser.Posts[opp] = instagramUser.Posts[opp], instagramUser.Posts[i]
			}
			for i := len(instagramUser.ReelMedias)/2 - 1; i >= 0; i-- {
				opp := len(instagramUser.ReelMedias) - 1 - i
				instagramUser.ReelMedias[i], instagramUser.ReelMedias[opp] = instagramUser.ReelMedias[opp], instagramUser.ReelMedias[i]
			}

			for _, post := range instagramUser.Posts {
				postAlreadyPosted := false
				for _, postedPosts := range entry.PostedPosts {
					if postedPosts.ID == post.ID {
						postAlreadyPosted = true
					}
				}
				if postAlreadyPosted == false {
					log.WithField("module", "instagram").Info(fmt.Sprintf("Posting Post: #%s", post.ID))
					entry.PostedPosts = append(entry.PostedPosts, DB_Instagram_Post{ID: post.ID, CreatedAt: post.Caption.CreatedAt})
					changes = true
					go m.postPostToChannel(entry.ChannelID, post, instagramUser, entry.PostDirectLinks)
				}

			}

			for _, reelMedia := range instagramUser.ReelMedias {
				reelMediaAlreadyPosted := false
				for _, reelMediaPostPosted := range entry.PostedReelMedias {
					if reelMediaPostPosted.ID == reelMedia.ID {
						reelMediaAlreadyPosted = true
					}
				}
				if reelMediaAlreadyPosted == false {
					log.WithField("module", "instagram").Info(fmt.Sprintf("Posting Reel Media: #%s", reelMedia.ID))
					entry.PostedReelMedias = append(entry.PostedReelMedias, DB_Instagram_ReelMedia{ID: reelMedia.ID, CreatedAt: reelMedia.DeviceTimestamp})
					changes = true
					go m.postReelMediaToChannel(entry.ChannelID, reelMedia, instagramUser)
				}

			}

			if entry.IsLive == false {
				if instagramUser.Broadcast.ID != 0 {
					log.WithField("module", "instagram").Info(fmt.Sprintf("Posting Live: #%s", instagramUser.Broadcast.ID))
					go m.postLiveToChannel(entry.ChannelID, instagramUser)
					entry.IsLive = true
					changes = true
				}
			} else {
				if instagramUser.Broadcast.ID == 0 {
					entry.IsLive = false
					changes = true
				}
			}

			if changes == true {
				m.setEntry(entry)
			}
			safeEntries.mux.Unlock()
		}

		time.Sleep(1 * time.Minute)
	}
}

func (m *Instagram) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	args := strings.Fields(content)
	if len(args) >= 1 {
		switch args[0] {
		case "add": // [p]instagram add <instagram account name (with or without @)> <discord channel>
			helpers.RequireMod(msg, func() {
				session.ChannelTyping(msg.ChannelID)
				// get target channel
				var err error
				var targetChannel *discordgo.Channel
				var targetGuild *discordgo.Guild
				if len(args) >= 3 {
					targetChannel, err = helpers.GetChannelFromMention(msg, args[len(args)-1])
					if err != nil {
						session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
						return
					}
				} else {
					session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
					return
				}
				targetGuild, err = helpers.GetGuild(targetChannel.GuildID)
				helpers.Relax(err)
				// get instagram account
				instagramUsername := strings.Replace(args[1], "@", "", 1)
				instagramUser, err := m.lookupInstagramUser(instagramUsername)
				if err != nil || instagramUser.Username == "" {
					session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-not-found"))
					return
				}
				// Create DB Entries
				var dbPosts []DB_Instagram_Post
				for _, post := range instagramUser.Posts {
					postEntry := DB_Instagram_Post{ID: post.ID, CreatedAt: post.Caption.CreatedAt}
					dbPosts = append(dbPosts, postEntry)

				}
				var dbRealMedias []DB_Instagram_ReelMedia
				for _, reelMedia := range instagramUser.ReelMedias {
					reelMediaEntry := DB_Instagram_ReelMedia{ID: reelMedia.ID, CreatedAt: reelMedia.DeviceTimestamp}
					dbRealMedias = append(dbRealMedias, reelMediaEntry)

				}
				// create new entry in db
				entry := m.getEntryByOrCreateEmpty("id", "")
				entry.ServerID = targetChannel.GuildID
				entry.ChannelID = targetChannel.ID
				entry.Username = instagramUser.Username
				entry.PostedPosts = dbPosts
				entry.PostedReelMedias = dbRealMedias
				m.setEntry(entry)

				session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-added-success", entry.Username, entry.ChannelID))
				cache.GetLogger().WithField("module", "instagram").Info(fmt.Sprintf("Added Instagram Account @%s to Channel %s (#%s) on Guild %s (#%s)", entry.Username, targetChannel.Name, entry.ChannelID, targetGuild.Name, targetGuild.ID))
			})
		case "delete", "del", "remove": // [p]instagram delete <id>
			helpers.RequireMod(msg, func() {
				if len(args) >= 2 {
					session.ChannelTyping(msg.ChannelID)
					entryId := args[1]
					entryBucket := m.getEntryBy("id", entryId)
					if entryBucket.ID != "" {
						m.deleteEntryById(entryBucket.ID)

						session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-delete-success", entryBucket.Username))
						cache.GetLogger().WithField("module", "instagram").Info(fmt.Sprintf("Deleted Instagram Account @%s", entryBucket.Username))
					} else {
						session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.instagram.account-delete-not-found-error"))
						return
					}
				} else {
					session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					return
				}
			})
		case "list": // [p]instagram list
			currentChannel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)
			var entryBucket []DB_Instagram_Entry
			listCursor, err := rethink.Table("instagram").Filter(
				rethink.Row.Field("serverid").Eq(currentChannel.GuildID),
			).Run(helpers.GetDB())
			helpers.Relax(err)
			defer listCursor.Close()
			err = listCursor.All(&entryBucket)

			if err == rethink.ErrEmptyResult || len(entryBucket) <= 0 {
				session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-list-no-accounts-error"))
				return
			} else if err != nil {
				helpers.Relax(err)
			}

			resultMessage := ""
			for _, entry := range entryBucket {
				resultMessage += fmt.Sprintf("`%s`: Instagram Account `@%s` posting to <#%s>\n", entry.ID, entry.Username, entry.ChannelID)
			}
			resultMessage += fmt.Sprintf("Found **%d** Instagram Accounts in total.", len(entryBucket))
			for _, resultPage := range helpers.Pagify(resultMessage, "\n") {
				_, err = session.ChannelMessageSend(msg.ChannelID, resultPage)
				helpers.Relax(err)
			}
		case "toggle-direct-link", "toggle-direct-links": // [p]instagram toggle-direct-links <id>
			helpers.RequireMod(msg, func() {
				session.ChannelTyping(msg.ChannelID)
				if len(args) < 2 {
					session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					return
				}
				entryId := args[1]
				entryBucket := m.getEntryBy("id", entryId)
				if entryBucket.ID == "" {
					session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					return
				}
				var messageText string
				if entryBucket.PostDirectLinks {
					entryBucket.PostDirectLinks = false
					messageText = helpers.GetText("plugins.instagram.post-direct-links-disabled")
				} else {
					entryBucket.PostDirectLinks = true
					messageText = helpers.GetText("plugins.instagram.post-direct-links-enabled")
				}
				m.setEntry(entryBucket)
				session.ChannelMessageSend(msg.ChannelID, messageText)
				return
			})
		default:
			session.ChannelTyping(msg.ChannelID)
			instagramUsername := strings.Replace(args[0], "@", "", 1)
			instagramUser, err := m.lookupInstagramUser(instagramUsername)

			if err != nil || instagramUser.Username == "" {
				session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-not-found"))
				return
			}

			instagramNameModifier := ""
			if instagramUser.IsVerified {
				instagramNameModifier += " ☑"
			}
			if instagramUser.IsPrivate {
				instagramNameModifier += " 🔒"
			}
			if instagramUser.IsBusiness {
				instagramNameModifier += " 🏢"
			}
			if instagramUser.IsFavorite {
				instagramNameModifier += " ⭐"
			}
			accountEmbed := &discordgo.MessageEmbed{
				Title:       helpers.GetTextF("plugins.instagram.account-embed-title", instagramUser.FullName, instagramUser.Username, instagramNameModifier),
				URL:         fmt.Sprintf(instagramFriendlyUser, instagramUser.Username),
				Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: instagramUser.ProfilePic.URL},
				Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.instagram.embed-footer")},
				Description: instagramUser.Biography,
				Fields: []*discordgo.MessageEmbedField{
					{Name: "Followers", Value: humanize.Comma(int64(instagramUser.FollowerCount)), Inline: true},
					{Name: "Following", Value: humanize.Comma(int64(instagramUser.FollowingCount)), Inline: true},
					{Name: "Posts", Value: humanize.Comma(int64(instagramUser.MediaCount)), Inline: true}},
				Color: helpers.GetDiscordColorFromHex(hexColor),
			}
			if instagramUser.ExternalURL != "" {
				accountEmbed.Fields = append(accountEmbed.Fields, &discordgo.MessageEmbedField{
					Name:   "Website",
					Value:  instagramUser.ExternalURL,
					Inline: true,
				})
			}
			_, err = session.ChannelMessageSendComplex(
				msg.ChannelID, &discordgo.MessageSend{
					Content: fmt.Sprintf("<%s>", fmt.Sprintf(instagramFriendlyUser, instagramUser.Username)),
					Embed:   accountEmbed,
				})
			helpers.Relax(err)
			return
		}
	} else {
		session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
	}
}

func (m *Instagram) postLiveToChannel(channelID string, instagramUser Instagram_User) {
	instagramNameModifier := ""
	if instagramUser.IsVerified {
		instagramNameModifier += " ☑"
	}
	if instagramUser.IsPrivate {
		instagramNameModifier += " 🔒"
	}
	if instagramUser.IsBusiness {
		instagramNameModifier += " 🏢"
	}
	if instagramUser.IsFavorite {
		instagramNameModifier += " ⭐"
	}

	channelEmbed := &discordgo.MessageEmbed{
		Title:     helpers.GetTextF("plugins.instagram.live-embed-title", instagramUser.FullName, instagramUser.Username, instagramNameModifier),
		URL:       fmt.Sprintf(instagramFriendlyUser, instagramUser.Username),
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: instagramUser.ProfilePic.URL},
		Footer:    &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.instagram.embed-footer")},
		Image:     &discordgo.MessageEmbedImage{URL: instagramUser.Broadcast.CoverFrameURL},
		Color:     helpers.GetDiscordColorFromHex(hexColor),
	}

	mediaUrl := ""

	if mediaUrl != "" {
		channelEmbed.URL = mediaUrl
	} else {
		mediaUrl = channelEmbed.URL
	}

	_, err := cache.GetSession().ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content: fmt.Sprintf("<%s>", mediaUrl),
		Embed:   channelEmbed,
	})
	if err != nil {
		cache.GetLogger().WithField("module", "instagram").Error(fmt.Sprintf("posting broadcast: #%s to channel: #%s failed: %s", instagramUser.Broadcast.ID, channelID, err))
	}
}

func (m *Instagram) postReelMediaToChannel(channelID string, reelMedia Instagram_ReelMedia, instagramUser Instagram_User) {
	instagramNameModifier := ""
	if instagramUser.IsVerified {
		instagramNameModifier += " ☑"
	}
	if instagramUser.IsPrivate {
		instagramNameModifier += " 🔒"
	}
	if instagramUser.IsBusiness {
		instagramNameModifier += " 🏢"
	}
	if instagramUser.IsFavorite {
		instagramNameModifier += " ⭐"
	}

	mediaModifier := "Picture"
	if reelMedia.MediaType == 2 {
		mediaModifier = "Video"
	}

	channelEmbed := &discordgo.MessageEmbed{
		Title:       helpers.GetTextF("plugins.instagram.reelmedia-embed-title", instagramUser.FullName, instagramUser.Username, instagramNameModifier, mediaModifier),
		URL:         fmt.Sprintf(instagramFriendlyUser, instagramUser.Username),
		Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: instagramUser.ProfilePic.URL},
		Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.instagram.embed-footer")},
		Description: reelMedia.Caption,
		Color:       helpers.GetDiscordColorFromHex(hexColor),
	}

	mediaUrl := ""

	if len(reelMedia.ImageVersions2.Candidates) > 0 {
		channelEmbed.Image = &discordgo.MessageEmbedImage{URL: reelMedia.ImageVersions2.Candidates[0].URL}
		mediaUrl = getFullResUrl(reelMedia.ImageVersions2.Candidates[0].URL)
	}
	if len(reelMedia.VideoVersions) > 0 {
		channelEmbed.Video = &discordgo.MessageEmbedVideo{
			URL: reelMedia.VideoVersions[0].URL, Height: reelMedia.VideoVersions[0].Height, Width: reelMedia.VideoVersions[0].Width}
		mediaUrl = getFullResUrl(reelMedia.VideoVersions[0].URL)
	}

	if mediaUrl != "" {
		channelEmbed.URL = mediaUrl
	} else {
		mediaUrl = channelEmbed.URL
	}

	_, err := cache.GetSession().ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content: fmt.Sprintf("<%s>", mediaUrl),
		Embed:   channelEmbed,
	})
	if err != nil {
		cache.GetLogger().WithField("module", "instagram").Error("posting reel media: #%s to channel: #%s failed: %s", reelMedia.ID, channelID, err)
	}
}

func (m *Instagram) postPostToChannel(channelID string, post Instagram_Post, instagramUser Instagram_User, postDirectLinks bool) {
	instagramNameModifier := ""
	if instagramUser.IsVerified {
		instagramNameModifier += " ☑"
	}
	if instagramUser.IsPrivate {
		instagramNameModifier += " 🔒"
	}
	if instagramUser.IsBusiness {
		instagramNameModifier += " 🏢"
	}
	if instagramUser.IsFavorite {
		instagramNameModifier += " ⭐"
	}

	mediaModifier := "Picture"
	if post.MediaType == 2 {
		mediaModifier = "Video"
	}
	if post.MediaType == 8 {
		mediaModifier = "Album"
		if len(post.CarouselMedia) > 0 {
			mediaModifier = fmt.Sprintf("Album (%d items)", len(post.CarouselMedia))
		}
	}

	var content string
	channelEmbed := &discordgo.MessageEmbed{
		Title:       helpers.GetTextF("plugins.instagram.post-embed-title", instagramUser.FullName, instagramUser.Username, instagramNameModifier, mediaModifier),
		URL:         fmt.Sprintf(instagramFriendlyPost, post.Code),
		Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: instagramUser.ProfilePic.URL},
		Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.instagram.embed-footer")},
		Description: post.Caption.Text,
		Color:       helpers.GetDiscordColorFromHex(hexColor),
	}
	if postDirectLinks {
		content += "**" + helpers.GetTextF("plugins.instagram.post-embed-title", instagramUser.FullName, instagramUser.Username, instagramNameModifier, mediaModifier) + "**\n"
		if post.Caption.Text != "" {
			content += post.Caption.Text + "\n"
		}
	}

	if len(post.ImageVersions2.Candidates) > 0 {
		channelEmbed.Image = &discordgo.MessageEmbedImage{URL: getFullResUrl(post.ImageVersions2.Candidates[0].URL)}
	}
	if len(post.CarouselMedia) > 0 && len(post.CarouselMedia[0].ImageVersions2.Candidates) > 0 {
		channelEmbed.Image = &discordgo.MessageEmbedImage{URL: getFullResUrl(post.CarouselMedia[0].ImageVersions2.Candidates[0].URL)}
	}

	mediaUrls := make([]string, 0)
	if len(post.CarouselMedia) <= 0 {
		mediaUrls = append(mediaUrls, getFullResUrl(post.ImageVersions2.Candidates[0].URL))
	} else {
		for _, carouselMedia := range post.CarouselMedia {
			mediaUrls = append(mediaUrls, getFullResUrl(carouselMedia.ImageVersions2.Candidates[0].URL))
		}
	}

	content += fmt.Sprintf("<%s>", fmt.Sprintf(instagramFriendlyPost, post.Code))

	if len(mediaUrls) > 0 {
		channelEmbed.Description += "\n\n`Links:` "
		for i, mediaUrl := range mediaUrls {
			if postDirectLinks {
				content += "\n" + getFullResUrl(mediaUrl)
			}
			channelEmbed.Description += fmt.Sprintf("[%s](%s) ", emojis.From(strconv.Itoa(i+1)), getFullResUrl(mediaUrl))
		}
	}

	messageSend := &discordgo.MessageSend{
		Content: content,
	}
	if !postDirectLinks {
		messageSend.Embed = channelEmbed
	}

	_, err := cache.GetSession().ChannelMessageSendComplex(channelID, messageSend)
	if err != nil {
		cache.GetLogger().WithField("module", "instagram").Error(fmt.Sprintf("posting post: #%s to channel: #%s failed: %s", post.ID, channelID, err))
	}
}

func getFullResUrl(url string) string {
	result := instagramPicUrlRegex.FindStringSubmatch(url)
	if result != nil && len(result) >= 6 {
		return result[1] + result[5]
	}
	return url
}

func (m *Instagram) signDataValue(data string) string {
	key := hmac.New(sha256.New, []byte(instagramSignKey))
	key.Write([]byte(data))
	return fmt.Sprintf("ig_sig_key_version=%s&signed_body=%s.%s", "4", hex.EncodeToString(key.Sum(nil)), url.QueryEscape(data))
}

func (m *Instagram) applyHeaders(request *http.Request) {
	request.Header.Add("Connection", "close")
	request.Header.Add("Accept", "*/*")
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	request.Header.Add("Cookie2", "$Version=1")
	request.Header.Add("Accept-Language", "en-US")
	request.Header.Add("User-Agent", apiUserAgent)
	if sessionId != "" {
		request.Header.Add("Cookie", fmt.Sprintf("sessionid=%s", sessionId))
	}
}

// quick port of https://github.com/LevPasha/Instagram-API-python
func (m *Instagram) login() {
	usedUuid = uuid.NewV4().String()
	// get csrf token
	signupEndpoint := fmt.Sprintf(apiBaseUrl, fmt.Sprintf("si/fetch_headers/?challenge_type=signup&guid=%s", usedUuid))
	httpClient = &http.Client{}
	request, err := http.NewRequest("GET", signupEndpoint, nil)
	helpers.Relax(err)
	m.applyHeaders(request)
	response, err := httpClient.Do(request)
	helpers.Relax(err)
	defer response.Body.Close()
	csrfToken := ""
	for _, cookie := range response.Cookies() {
		if cookie.Name == "csrftoken" {
			csrfToken = cookie.Value
		}
	}
	if csrfToken == "" {
		helpers.Relax(errors.New("Unable to get CSRF Token while trying to authenticate to instagram."))
	}
	// login
	loginEndpoint := fmt.Sprintf(apiBaseUrl, "accounts/login/")
	jsonParsed, err := gabs.ParseJSON([]byte(fmt.Sprintf(
		`{"phone_id": "%s",
    "_csrftoken": "%s",
    "username": "%s",
    "guid": "%s",
    "device_id": "%s",
    "password": "%s",
    "login_attempt_count": "0"}`, uuid.NewV4().String(), csrfToken, helpers.GetConfig().Path("instagram.username").Data().(string), usedUuid, deviceId, helpers.GetConfig().Path("instagram.password").Data().(string))))
	helpers.Relax(err)
	request, err = http.NewRequest("POST", loginEndpoint, strings.NewReader(m.signDataValue(jsonParsed.String())))
	helpers.Relax(err)
	m.applyHeaders(request)
	response, err = httpClient.Do(request)
	helpers.Relax(err)
	defer response.Body.Close()
	csrfToken = ""
	sessionId = ""
	for _, cookie := range response.Cookies() {
		if cookie.Name == "csrftoken" {
			csrfToken = cookie.Value
		}
		if cookie.Name == "sessionid" {
			sessionId = cookie.Value
		}
	}
	if csrfToken == "" {
		helpers.Relax(errors.New("Unable to get CSRF Token while trying to authenticate to instagram."))
	}
	if sessionId == "" {
		helpers.Relax(errors.New("Unable to get Session ID while trying to authenticate to instagram."))
	}
	if response.StatusCode != 200 {
		helpers.Relax(errors.New(fmt.Sprintf("Instagram login failed, unexpected status code: %d", response.StatusCode)))
	}
	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, response.Body)
	helpers.Relax(err)
	jsonResult, err := gabs.ParseJSON(buf.Bytes())
	helpers.Relax(err)
	usernameIdFloat, ok := jsonResult.Path("logged_in_user.pk").Data().(float64)
	if ok == false {
		helpers.Relax(errors.New("Unable to get username id from instagram login reply"))
	}
	usernameId := strconv.FormatFloat(usernameIdFloat, 'f', 0, 64)
	usernameLoggedIn, ok := jsonResult.Path("logged_in_user.username").Data().(string)
	if ok == false {
		helpers.Relax(errors.New("Unable to get username from instagram login reply"))
	}
	cache.GetLogger().WithField("module", "instagram").Info(fmt.Sprintf("logged in as @%s", usernameLoggedIn))
	rankToken = fmt.Sprintf("%s_%s", usernameId, usedUuid)
}

func (m *Instagram) lookupInstagramUser(username string) (Instagram_User, error) {
	var instagramUser Instagram_User

	userEndpoint := fmt.Sprintf(apiBaseUrl, fmt.Sprintf("users/%s/usernameinfo/", username))
	request, err := http.NewRequest("GET", userEndpoint, nil)
	if err != nil {
		return instagramUser, err
	}
	m.applyHeaders(request)
	response, err := httpClient.Do(request)
	if err != nil {
		return instagramUser, err
	}
	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, response.Body)
	if err != nil {
		return instagramUser, err
	}
	jsonResult, err := gabs.ParseJSON(buf.Bytes())
	if err != nil {
		return instagramUser, err
	}
	retry, errorMessage := m.checkInstagramResult(jsonResult)
	if retry == true {
		cache.GetLogger().WithField("module", "instagram").Info(fmt.Sprintf("hit rate limit checking Instagram Account @%s, sleeping for 20 seconds and then trying again", username))
		time.Sleep(20 * time.Second)
		return m.lookupInstagramUser(username)
	} else if errorMessage != "" {
		return instagramUser, errors.New(fmt.Sprintf("Instagram API Error: %s", errorMessage))
	}
	json.Unmarshal([]byte(jsonResult.Path("user").String()), &instagramUser)

	userFeedEndpoint := fmt.Sprintf(apiBaseUrl, fmt.Sprintf("feed/user/%s/?max_id=%s&min_timestamp=%s&rank_token=%s&ranked_content=true", strconv.Itoa(instagramUser.Pk), "", "", rankToken))
	request, err = http.NewRequest("GET", userFeedEndpoint, nil)
	if err != nil {
		return instagramUser, err
	}
	m.applyHeaders(request)
	response, err = httpClient.Do(request)
	if err != nil {
		return instagramUser, err
	}
	buf = bytes.NewBuffer(nil)
	_, err = io.Copy(buf, response.Body)
	if err != nil {
		return instagramUser, err
	}
	jsonResult, err = gabs.ParseJSON(buf.Bytes())
	if err != nil {
		return instagramUser, err
	}
	retry, errorMessage = m.checkInstagramResult(jsonResult)
	if retry == true {
		cache.GetLogger().WithField("module", "instagram").Info(fmt.Sprintf("hit rate limit checking Instagram Account @%s, sleeping for 20 seconds and then trying again", username))
		time.Sleep(20 * time.Second)
		return m.lookupInstagramUser(username)
	} else if errorMessage != "" {
		return instagramUser, errors.New(fmt.Sprintf("Instagram API Error: %s", errorMessage))
	}

	var instagramPosts []Instagram_Post
	instagramPostsJsons, err := jsonResult.Path("items").Children()
	if err != nil {
		return instagramUser, err
	}
	for _, instagramPostJson := range instagramPostsJsons {
		var instagramPost Instagram_Post
		json.Unmarshal([]byte(instagramPostJson.String()), &instagramPost)
		instagramPosts = append(instagramPosts, instagramPost)
	}
	instagramUser.Posts = instagramPosts

	userReelEndpoint := fmt.Sprintf(apiBaseUrl, fmt.Sprintf("feed/user/%s/story/", strconv.Itoa(instagramUser.Pk)))
	request, err = http.NewRequest("GET", userReelEndpoint, nil)
	if err != nil {
		return instagramUser, err
	}
	m.applyHeaders(request)
	response, err = httpClient.Do(request)
	if err != nil {
		return instagramUser, err
	}
	buf = bytes.NewBuffer(nil)
	_, err = io.Copy(buf, response.Body)
	if err != nil {
		return instagramUser, err
	}
	jsonResult, err = gabs.ParseJSON(buf.Bytes())
	if err != nil {
		return instagramUser, err
	}
	retry, errorMessage = m.checkInstagramResult(jsonResult)
	if retry == true {
		cache.GetLogger().WithField("module", "instagram").Info(fmt.Sprintf("hit rate limit checking Instagram Account @%s, sleeping for 20 seconds and then trying again", username))
		time.Sleep(20 * time.Second)
		return m.lookupInstagramUser(username)
	} else if errorMessage != "" {
		return instagramUser, errors.New(fmt.Sprintf("Instagram API Error: %s", errorMessage))
	}

	if jsonResult.ExistsP("reel.items") {
		var instagramReelMedias []Instagram_ReelMedia
		instagramReelMediasJsons, err := jsonResult.Path("reel.items").Children()
		if err != nil {
			return instagramUser, err
		}
		for _, instagramReelMediaJson := range instagramReelMediasJsons {
			var instagramReelMedia Instagram_ReelMedia
			json.Unmarshal([]byte(instagramReelMediaJson.String()), &instagramReelMedia)
			instagramReelMedias = append(instagramReelMedias, instagramReelMedia)
		}
		instagramUser.ReelMedias = instagramReelMedias
	}

	if jsonResult.ExistsP("broadcast.id") {
		json.Unmarshal([]byte(jsonResult.Path("broadcast").String()), &instagramUser.Broadcast)
	}

	return instagramUser, nil
}

func (m *Instagram) checkInstagramResult(jsonResult *gabs.Container) (bool, string) {
	if jsonResult.Path("status").Data().(string) == "fail" {
		errorMessage := jsonResult.Path("message").Data().(string)
		if errorMessage == "Please wait a few minutes before you try again." {
			return true, errorMessage
		} else {
			return false, errorMessage
		}
	}
	return false, ""
}

func (m *Instagram) getEntryBy(key string, id string) DB_Instagram_Entry {
	var entryBucket DB_Instagram_Entry
	listCursor, err := rethink.Table("instagram").Filter(
		rethink.Row.Field(key).Eq(id),
	).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer listCursor.Close()
	err = listCursor.One(&entryBucket)

	if err == rethink.ErrEmptyResult {
		return entryBucket
	} else if err != nil {
		panic(err)
	}

	return entryBucket
}

func (m *Instagram) getEntryByOrCreateEmpty(key string, id string) DB_Instagram_Entry {
	var entryBucket DB_Instagram_Entry
	listCursor, err := rethink.Table("instagram").Filter(
		rethink.Row.Field(key).Eq(id),
	).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer listCursor.Close()
	err = listCursor.One(&entryBucket)

	// If user has no DB entries create an empty document
	if err == rethink.ErrEmptyResult {
		insert := rethink.Table("instagram").Insert(DB_Instagram_Entry{})
		res, e := insert.RunWrite(helpers.GetDB())
		// If the creation was successful read the document
		if e != nil {
			panic(e)
		} else {
			return m.getEntryByOrCreateEmpty("id", res.GeneratedKeys[0])
		}
	} else if err != nil {
		panic(err)
	}

	return entryBucket
}

func (m *Instagram) setEntry(entry DB_Instagram_Entry) {
	_, err := rethink.Table("instagram").Update(entry).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}

func (m *Instagram) deleteEntryById(id string) {
	_, err := rethink.Table("instagram").Filter(
		rethink.Row.Field("id").Eq(id),
	).Delete().RunWrite(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
