package youtube

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
	humanize "github.com/dustin/go-humanize"
	rethink "github.com/gorethink/gorethink"
)

type YouTube struct {
	service          service
	quota            quota
	feedsLoopRunning bool

	// 1) Every jobs which use youtube API calls, must hold read lock.
	// 2) Change YouTube struct fields(except quota: quota has an own lock)
	//    must hold write lock.
	sync.RWMutex
}

type youtubeAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next youtubeAction)

const (
	youtubeChannelBaseUrl string = "https://www.youtube.com/channel/%s"
	youtubeVideoBaseUrl   string = "https://youtu.be/%s"
	youtubeColor          string = "cd201f"

	youtubeConfigFileName string = "google.client_credentials_json_location"
)

func (yt *YouTube) Commands() []string {
	return []string{
		"youtube",
		"yt",
	}
}

func (yt *YouTube) Init(session *discordgo.Session) {
	yt.Lock()
	defer yt.Unlock()
	defer helpers.Recover()

	yt.service.Init(youtubeConfigFileName)
	yt.runYoutubeFeedsLoop()
}

func (yt *YouTube) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	yt.RLock()
	defer yt.RUnlock()
	defer helpers.Recover()

	session.ChannelTyping(msg.ChannelID)

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := yt.actionStart
	for action != nil {
		action = action(args, msg, &result)
	}
}

func (yt *YouTube) actionStart(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	if len(args) < 1 {
		*out = yt.newMsg("bot.arguments.too-few")
		return yt.actionFinish
	}

	switch args[0] {
	case "video":
		return yt.actionVideo
	case "channel":
		return yt.actionChannel
	case "service":
		return yt.actionSystem
	case "quota":
		return yt.actionQuota
	default:
		return yt.actionSearch
	}
}

func (yt *YouTube) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	_, err := cache.GetSession().ChannelMessageSendComplex(in.ChannelID, *out)
	helpers.Relax(err)

	return nil
}

// DEBUG PURPOSE
func (yt *YouTube) actionQuota(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	if helpers.IsBotAdmin(in.Author.ID) == false {
		*out = yt.newMsg("botadmin.no_permission")
		return yt.actionFinish
	}

	q := yt.quota.GetQuota()

	msg := fmt.Sprintf("next reset: `%s`, left quota: `%s`, channel count: `%s`, time interval: `%s`",
		time.Unix(q.ResetTime, 0).Format(time.ANSIC),
		humanize.Comma(q.Left),
		humanize.Comma(yt.quota.GetCount()),
		humanize.Comma(yt.quota.GetInterval()))

	*out = yt.newMsg(msg)

	return yt.actionFinish
}

// _yt video <search by keywords...>
func (yt *YouTube) actionVideo(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	if len(args) < 2 {
		*out = yt.newMsg("bot.arguments.too-few")
		return yt.actionFinish
	}

	/* TODO:
	switch args[1] {
	case "add":
		return yt.actionAddVideo
	case "delete":
		return yt.actionDeleteVideo
	case "list":
		return yt.actionListVideo
	}
	*/

	yt.quota.Sub(searchQuotaCost)
	item, err := yt.service.SearchQuerySingle(args[1:], "video")
	if err != nil {
		yt.logger().Error(err)
		err = yt.handleGoogleAPIError(err)
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}

	if item == nil {
		*out = yt.newMsg("plugins.youtube.video-not-found")
		return yt.actionFinish
	}

	*out = yt.getVideoInfo(item.Id.VideoId)
	return yt.actionFinish
}

// _yt video add <video id/link/search keywords> <discord channel>
func (yt *YouTube) actionAddVideo(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	*out = yt.newMsg("bot.arguments.invalid")
	return yt.actionFinish
}

// _yt video delete <video id> <discord channel>
func (yt *YouTube) actionDeleteVideo(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	*out = yt.newMsg("bot.arguments.invalid")
	return yt.actionFinish
}

// _yt video list
func (yt *YouTube) actionListVideo(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	*out = yt.newMsg("bot.arguments.invalid")
	return yt.actionFinish
}

// _yt channel <search by keywords...>
func (yt *YouTube) actionChannel(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	if len(args) < 2 {
		*out = yt.newMsg("bot.arguments.too-few")
		return yt.actionFinish
	}

	switch args[1] {
	case "add":
		return yt.actionAddChannel
	case "delete":
		return yt.actionDeleteChannel
	case "list":
		return yt.actionListChannel
	}

	yt.quota.Sub(searchQuotaCost)
	item, err := yt.service.SearchQuerySingle(args[1:], "channel")
	if err != nil {
		yt.logger().Error(err)
		err = yt.handleGoogleAPIError(err)
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}

	if item == nil {
		*out = yt.newMsg("plugins.youtube.channel-not-found")
		return yt.actionFinish
	}

	// Very few channels only have snippet.ChannelID
	// Maybe it's youtube API bug.
	channelId := item.Id.ChannelId
	if channelId == "" {
		channelId = item.Snippet.ChannelId
	}
	*out = yt.getChannelInfo(channelId)

	return yt.actionFinish
}

