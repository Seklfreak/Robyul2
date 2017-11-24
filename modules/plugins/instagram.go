package plugins

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/emojis"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/ahmdrz/goinsta"
	goinstaResponse "github.com/ahmdrz/goinsta/response"
	goinstaStore "github.com/ahmdrz/goinsta/store"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	rethink "github.com/gorethink/gorethink"
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
	CreatedAt int64  `gorethink:"createdat"`
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
	instagramClient      *goinsta.Instagram
	httpClient           *http.Client
	instagramPicUrlRegex *regexp.Regexp
)

const (
	hexColor                 = "#fcaf45"
	instagramFriendlyUser    = "https://www.instagram.com/%s/"
	instagramFriendlyPost    = "https://www.instagram.com/p/%s/"
	instagramPicUrlRegexText = `(http(s)?\:\/\/[^\/]+\/[^\/]+\/)([a-z0-9\.]+\/)?([a-z0-9\.]+\/)?([a-z0-9]+x[a-z0-9]+\/)?([a-z0-9\.]+\/)?(([a-z0-9]+\/)?.+\.jpg)`
	instagramSessionKey      = "robyul2-discord:instagram:session"
)

func (m *Instagram) Commands() []string {
	return []string{
		"instagram",
	}
}

func (m *Instagram) Init(session *discordgo.Session) {
	var err error

	storedInstagram, err := cache.GetRedisClient().Get(instagramSessionKey).Bytes()
	if err == nil {
		instagramClient, err = goinstaStore.Import(storedInstagram, make([]byte, 32))
		helpers.Relax(err)
		cache.GetLogger().WithField("module", "instagram").Infof(
			"restoring instagram session from redis",
		)
	} else {
		instagramClient = goinsta.New(
			helpers.GetConfig().Path("instagram.username").Data().(string),
			helpers.GetConfig().Path("instagram.password").Data().(string),
		)
		cache.GetLogger().WithField("module", "instagram").Infof(
			"starting new instagram session",
		)
	}
	err = instagramClient.Login() // TODO: login required when restoring session?
	helpers.Relax(err)
	cache.GetLogger().WithField("module", "instagram").Infof(
		"logged in to instagram as @%s",
		instagramClient.Informations.Username,
	)
	// defer instagramClient.Logout() TODO
	storedInstagram, err = goinstaStore.Export(instagramClient, make([]byte, 32))
	helpers.Relax(err)
	err = cache.GetRedisClient().Set(instagramSessionKey, storedInstagram, 0).Err()
	helpers.Relax(err)
	cache.GetLogger().WithField("module", "instagram").Infof(
		"stored instagram session in redis",
	)

	instagramPicUrlRegex, err = regexp.Compile(instagramPicUrlRegexText)
	helpers.Relax(err)

	go m.checkInstagramFeedsLoop()
	cache.GetLogger().WithField("module", "instagram").Info("Started Instagram loop (10m)")
}

