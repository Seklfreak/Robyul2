package instagram

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"sync"

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

type Handler struct{}

var (
	instagramClient      *goinsta.Instagram
	instagramPicUrlRegex *regexp.Regexp
	useGraphQlQuery      = true
	lockPostedPosts      sync.Mutex
)

const (
	hexColor                 = "#fcaf45"
	instagramFriendlyUser    = "https://www.instagram.com/%s/"
	instagramFriendlyPost    = "https://www.instagram.com/p/%s/"
	instagramPicUrlRegexText = `(http(s)?\:\/\/[^\/]+\/[^\/]+\/)([a-z0-9\.]+\/)?([a-z0-9\.]+\/)?([a-z0-9]+x[a-z0-9]+\/)?([a-z0-9\.]+\/)?(([a-z0-9]+\/)?.+\.jpg)`
	instagramSessionKey      = "robyul2-discord:instagram:session"
)

func (m *Handler) Commands() []string {
	return []string{
		"instagram",
	}
}

func (m *Handler) Init(session *discordgo.Session) {
	go func() {
		defer helpers.Recover()
		// TODO: add retry

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
		err = instagramClient.Login()
		helpers.Relax(err)
		cache.GetLogger().WithField("module", "instagram").Infof(
			"logged in to instagram as @%s",
			instagramClient.Informations.Username,
		)
		storedInstagram, err = goinstaStore.Export(instagramClient, make([]byte, 32))
		helpers.Relax(err)
		err = cache.GetRedisClient().Set(instagramSessionKey, storedInstagram, 0).Err()
		helpers.Relax(err)
		cache.GetLogger().WithField("module", "instagram").Infof(
			"stored instagram session in redis",
		)

		instagramPicUrlRegex, err = regexp.Compile(instagramPicUrlRegexText)
		helpers.Relax(err)

		go m.checkInstagramGraphQlFeedLoop()
		cache.GetLogger().WithField("module", "instagram").Info("Started Instagram GraphQl Feed loop")

		go m.checkInstagramFeedsAndStoryLoop()
		cache.GetLogger().WithField("module", "instagram").Info("Started Instagram Feeds and Story loop")
	}()
}

func (m *Handler) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermInstagram) {
		return
	}

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
					targetChannel, err = helpers.GetChannelFromMention(msg, args[2])
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
					if err != nil && strings.Contains(err.Error(), "Please wait a few minutes before you try again.") {
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.instagram.ratelimited"))
						return
					}
				}
				helpers.Relax(err)
				story, err := instagramClient.GetUserStories(instagramUser.User.ID)
				if err != nil {
					if err != nil && strings.Contains(err.Error(), "Please wait a few minutes before you try again.") {
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
				var specialText string
				var linkMode bool
				if strings.HasSuffix(content, " direct link mode") ||
					strings.HasSuffix(content, " link mode") ||
					strings.HasSuffix(content, " links") {
					linkMode = true
					specialText += " using direct links"
				}

				entry := m.getEntryByOrCreateEmpty("id", "")
				entry.ServerID = targetChannel.GuildID
				entry.ChannelID = targetChannel.ID
				entry.Username = instagramUser.User.Username
				entry.PostedPosts = dbPosts
				entry.PostedReelMedias = dbRealMedias
				entry.PostDirectLinks = linkMode
				entry.InstagramUserID = instagramUser.User.ID
				m.setEntry(entry)

				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-added-success", entry.Username, entry.ChannelID, specialText))
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
				var directLinkModeText string
				if entry.PostDirectLinks {
					directLinkModeText = " (direct link mode)"
				}

				resultMessage += fmt.Sprintf("`%s`: Instagram Account `@%s` posting to <#%s>%s\n",
					entry.ID, entry.Username, entry.ChannelID, directLinkModeText)
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
					Text: helpers.GetTextF("plugins.instagram.account-embed-footer", instagramUser.User.ID) + " | " +
						helpers.GetText("plugins.instagram.embed-footer"),
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

func (m *Handler) postLiveToChannel(channelID string, instagramUser Instagram_User) {
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
		cache.GetLogger().WithField("module", "instagram").Warnf("posting broadcast: #%d to channel: #%s failed: %s", instagramUser.Broadcast.ID, channelID, err.Error())
	}
}

func (m *Handler) postReelMediaToChannel(channelID string, story goinstaResponse.StoryResponse, number int, postDirectLinks bool) {
	instagramNameModifier := ""
	if story.Reel.User.IsVerified {
		instagramNameModifier += " ☑"
	}
	if story.Reel.User.IsPrivate {
		instagramNameModifier += " 🔒"
	}
	/*
		if story.Reel.User.IsBusiness {
			instagramNameModifier += " 🏢"
		}
		if story.Reel.User.IsFavorite {
			instagramNameModifier += " ⭐"
		}
	*/

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
		Title:     helpers.GetTextF("plugins.instagram.reelmedia-embed-title", story.Reel.User.FullName, story.Reel.User.Username, instagramNameModifier, mediaModifier),
		URL:       fmt.Sprintf(instagramFriendlyUser, story.Reel.User.Username),
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: story.Reel.User.ProfilePicURL},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.instagram.embed-footer"),
			IconURL: helpers.GetText("plugins.instagram.embed-footer-imageurl"),
		},
		Description: caption,
		Color:       helpers.GetDiscordColorFromHex(hexColor),
	}
	if postDirectLinks {
		content += "**" + helpers.GetTextF("plugins.instagram.reelmedia-embed-title", story.Reel.User.FullName, story.Reel.User.Username, instagramNameModifier, mediaModifier) + "** _" + helpers.GetText("plugins.instagram.embed-footer") + "_\n"
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
		cache.GetLogger().WithField("module", "instagram").Warnf("posting reel media: #%s to channel: #%s failed: %s", reelMedia.ID, channelID, err.Error())
	}
}