// _yt channel add <channel id/link/search keywords> <discord channel>
func (yt *YouTube) actionAddChannel(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	if len(args) < 4 {
		*out = yt.newMsg("bot.arguments.too-few")
		return yt.actionFinish
	}

	// check permission
	if helpers.IsMod(in) == false {
		*out = yt.newMsg("mod.no_permission")
		return yt.actionFinish
	}

	// check discord channel
	dc, err := helpers.GetChannelFromMention(in, args[len(args)-1])
	if err != nil {
		*out = yt.newMsg("bot.arguments.invalid")
		return yt.actionFinish
	}

	// search channel
	yt.quota.Sub(searchQuotaCost)
	yc, err := yt.service.SearchQuerySingle(args[2:len(args)-1], "channel")
	if err != nil {
		yt.logger().Error(err)
		err = yt.handleGoogleAPIError(err)
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}

	if yc == nil {
		*out = yt.newMsg("plugins.youtube.channel-not-found")
		return yt.actionFinish
	}

	entry := models.YoutubeChannelEntry{
		ServerID:                dc.GuildID,
		ChannelID:               dc.ID,
		NextCheckTime:           time.Now().Unix(),
		LastSuccessfulCheckTime: time.Now().Unix(),

		YoutubeChannelID:   yc.Id.ChannelId,
		YoutubeChannelName: yc.Snippet.ChannelTitle,
	}

	// Very few channels only have snippet.ChannelID
	// Maybe it's youtube API bug.
	if entry.YoutubeChannelID == "" {
		entry.YoutubeChannelID = yc.Snippet.ChannelId
	}

	if entry.YoutubeChannelID == "" || entry.YoutubeChannelName == "" {
		*out = yt.newMsg("plugins.youtube.channel-not-found")
		return yt.actionFinish
	}

	// insert entry into the db
	_, err = createEntry(entry)
	if err != nil {
		yt.logger().Error(err)
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}

	yt.quota.IncEntryCount()

	*out = yt.newMsg("plugins.youtube.channel-added-success", yc.Snippet.ChannelTitle, dc.ID)
	return yt.actionFinish
}

// _yt channel delete <channel id> <discord channel>
func (yt *YouTube) actionDeleteChannel(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	if len(args) < 3 {
		*out = yt.newMsg("bot.arguments.too-few")
		return yt.actionFinish
	}

	if helpers.IsMod(in) == false {
		*out = yt.newMsg("mod.no_permission")
		return yt.actionFinish
	}

	n, err := deleteEntry(args[2])
	if err != nil {
		yt.logger().Error(err)
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}

	if n < 1 {
		*out = yt.newMsg("plugins.youtube.channel-delete-not-found-error")
		return yt.actionFinish
	}

	yt.quota.DecEntryCount()

	*out = yt.newMsg("Delete channel, ID: " + args[2])
	return yt.actionFinish
}

// _yt channel list
func (yt *YouTube) actionListChannel(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	if helpers.IsMod(in) == false {
		*out = yt.newMsg("mod.no_permission")
		return yt.actionFinish
	}

	ch, err := helpers.GetChannel(in.ChannelID)
	if err != nil {
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}

	entries, err := readEntries(map[string]interface{}{
		"server_id": ch.GuildID,
	})
	if err != nil {
		yt.logger().Error(err)
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}

	if len(entries) < 1 {
		*out = yt.newMsg("plugins.youtube.no-entry")
		return yt.actionFinish
	}

	// TODO: pagify
	msg := ""
	for _, e := range entries {
		msg += helpers.GetTextF("plugins.youtube.channel-list-entry", e.ID, e.YoutubeChannelName, e.ChannelID)
	}

	for _, resultPage := range helpers.Pagify(msg, "\n") {
		_, err := cache.GetSession().ChannelMessageSend(in.ChannelID, resultPage)
		helpers.Relax(err)
	}

	*out = yt.newMsg("plugins.youtube.channel-list-sum", len(entries))
	return yt.actionFinish
}

// _yt system restart
func (yt *YouTube) actionSystem(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	if len(args) < 2 {
		*out = yt.newMsg("bot.arguments.too-few")
		return yt.actionFinish
	}

	if args[1] != "restart" {
		*out = yt.newMsg("bot.arguments.invalid")
		return yt.actionFinish
	}

	if helpers.IsBotAdmin(in.Author.ID) == false {
		*out = yt.newMsg("botadmin.no_permission")
		return yt.actionFinish
	}

	go yt.Init(nil)

	*out = yt.newMsg("plugins.youtube.service-restart")
	return yt.actionFinish
}