func (m *Instagram) checkInstagramFeedsLoop() {
	log := cache.GetLogger()

	defer helpers.Recover()
	defer func() {
		go func() {
			log.WithField("module", "instagram").Error("The checkInstagramFeedsLoop died. Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			m.checkInstagramFeedsLoop()
		}()
	}()

	var entries []DB_Instagram_Entry
	var bundledEntries map[string][]DB_Instagram_Entry

	for {
		cursor, err := rethink.Table("instagram").Run(helpers.GetDB())
		helpers.Relax(err)

		err = cursor.All(&entries)
		helpers.Relax(err)

		bundledEntries = make(map[string][]DB_Instagram_Entry, 0)

		for _, entry := range entries {
			channel, err := helpers.GetChannelWithoutApi(entry.ChannelID)
			if err != nil || channel == nil || channel.ID == "" {
				cache.GetLogger().WithField("module", "instagram").Warn(fmt.Sprintf("skipped instagram @%s for Channel #%s on Guild #%s: channel not found!",
					entry.Username, entry.ChannelID, entry.ServerID))
				continue
			}

			if _, ok := bundledEntries[entry.Username]; ok {
				bundledEntries[entry.Username] = append(bundledEntries[entry.Username], entry)
			} else {
				bundledEntries[entry.Username] = []DB_Instagram_Entry{entry}
			}
		}

		cache.GetLogger().WithField("module", "instagram").Infof("checking %d accounts for %d feeds", len(bundledEntries), len(entries))

		// TODO: Check multiple entries at once
		for instagramUsername, entries := range bundledEntries {
		RetryAccount:
			// log.WithField("module", "instagram").Debug(fmt.Sprintf("checking Instagram Account @%s", instagramUsername))

			instagramUser, err := instagramClient.GetUserByUsername(instagramUsername)
			if err != nil || instagramUser.User.Username == "" {
				if strings.Contains(err.Error(), "Please wait a few minutes before you try again.") {
					cache.GetLogger().WithField("module", "instagram").Info(fmt.Sprintf("hit rate limit checking Instagram Account @%s, sleeping for 20 seconds and then trying again", instagramUsername))
					time.Sleep(20 * time.Second)
					goto RetryAccount
				}
				log.WithField("module", "instagram").Error(fmt.Sprintf("updating instagram account @%s failed: %s", instagramUsername, err))
				continue
			}
			posts, err := instagramClient.LatestUserFeed(instagramUser.User.ID)
			if err != nil || instagramUser.User.Username == "" {
				if strings.Contains(err.Error(), "Please wait a few minutes before you try again.") {
					cache.GetLogger().WithField("module", "instagram").Info(fmt.Sprintf("hit rate limit checking Instagram Account @%s, sleeping for 20 seconds and then trying again", instagramUsername))
					time.Sleep(20 * time.Second)
					goto RetryAccount
				}
				log.WithField("module", "instagram").Error(fmt.Sprintf("updating instagram account @%s failed: %s", instagramUsername, err))
				continue
			}
			story, err := instagramClient.GetUserStories(instagramUser.User.ID)
			if err != nil || instagramUser.User.Username == "" {
				if strings.Contains(err.Error(), "Please wait a few minutes before you try again.") {
					cache.GetLogger().WithField("module", "instagram").Info(fmt.Sprintf("hit rate limit checking Instagram Account @%s, sleeping for 20 seconds and then trying again", instagramUsername))
					time.Sleep(20 * time.Second)
					goto RetryAccount
				}
				log.WithField("module", "instagram").Error(fmt.Sprintf("updating instagram account @%s failed: %s", instagramUsername, err))
				continue
			}

			// https://github.com/golang/go/wiki/SliceTricks#reversing
			for i := len(posts.Items)/2 - 1; i >= 0; i-- {
				opp := len(posts.Items) - 1 - i
				posts.Items[i], posts.Items[opp] = posts.Items[opp], posts.Items[i]
			}
			for i := len(story.Reel.Items)/2 - 1; i >= 0; i-- {
				opp := len(story.Reel.Items) - 1 - i
				story.Reel.Items[i], story.Reel.Items[opp] = story.Reel.Items[opp], story.Reel.Items[i]
			}

			for _, entry := range entries {
				changes := false
				for _, post := range posts.Items {
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

				for n, reelMedia := range story.Reel.Items {
					reelMediaAlreadyPosted := false
					for _, reelMediaPostPosted := range entry.PostedReelMedias {
						if reelMediaPostPosted.ID == reelMedia.ID {
							reelMediaAlreadyPosted = true
						}
					}
					if reelMediaAlreadyPosted == false {
						log.WithField("module", "instagram").Info(fmt.Sprintf("Posting Reel Media: #%s", reelMedia.ID))
						entry.PostedReelMedias = append(entry.PostedReelMedias, DB_Instagram_ReelMedia{ID: reelMedia.ID, CreatedAt: int64(reelMedia.DeviceTimestamp)})
						changes = true
						go m.postReelMediaToChannel(entry.ChannelID, story, n, instagramUser, entry.PostDirectLinks)
					}

				}

				// TODO: no broadcast information received from story anymore?
				/*
				   if entry.IsLive == false {
				       if story.Broadcast != 0 {
				           log.WithField("module", "instagram").Info(fmt.Sprintf("Posting Live: #%s", instagramUser.User.Broadcast.ID))
				           go m.postLiveToChannel(entry.ChannelID, instagramUser)
				           entry.IsLive = true
				           changes = true
				       }
				   } else {
				       if instagramUser.User.Broadcast.ID == 0 {
				           entry.IsLive = false
				           changes = true
				       }
				   }*/

				if changes == true {
					m.setEntry(entry)
				}
			}
		}

		if len(entries) <= 10 {
			time.Sleep(1 * time.Minute)
		}
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
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
						return
					}
				} else {
					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
					return
				}
				targetGuild, err = helpers.GetGuild(targetChannel.GuildID)
				helpers.Relax(err)
				// get instagram account
				instagramUsername := strings.Replace(args[1], "@", "", 1)
				instagramUser, err := instagramClient.GetUserByUsername(instagramUsername)
				if err != nil || instagramUser.User.Username == "" {
					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-not-found"))
					return
				}
				feed, err := instagramClient.LatestUserFeed(instagramUser.User.ID)
				if err != nil {
					if strings.Contains(err.Error(), "Please wait a few minutes before you try again.") {
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.instagram.ratelimited"))
						return
					}
				}
				helpers.Relax(err)
				story, err := instagramClient.GetUserStories(instagramUser.User.ID)
				if err != nil {
					if strings.Contains(err.Error(), "Please wait a few minutes before you try again.") {
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.instagram.ratelimited"))
						return
					}
				}
				helpers.Relax(err)
				// Create DB Entries
				var dbPosts []DB_Instagram_Post
				for _, post := range feed.Items {
					postEntry := DB_Instagram_Post{ID: post.ID, CreatedAt: post.Caption.CreatedAt}
					dbPosts = append(dbPosts, postEntry)

				}
				var dbRealMedias []DB_Instagram_ReelMedia
				for _, reelMedia := range story.Reel.Items {
					reelMediaEntry := DB_Instagram_ReelMedia{ID: reelMedia.ID, CreatedAt: reelMedia.DeviceTimestamp}
					dbRealMedias = append(dbRealMedias, reelMediaEntry)

				}
				// create new entry in db
				entry := m.getEntryByOrCreateEmpty("id", "")
				entry.ServerID = targetChannel.GuildID
				entry.ChannelID = targetChannel.ID
				entry.Username = instagramUser.User.Username
				entry.PostedPosts = dbPosts
				entry.PostedReelMedias = dbRealMedias
				m.setEntry(entry)

				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-added-success", entry.Username, entry.ChannelID))
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

						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-delete-success", entryBucket.Username))
						cache.GetLogger().WithField("module", "instagram").Info(fmt.Sprintf("Deleted Instagram Account @%s", entryBucket.Username))
					} else {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.instagram.account-delete-not-found-error"))
						return
					}
				} else {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
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
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-list-no-accounts-error"))
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
				_, err = helpers.SendMessage(msg.ChannelID, resultPage)
				helpers.Relax(err)
			}
		case "toggle-direct-link", "toggle-direct-links": // [p]instagram toggle-direct-links <id>
			helpers.RequireMod(msg, func() {
				session.ChannelTyping(msg.ChannelID)
				if len(args) < 2 {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					return
				}
				entryId := args[1]
				entryBucket := m.getEntryBy("id", entryId)
				if entryBucket.ID == "" {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
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
				helpers.SendMessage(msg.ChannelID, messageText)
				return
			})
		default:
			session.ChannelTyping(msg.ChannelID)
			instagramUsername := strings.Replace(args[0], "@", "", 1)
			instagramUser, err := instagramClient.GetUserByUsername(instagramUsername)
			if err != nil || instagramUser.User.Username == "" {
				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-not-found"))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			}

			instagramNameModifier := ""
			if instagramUser.User.IsVerified {
				instagramNameModifier += " ☑"
			}
			if instagramUser.User.IsPrivate {
				instagramNameModifier += " 🔒"
			}
			if instagramUser.User.IsBusiness {
				instagramNameModifier += " 🏢"
			}
			if instagramUser.User.IsFavorite {
				instagramNameModifier += " ⭐"
			}
			accountEmbed := &discordgo.MessageEmbed{
				Title:     helpers.GetTextF("plugins.instagram.account-embed-title", instagramUser.User.FullName, instagramUser.User.Username, instagramNameModifier),
				URL:       fmt.Sprintf(instagramFriendlyUser, instagramUser.User.Username),
				Thumbnail: &discordgo.MessageEmbedThumbnail{URL: instagramUser.User.ProfilePicURL},
				Footer: &discordgo.MessageEmbedFooter{
					Text:    helpers.GetText("plugins.instagram.embed-footer"),
					IconURL: helpers.GetText("plugins.instagram.embed-footer-imageurl"),
				},
				Description: instagramUser.User.Biography,
				Fields: []*discordgo.MessageEmbedField{
					{Name: "Followers", Value: humanize.Comma(int64(instagramUser.User.FollowerCount)), Inline: true},
					{Name: "Following", Value: humanize.Comma(int64(instagramUser.User.FollowingCount)), Inline: true},
					{Name: "Posts", Value: humanize.Comma(int64(instagramUser.User.MediaCount)), Inline: true}},
				Color: helpers.GetDiscordColorFromHex(hexColor),
			}
			if instagramUser.User.ExternalURL != "" {
				accountEmbed.Fields = append(accountEmbed.Fields, &discordgo.MessageEmbedField{
					Name:   "Website",
					Value:  instagramUser.User.ExternalURL,
					Inline: true,
				})
			}
			_, err = helpers.SendComplex(
				msg.ChannelID, &discordgo.MessageSend{
					Content: fmt.Sprintf("<%s>", fmt.Sprintf(instagramFriendlyUser, instagramUser.User.Username)),
					Embed:   accountEmbed,
				})
			helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
			return
		}
	} else {
		helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
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
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.instagram.embed-footer"),
			IconURL: helpers.GetText("plugins.instagram.embed-footer-imageurl"),
		},
		Image: &discordgo.MessageEmbedImage{URL: instagramUser.Broadcast.CoverFrameURL},
		Color: helpers.GetDiscordColorFromHex(hexColor),
	}

	mediaUrl := channelEmbed.URL

	_, err := helpers.SendComplex(channelID, &discordgo.MessageSend{
		Content: fmt.Sprintf("<%s>", mediaUrl),
		Embed:   channelEmbed,
	})
	if err != nil {
		cache.GetLogger().WithField("module", "instagram").Errorf("posting broadcast: #%d to channel: #%s failed: %s", instagramUser.Broadcast.ID, channelID, err.Error())
	}
}