func (m *Handler) postPostToChannel(channelID string, post goinstaResponse.Item, postDirectLinks bool) {
	instagramNameModifier := ""
	if post.User.IsVerified {
		instagramNameModifier += " ☑"
	}
	if post.User.IsPrivate {
		instagramNameModifier += " 🔒"
	}
	/*
		if post.User.IsBusiness {
			instagramNameModifier += " 🏢"
		}
	*/
	if post.User.IsFavorite {
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
		Title:     helpers.GetTextF("plugins.instagram.post-embed-title", post.User.FullName, post.User.Username, instagramNameModifier, mediaModifier),
		URL:       fmt.Sprintf(instagramFriendlyPost, post.Code),
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: post.User.ProfilePicURL},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.instagram.embed-footer"),
			IconURL: helpers.GetText("plugins.instagram.embed-footer-imageurl"),
		},
		Description: post.Caption.Text,
		Color:       helpers.GetDiscordColorFromHex(hexColor),
	}
	if postDirectLinks {
		content += "**" + helpers.GetTextF("plugins.instagram.post-embed-title", post.User.FullName, post.User.Username, instagramNameModifier, mediaModifier) + "** _" + helpers.GetText("plugins.instagram.embed-footer") + "_\n"
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
		if len(post.VideoVersions) > 0 {
			mediaUrls = append(mediaUrls, getFullResUrl(post.VideoVersions[0].URL))
		} else {
			mediaUrls = append(mediaUrls, getFullResUrl(post.ImageVersions2.Candidates[0].URL))
		}
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
		cache.GetLogger().WithField("module", "instagram").Warnf("posting post: #%s to channel: #%s failed: %s", post.ID, channelID, err)
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

func (m *Handler) getEntryBy(key string, id string) DB_Instagram_Entry {
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

func (m *Handler) getEntryByOrCreateEmpty(key string, id string) DB_Instagram_Entry {
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

func (m *Handler) setEntry(entry DB_Instagram_Entry) {
	_, err := rethink.Table("instagram").Update(entry).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}

func (m *Handler) deleteEntryById(id string) {
	_, err := rethink.Table("instagram").Filter(
		rethink.Row.Field("id").Eq(id),
	).Delete().RunWrite(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