// _yt <video or channel search by keywords...>
func (yt *YouTube) actionSearch(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	yt.quota.Sub(searchQuotaCost)
	item, err := yt.service.SearchQuerySingle(args[0:], "channel, video")
	if err != nil {
		yt.logger().Error(err)
		*out = yt.newMsg(err.Error())
		err = yt.handleGoogleAPIError(err)
		return yt.actionFinish
	}

	if item == nil {
		*out = yt.newMsg("plugins.youtube.not-found")
		return yt.actionFinish
	}

	switch item.Id.Kind {
	case "youtube#video":
		*out = yt.getVideoInfo(item.Id.VideoId)
	case "youtube#channel":
		// Very few channels only have snippet.ChannelID
		// Maybe it's youtube API bug.
		channelId := item.Id.ChannelId
		if channelId == "" {
			channelId = item.Snippet.ChannelId
		}
		*out = yt.getChannelInfo(channelId)
	default:
		*out = yt.newMsg("plugins.youtube.video-not-found")
	}

	return yt.actionFinish
}

// getVideoInfo returns information of given video id through *discordgo.MessageSend.
func (yt *YouTube) getVideoInfo(videoId string) (data *discordgo.MessageSend) {
	yt.quota.Sub(videosQuotaCost)
	video, err := yt.service.GetVideoSingle(videoId)
	if err != nil {
		yt.logger().Error(err)
		err = yt.handleGoogleAPIError(err)
		return yt.newMsg(err.Error())
	}

	if video == nil {
		return yt.newMsg("plugins.youtube.video-not-found")
	}

	data = &discordgo.MessageSend{}

	data.Embed = &discordgo.MessageEmbed{
		Footer: &discordgo.MessageEmbedFooter{Text: "YouTube"},
		Author: &discordgo.MessageEmbedAuthor{
			Name: video.Snippet.ChannelTitle,
			URL:  fmt.Sprintf(youtubeChannelBaseUrl, video.Snippet.ChannelId),
		},
		Title: video.Snippet.Title,
		URL:   fmt.Sprintf(youtubeVideoBaseUrl, video.Id),
		Image: &discordgo.MessageEmbedImage{URL: video.Snippet.Thumbnails.High.Url},
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Views", Value: humanize.Comma(int64(video.Statistics.ViewCount)), Inline: true},
			{Name: "Likes", Value: humanize.Comma(int64(video.Statistics.LikeCount)), Inline: true},
			{Name: "Comments", Value: humanize.Comma(int64(video.Statistics.CommentCount)), Inline: true},
			{Name: "Published at", Value: yt.humanizeTime(video.Snippet.PublishedAt), Inline: true},
		},
		Color: helpers.GetDiscordColorFromHex(youtubeColor),
	}

	data.Embed.Fields = yt.verifyEmbedFields(data.Embed.Fields)

	return
}

// getChannelInfo returns information of given channel id through *discordgo.MessageSend.
func (yt *YouTube) getChannelInfo(channelId string) (data *discordgo.MessageSend) {
	yt.quota.Sub(channelsQuotaCost)
	channel, err := yt.service.GetChannelSingle(channelId)
	if err != nil {
		yt.logger().Error(err)
		err = yt.handleGoogleAPIError(err)
		return yt.newMsg(err.Error())
	}

	if channel == nil {
		return yt.newMsg("plugins.youtube.channel-not-found")
	}

	data = &discordgo.MessageSend{}

	data.Embed = &discordgo.MessageEmbed{
		Footer:      &discordgo.MessageEmbedFooter{Text: "YouTube"},
		Title:       channel.Snippet.Title,
		URL:         fmt.Sprintf(youtubeChannelBaseUrl, channel.Id),
		Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: channel.Snippet.Thumbnails.High.Url},
		Description: channel.Snippet.Description,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Views", Value: humanize.Comma(int64(channel.Statistics.ViewCount)), Inline: true},
			{Name: "Videos", Value: humanize.Comma(int64(channel.Statistics.VideoCount)), Inline: true},
			{Name: "Subscribers", Value: humanize.Comma(int64(channel.Statistics.SubscriberCount)), Inline: true},
			{Name: "Comments", Value: humanize.Comma(int64(channel.Statistics.CommentCount)), Inline: true},
			{Name: "Published at", Value: yt.humanizeTime(channel.Snippet.PublishedAt), Inline: true},
		},
		Color: helpers.GetDiscordColorFromHex(youtubeColor),
	}

	data.Embed.Fields = yt.verifyEmbedFields(data.Embed.Fields)

	return
}