func (m *Instagram) postReelMediaToChannel(channelID string, story goinstaResponse.StoryResponse, number int, instagramUser goinstaResponse.GetUsernameResponse, postDirectLinks bool) {
	instagramNameModifier := ""
	if instagramUser.User.IsVerified {
		instagramNameModifier += " ☑"
	}
	if instagramUser.User.IsPrivate {
		instagramNameModifier += " 🔒"
	}
	if instagramUser.User.IsBusiness {
		instagramNameModifier += " 🏢"
	}
	if instagramUser.User.IsFavorite {
		instagramNameModifier += " ⭐"
	}

	reelMedia := story.Reel.Items[number]

	mediaModifier := "Picture"
	if reelMedia.MediaType == 2 {
		mediaModifier = "Video"
	}

	caption := ""
	if captionData, ok := reelMedia.Caption.(map[string]interface{}); ok {
		caption, _ = captionData["text"].(string)
	}

	var content string
	channelEmbed := &discordgo.MessageEmbed{
		Title:     helpers.GetTextF("plugins.instagram.reelmedia-embed-title", instagramUser.User.FullName, instagramUser.User.Username, instagramNameModifier, mediaModifier),
		URL:       fmt.Sprintf(instagramFriendlyUser, instagramUser.User.Username),
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: instagramUser.User.ProfilePicURL},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.instagram.embed-footer"),
			IconURL: helpers.GetText("plugins.instagram.embed-footer-imageurl"),
		},
		Description: caption,
		Color:       helpers.GetDiscordColorFromHex(hexColor),
	}
	if postDirectLinks {
		content += "**" + helpers.GetTextF("plugins.instagram.reelmedia-embed-title", instagramUser.User.FullName, instagramUser.User.Username, instagramNameModifier, mediaModifier) + "** _" + helpers.GetText("plugins.instagram.embed-footer") + "_\n"
		if caption != "" {
			content += caption + "\n"
		}
	}

	mediaUrl := ""
	thumbnailUrl := ""

	if len(reelMedia.ImageVersions2.Candidates) > 0 {
		channelEmbed.Image = &discordgo.MessageEmbedImage{URL: reelMedia.ImageVersions2.Candidates[0].URL}
		mediaUrl = reelMedia.ImageVersions2.Candidates[0].URL
	}
	if len(reelMedia.VideoVersions) > 0 {
		channelEmbed.Video = &discordgo.MessageEmbedVideo{
			URL: reelMedia.VideoVersions[0].URL, Height: reelMedia.VideoVersions[0].Height, Width: reelMedia.VideoVersions[0].Width}
		if mediaUrl != "" {
			thumbnailUrl = mediaUrl
		}
		mediaUrl = reelMedia.VideoVersions[0].URL
	}

	if mediaUrl != "" {
		channelEmbed.URL = mediaUrl
	} else {
		mediaUrl = channelEmbed.URL
	}

	content += mediaUrl + "\n"
	if thumbnailUrl != "" {
		content += thumbnailUrl + "\n"
	}

	messageSend := &discordgo.MessageSend{
		Content: content,
	}
	if !postDirectLinks {
		messageSend.Content = fmt.Sprintf("<%s>", mediaUrl)
		messageSend.Embed = channelEmbed
	}

	_, err := helpers.SendComplex(channelID, messageSend)
	if err != nil {
		cache.GetLogger().WithField("module", "instagram").Errorf("posting reel media: #%s to channel: #%s failed: %s", reelMedia.ID, channelID, err.Error())
	}
}

