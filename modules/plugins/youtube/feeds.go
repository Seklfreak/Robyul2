package youtube

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Seklfreak/Robyul2/modules/plugins/youtube/service"
	"github.com/bwmarrin/discordgo"
	rethink "github.com/gorethink/gorethink"
	"github.com/sirupsen/logrus"
)

type feeds struct {
	service *service.Service
	running uint32
}

func (f *feeds) Init(e *service.Service) {
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
	entries, err := readEntries(rethink.Row.Field("next_check_time").Le(t))
	helpers.Relax(err)

	for _, e := range entries {
		e = f.checkChannelFeeds(e)

		// update next check time
		e = f.setNextCheckTime(e)
		err := updateEntry(e)
		helpers.Relax(err)
	}
}

func (f *feeds) checkChannelFeeds(e models.YoutubeChannelEntry) models.YoutubeChannelEntry {
	// set iso8601 time which will be used search query filter "published after"
	lastSuccessfulCheckTime := time.Unix(e.LastSuccessfulCheckTime, 0)
	publishedAfter := lastSuccessfulCheckTime.
		Add(time.Duration(-1) * time.Hour).
		Format("2006-01-02T15:04:05-0700")

	// get updated feeds
	feeds, err := f.service.GetChannelFeeds(e.YoutubeChannelID, publishedAfter)
	if err != nil {
		logger().Warn("check channel feeds error: " + err.Error() + " channel name: " + e.YoutubeChannelName + "id: " + e.YoutubeChannelID)
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
