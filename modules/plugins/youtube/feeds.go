package youtube

import (
	"fmt"
	"sync/atomic"
	"time"

	"gopkg.in/mgo.v2/bson"

	youtubeService "github.com/Seklfreak/Robyul2/services/youtube"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

type feeds struct {
	service *youtubeService.Service
	running uint32
}

func (f *feeds) Init(e *youtubeService.Service) {
	if e == nil {
		helpers.Relax(fmt.Errorf("feeds loop initialize failed"))
	}
	f.service = e

	f.start()
}

func (f *feeds) start() {
	// Checks if feeds loop is already running
	if atomic.SwapUint32(&f.running, uint32(1)) == 1 {
		logger().Error("feeds loop already running")
		return
	}

	go f.run()
}

func (f *feeds) run() {
	defer helpers.Recover()
	defer func() {
		// Atomically set 'running' variable to 0(false)
		atomic.StoreUint32(&f.running, uint32(0))

		logger().Error("The feeds loop died. Please investigate! Will be restarted in 60 seconds")
		time.Sleep(60 * time.Second)

		f.start()
	}()

	for ; ; time.Sleep(10 * time.Second) {
		err := f.service.UpdateCheckingInterval()
		helpers.Relax(err)

		f.check()
	}
}

func (f *feeds) check() {
	t := time.Now().Unix()
	var entries []models.YoutubeChannelEntry
	err := helpers.MDbIterWithoutLogging(helpers.MdbCollection(models.YoutubeChannelTable).Find(
		bson.M{"nextchecktime": bson.M{"$lte": t}},
	)).All(&entries)
	helpers.Relax(err)
	// TODO: test

	for _, e := range entries {
		e = f.checkChannelFeeds(e)

		// update next check time
		e = f.setNextCheckTime(e)
		err = helpers.MDbUpdateWithoutLogging(models.YoutubeChannelTable, e.ID, e)
		helpers.Relax(err)
	}
}

func (f *feeds) checkChannelFeeds(e models.YoutubeChannelEntry) models.YoutubeChannelEntry {
	// check if we have access to channel
	channel, err := helpers.GetChannelWithoutApi(e.ChannelID)
	if err != nil || channel == nil || channel.ID == "" {
		return e
	}

	// check if we can send messages and embed links in channel
	channelPermission, err := cache.GetSession().SessionForGuildS(channel.GuildID).State.UserChannelPermissions(cache.GetSession().SessionForGuildS(channel.GuildID).State.User.ID, channel.ID)
	if err != nil {
		return e
	}

	if channelPermission&discordgo.PermissionSendMessages != discordgo.PermissionSendMessages ||
		channelPermission&discordgo.PermissionEmbedLinks != discordgo.PermissionEmbedLinks {
		return e
	}

	// set iso8601 time which will be used search query filter "published after"
	lastSuccessfulCheckTime := time.Unix(e.LastSuccessfulCheckTime, 0)
	publishedAfter := lastSuccessfulCheckTime.
		Add(time.Duration(-1) * time.Hour).
		Format("2006-01-02T15:04:05-0700")

	// get updated feeds
	feeds, err := f.service.GetChannelFeeds(e.YoutubeChannelID, publishedAfter)
	if err != nil {
		logger().Warn("check channel feeds error: " + err.Error() + " id: " + e.YoutubeChannelID)
		return e
	}

	newPostedVideos := make([]string, 0)
	alreadyPostedVideos := make([]string, 0)

	// check if posted videos and post new videos
	for i := len(feeds) - 1; i >= 0; i-- {
		feed := feeds[i]

		// only 'upload' type video can be posted
		if feed.Snippet.Type != "upload" {
			continue
		}
		videoId := feed.ContentDetails.Upload.VideoId

		// check if the video is already posted
		if f.isPosted(videoId, e.YoutubePostedVideos) {
			alreadyPostedVideos = append(alreadyPostedVideos, videoId)
			continue
		}

		// make a message and send to discord channel
		msg := &discordgo.MessageSend{
			Content: fmt.Sprintf(youtubeVideoBaseUrl, videoId),
			Embed: &discordgo.MessageEmbed{
				Author: &discordgo.MessageEmbedAuthor{
					Name: feed.Snippet.ChannelTitle,
					URL:  fmt.Sprintf(youtubeChannelBaseUrl, feed.Snippet.ChannelId),
				},
				Title:       helpers.GetTextF("plugins.youtube.channel-embed-title-vod", feed.Snippet.ChannelTitle),
				URL:         fmt.Sprintf(youtubeVideoBaseUrl, videoId),
				Description: fmt.Sprintf("**%s**", feed.Snippet.Title),
				Image:       &discordgo.MessageEmbedImage{URL: feed.Snippet.Thumbnails.High.Url},
				Footer:      &discordgo.MessageEmbedFooter{Text: "YouTube"},
				Color:       helpers.GetDiscordColorFromHex(youtubeColor),
			},
		}

		_, err = helpers.SendComplex(e.ChannelID, msg)
		if err != nil {
			logger().Warn(err)
			break
		}

		newPostedVideos = append(newPostedVideos, videoId)

		logger().WithFields(logrus.Fields{
			"title":   feed.Snippet.Title,
			"channel": e.ChannelID,
		}).Info("posting video")
	}

	if err == nil {
		e.LastSuccessfulCheckTime = e.NextCheckTime
		e.YoutubePostedVideos = alreadyPostedVideos
	}
	e.YoutubePostedVideos = append(e.YoutubePostedVideos, newPostedVideos...)

	return e
}

func (f *feeds) setNextCheckTime(e models.YoutubeChannelEntry) models.YoutubeChannelEntry {
	e.NextCheckTime = time.Now().
		Add(time.Duration(f.service.GetCheckingInterval()) * time.Second).
		Unix()

	return e
}

func (f *feeds) isPosted(id string, postedIds []string) bool {
	for _, posted := range postedIds {
		if id == posted {
			return true
		}
	}
	return false
}