func (m *Instagram) postPostToChannel(channelID string, post goinstaResponse.Item, instagramUser goinstaResponse.GetUsernameResponse, postDirectLinks bool) {
	instagramNameModifier := ""
	if instagramUser.User.IsVerified {
		instagramNameModifier += " ☑"
	}
	if instagramUser.User.IsPrivate {
		instagramNameModifier += " 🔒"
	}
	if instagramUser.User.IsBusiness {
		instagramNameModifier += " 🏢"
	}
	if instagramUser.User.IsFavorite {
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
		Title:     helpers.GetTextF("plugins.instagram.post-embed-title", instagramUser.User.FullName, instagramUser.User.Username, instagramNameModifier, mediaModifier),
		URL:       fmt.Sprintf(instagramFriendlyPost, post.Code),
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: instagramUser.User.ProfilePicURL},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.instagram.embed-footer"),
			IconURL: helpers.GetText("plugins.instagram.embed-footer-imageurl"),
		},
		Description: post.Caption.Text,
		Color:       helpers.GetDiscordColorFromHex(hexColor),
	}
	if postDirectLinks {
		content += "**" + helpers.GetTextF("plugins.instagram.post-embed-title", instagramUser.User.FullName, instagramUser.User.Username, instagramNameModifier, mediaModifier) + "** _" + helpers.GetText("plugins.instagram.embed-footer") + "_\n"
		if post.Caption.Text != "" {
			content += post.Caption.Text + "\n"
		}
	}

	if len(post.ImageVersions2.Candidates) > 0 {
		channelEmbed.Image = &discordgo.MessageEmbedImage{URL: getFullResUrl(post.ImageVersions2.Candidates[0].URL)}
	}
	if len(post.CarouselMedia) > 0 && len(post.CarouselMedia[0].ImageVersions.Candidates) > 0 {
		channelEmbed.Image = &discordgo.MessageEmbedImage{URL: getFullResUrl(post.CarouselMedia[0].ImageVersions.Candidates[0].URL)}
	}

	mediaUrls := make([]string, 0)
	if len(post.CarouselMedia) <= 0 {
		mediaUrls = append(mediaUrls, getFullResUrl(post.ImageVersions2.Candidates[0].URL))
	} else {
		for _, carouselMedia := range post.CarouselMedia {
			if len(carouselMedia.VideoVersions) > 0 {
				mediaUrls = append(mediaUrls, getFullResUrl(carouselMedia.VideoVersions[0].URL))
			} else {
				mediaUrls = append(mediaUrls, getFullResUrl(carouselMedia.ImageVersions.Candidates[0].URL))
			}
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

	_, err := helpers.SendComplex(channelID, messageSend)
	if err != nil {
		cache.GetLogger().WithField("module", "instagram").Error(fmt.Sprintf("posting post: #%s to channel: #%s failed: %s", post.ID, channelID, err))
	}
}

// breaks reel media links!
func getFullResUrl(url string) string {
	result := instagramPicUrlRegex.FindStringSubmatch(url)
	if result != nil && len(result) >= 8 {
		return result[1] + result[7]
	}
	return url
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