// Youtube channel/video can hide their subscribers or comments and just return 0 to API calls.
// verifyEmbedFields trim hided statistic information field and invalid field with empty string.
func (yt *YouTube) verifyEmbedFields(fields []*discordgo.MessageEmbedField) []*discordgo.MessageEmbedField {
	for i := len(fields) - 1; i >= 0; i-- {
		if fields[i].Value == "0" || fields[i].Value == "" || fields[i].Name == "" {
			fields = append(fields[:i], fields[i+1:]...)
		}
	}

	return fields
}

func (yt *YouTube) humanizeTime(t string) string {
	parsedTime, err := time.Parse(time.RFC3339, t)
	if err != nil {
		yt.logger().Error(err)
		return t
	}

	year, month, day := parsedTime.Date()
	return fmt.Sprintf("%d-%d-%d", year, month, day)
}

func (yt *YouTube) newMsg(content string, replacements ...interface{}) *discordgo.MessageSend {
	if len(replacements) < 1 {
		return &discordgo.MessageSend{Content: helpers.GetText(content)}
	}
	return &discordgo.MessageSend{Content: helpers.GetTextF(content, replacements...)}
}

func (yt *YouTube) logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "youtube")
}

// yt.lock must held.
func (yt *YouTube) runYoutubeFeedsLoop() {
	if yt.feedsLoopRunning {
		yt.logger().Error("youtube feeds loop already running")
		return
	}
	yt.feedsLoopRunning = true

	go yt.youtubeFeedsLoop()

	return
}

func (yt *YouTube) youtubeFeedsLoop() {
	defer helpers.Recover()
	defer func() {
		yt.Lock()
		yt.feedsLoopRunning = false
		yt.Unlock()

		go func() {
			yt.logger().Error("The checkYoutubeFeedsLoop died. Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)

			yt.Lock()
			yt.runYoutubeFeedsLoop()
			yt.Unlock()
		}()
	}()

	if err := yt.quota.Init(); err != nil {
		helpers.Relax(err)
	}

	for ; ; time.Sleep(10 * time.Second) {
		err := yt.quota.UpdateCheckingInterval()
		helpers.Relax(err)

		yt.checkYoutubeFeeds()
	}
}

func (yt *YouTube) checkYoutubeFeeds() {
	yt.RLock()
	defer yt.RUnlock()

	t := time.Now().Unix()
	entries, err := readEntries(rethink.Row.Field("next_check_time").Le(t))
	helpers.Relax(err)

	for _, e := range entries {
		e = yt.checkYoutubeChannelFeeds(e)

		// update next check time
		e = yt.setNextCheckTime(e)
		err := updateEntry(e)
		helpers.Relax(err)
	}
}

func (yt *YouTube) checkYoutubeChannelFeeds(e models.YoutubeChannelEntry) models.YoutubeChannelEntry {
	// set iso8601 time which will be used search query filter "published after"
	lastSuccessfulCheckTime := time.Unix(e.LastSuccessfulCheckTime, 0)
	publishedAfter := lastSuccessfulCheckTime.
		Add(time.Duration(-1) * time.Hour).
		Format("2006-01-02T15:04:05-0700")

	// get updated feeds
	yt.quota.Sub(activityQuotaCost)
	feeds, err := yt.service.GetChannelFeeds(e.YoutubeChannelID, publishedAfter)
	if err != nil {
		yt.logger().Error("check channel feeds error: " + err.Error() + " channel name: " + e.YoutubeChannelName + "id: " + e.YoutubeChannelID)
		err = yt.handleGoogleAPIError(err)
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
		if yt.isPosted(videoId, e.YoutubePostedVideos) {
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

		_, err = cache.GetSession().ChannelMessageSendComplex(e.ChannelID, msg)
		if err != nil {
			yt.logger().Error(err)
			break
		}

		newPostedVideos = append(newPostedVideos, videoId)

		yt.logger().WithFields(logrus.Fields{
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

func (yt *YouTube) setNextCheckTime(e models.YoutubeChannelEntry) models.YoutubeChannelEntry {
	e.NextCheckTime = time.Now().
		Add(time.Duration(yt.quota.GetInterval()) * time.Second).
		Unix()

	return e
}

func (yt *YouTube) isPosted(id string, postedIds []string) bool {
	for _, posted := range postedIds {
		if id == posted {
			return true
		}
	}
	return false
}

func (yt *YouTube) handleGoogleAPIError(err error) error {
	var errCode int
	var errMsg string
	_, scanErr := fmt.Sscanf(err.Error(), "googleapi: Error %d: %s", &errCode, &errMsg)
	if scanErr != nil {
		return err
	}

	// Handle google API error by code
	switch errCode {
	case 403:
		yt.quota.DailyLimitExceeded()
		return fmt.Errorf("plugins.youtube.daily-limit-exceeded")
	default:
		return err
	}
}
