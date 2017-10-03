package plugins

import (
	"errors"
	"fmt"
	"io/ioutil"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
	humanize "github.com/dustin/go-humanize"
	rethink "github.com/gorethink/gorethink"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/youtube/v3"
)

type YouTube struct {
	service          *youtube.Service
	regexpSet        []*regexp.Regexp
	feedsLoopRunning bool
	quota            youtubeQuota

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
	youtubeDbTableName    string = "youtube"

	// for yt.regexpSet
	videoLongUrl   string = `^(https?\:\/\/)?(www\.)?(youtube\.com)\/watch\?v=(.[A-Za-z0-9_]*)`
	videoShortUrl  string = `^(https?\:\/\/)?(youtu\.be)\/(.[A-Za-z0-9_]*)`
	channelIdUrl   string = `^(https?\:\/\/)?(www\.)?(youtube\.com)\/channel\/(.[A-Za-z0-9_]*)`
	channelUserUrl string = `^(https?\:\/\/)?(www\.)?(youtube\.com)\/user\/(.[A-Za-z0-9_]*)`

	dailyQuota    int64 = 1000000
	activityQuota int64 = 5
	searchQuota   int64 = 100
	videosQuota   int64 = 5
	channelsQuota int64 = 5
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

	yt.service = nil

	yt.initQuota()
	yt.compileRegexpSet(videoLongUrl, videoShortUrl, channelIdUrl, channelUserUrl)
	yt.newYoutubeService()
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
	default:
		return yt.actionSearch
	}
}

func (yt *YouTube) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	_, err := cache.GetSession().ChannelMessageSendComplex(in.ChannelID, *out)
	helpers.Relax(err)

	return nil
}

// _yt video <search by keywords...>
func (yt *YouTube) actionVideo(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	if len(args) < 2 {
		*out = yt.newMsg("bot.arguments.too-few")
		return yt.actionFinish
	}

	switch args[1] {
	case "add":
		return yt.actionAddVideo
	case "delete":
		return yt.actionDeleteVideo
	case "list":
		return yt.actionListVideo
	}

	item, err := yt.searchQuerySingle(args[1:], "video")
	if err != nil {
		yt.logger().Error(err)
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

	item, err := yt.searchQuerySingle(args[1:], "channel")
	if err != nil {
		yt.logger().Error(err)
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
	yc, err := yt.searchQuerySingle(args[2:len(args)-1], "channel")
	if err != nil {
		yt.logger().Error(err)
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}

	if yc == nil {
		*out = yt.newMsg("plugins.youtube.channel-not-found")
		return yt.actionFinish
	}

	// fill content with default timestamp(channel published date)
	content := DB_Youtube_Content_Channel{
		ID:   yc.Id.ChannelId,
		Name: yc.Snippet.ChannelTitle,
	}

	// Very few channels only have snippet.ChannelID
	// Maybe it's youtube API bug.
	if content.ID == "" {
		content.ID = yc.Snippet.ChannelId
	}

	if content.ID == "" || content.Name == "" {
		*out = yt.newMsg("plugins.youtube.channel-not-found")
		return yt.actionFinish
	}

	// fill db entry
	entry := DB_Youtube_Entry{
		ServerID:                dc.GuildID,
		ChannelID:               dc.ID,
		NextCheckTime:           time.Now().Unix(),
		LastSuccessfulCheckTime: time.Now().Unix(),

		ContentType: "channel",
		Content:     content,
	}

	// insert entry into the db
	_, err = yt.createEntry(entry)
	if err != nil {
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}

	yt.quota.incEntryCount()

	*out = yt.newMsg("Added youtube channel <" + yc.Snippet.ChannelTitle + "> to the discord channel " + dc.ID)
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

	n, err := yt.deleteEntry(args[2])
	if err != nil {
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}

	if n < 1 {
		*out = yt.newMsg("plugins.youtube.channel-delete-not-found-error")
		return yt.actionFinish
	}

	yt.quota.decEntryCount()

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

	entries, err := yt.readEntries(map[string]interface{}{
		"content_type": "channel",
		"server_id":    ch.GuildID,
	})
	if err != nil {
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}

	if len(entries) < 1 {
		*out = yt.newMsg("No entries")
		return yt.actionFinish
	}

	msg := ""
	for _, e := range entries {
		c, err := e.getChannelContent()
		if err != nil {
			continue
		}
		msg += fmt.Sprintf("`%s`: Youtube channel name `@%s` posting to <#%s>\n", e.ID, c.Name, e.ChannelID)
	}

	for _, resultPage := range helpers.Pagify(msg, "\n") {
		_, err := cache.GetSession().ChannelMessageSend(in.ChannelID, resultPage)
		helpers.Relax(err)
	}

	msg = fmt.Sprintf("Found **%d** Youtube channel in total.", len(entries))

	*out = yt.newMsg(msg)
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
	item, err := yt.searchQuerySingle(args[0:], "channel, video")
	if err != nil {
		yt.logger().Error(err)
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}

	if item == nil {
		*out = yt.newMsg("plugins.youtube.video-not-found")
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

// searchQuerySingle retuns single search result with given type @searchType.
// returns (nil, nil) when there is no matching results.
func (yt *YouTube) searchQuerySingle(keywords []string, searchType string) (*youtube.SearchResult, error) {
	if yt.service == nil {
		return nil, errors.New("plugins.youtube.service-not-available")
	}

	call := yt.service.Search.List("id, snippet").
		Type(searchType).
		MaxResults(1)

	items, err := yt.searchQuery(keywords, call)
	if err != nil {
		return nil, err
	}

	if len(items) < 1 {
		return nil, nil
	}

	return items[0], nil
}

// search returns searchQuery results with given keywords and searchListCall.
func (yt *YouTube) searchQuery(keywords []string, call *youtube.SearchListCall) ([]*youtube.SearchResult, error) {
	// extract ID from valid youtube url
	for i, w := range keywords {
		keywords[i], _ = yt.getIdFromUrl(w)
	}

	query := strings.Join(keywords, " ")

	call = call.Q(query)

	return yt.search(call)
}

// search returns search results with given searchListCall.
func (yt *YouTube) search(call *youtube.SearchListCall) ([]*youtube.SearchResult, error) {
	response, err := call.Do()
	if err != nil {
		return nil, err
	}

	return response.Items, nil
}

func (yt *YouTube) getChannelFeeds(channelId, publishedAfter string) ([]*youtube.Activity, error) {
	if yt.service == nil {
		return nil, errors.New("plugins.youtube.service-not-available")
	}

	call := yt.service.Activities.List("contentDetails, snippet").
		ChannelId(channelId).
		PublishedAfter(publishedAfter).
		MaxResults(50)

	yt.quota.sub(activityQuota)
	response, err := call.Do()
	if err != nil {
		return nil, err
	}

	return response.Items, nil
}

// getIdFromUrl extracts channel id, channel name, video id from given url.
func (yt *YouTube) getIdFromUrl(url string) (id string, ok bool) {
	// TODO: it failed to retrieve exact information from user name.
	// example) https://www.youtube.com/user/bruno
	for i := range yt.regexpSet {
		if yt.regexpSet[i].MatchString(url) {
			match := yt.regexpSet[i].FindStringSubmatch(url)
			return match[len(match)-1], true
		}
	}

	return url, false
}

// getVideoInfo returns information of given video id through *discordgo.MessageSend.
func (yt *YouTube) getVideoInfo(videoId string) (data *discordgo.MessageSend) {
	yt.quota.sub(videosQuota)
	call := yt.service.Videos.List("statistics, snippet").
		Id(videoId).
		MaxResults(1)

	response, err := call.Do()
	if err != nil {
		yt.logger().Error(err)
		return yt.newMsg(err.Error())
	}

	if len(response.Items) < 1 {
		return yt.newMsg("plugins.youtube.video-not-found")
	}
	video := response.Items[0]

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
	yt.quota.sub(channelsQuota)
	call := yt.service.Channels.List("statistics, snippet").
		Id(channelId).
		MaxResults(1)

	response, err := call.Do()
	if err != nil {
		yt.logger().Error(err)
		return yt.newMsg(err.Error())
	}

	if len(response.Items) < 1 {
		return yt.newMsg("plugins.youtube.channel-not-found")
	}
	channel := response.Items[0]

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

func (yt *YouTube) compileRegexpSet(regexps ...string) {
	for i := range yt.regexpSet {
		yt.regexpSet[i] = nil
	}
	yt.regexpSet = yt.regexpSet[:0]

	for i := range regexps {
		yt.regexpSet = append(yt.regexpSet, regexp.MustCompile(regexps[i]))
	}
}

func (yt *YouTube) newYoutubeService() {
	configFile := helpers.GetConfig().Path(youtubeConfigFileName).Data().(string)

	authJSON, err := ioutil.ReadFile(configFile)
	helpers.Relax(err)

	config, err := google.JWTConfigFromJSON(authJSON, youtube.YoutubeReadonlyScope)
	helpers.Relax(err)

	client := config.Client(context.Background())

	yt.service, err = youtube.New(client)
	helpers.Relax(err)
}

func (yt *YouTube) newMsg(content string) *discordgo.MessageSend {
	return &discordgo.MessageSend{Content: helpers.GetText(content)}
}

func (yt *YouTube) logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "youtube")
}

// RethinkDB CRUD wrapper functions.

func (yt *YouTube) createEntry(entry DB_Youtube_Entry) (id string, err error) {
	query := rethink.Table(youtubeDbTableName).Insert(entry)

	res, err := query.RunWrite(helpers.GetDB())
	if err != nil {
		return "", err
	}

	return res.GeneratedKeys[0], nil
}

func (yt *YouTube) readEntries(filter interface{}) (entry []DB_Youtube_Entry, err error) {
	query := rethink.Table(youtubeDbTableName).Filter(filter)

	cursor, err := query.Run(helpers.GetDB())
	if err != nil {
		return entry, err
	}
	defer cursor.Close()

	err = cursor.All(&entry)
	return
}

func (yt *YouTube) updateEntry(entry DB_Youtube_Entry) (err error) {
	query := rethink.Table(youtubeDbTableName).Update(entry)

	_, err = query.Run(helpers.GetDB())
	return
}

func (yt *YouTube) deleteEntry(id string) (n int, err error) {
	query := rethink.Table(youtubeDbTableName).Filter(rethink.Row.Field("id").Eq(id)).Delete()

	r, err := query.RunWrite(helpers.GetDB())
	if err == nil {
		n = r.Deleted
	}

	return
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

	for ; ; time.Sleep(10 * time.Second) {
		yt.quota.updateCheckingInterval()
		if err := yt.updateQuota(); err != nil {
			yt.logger().Error(err)
		}

		yt.checkYoutubeFeeds()
	}
}

func (yt *YouTube) checkYoutubeFeeds() {
	yt.RLock()
	defer yt.RUnlock()

	// DEBUG
	yt.logger().Error("quota left: ", yt.quota.content.Left)
	yt.logger().Error("entry count: ", yt.quota.count)
	yt.logger().Error("checking time interval: ", yt.quota.interval)

	t := time.Now().Unix()
	entries, err := yt.readEntries(rethink.Row.Field("next_check_time").Le(t))
	helpers.Relax(err)

	for _, e := range entries {
		switch e.ContentType {
		case "channel":
			e = yt.checkYoutubeChannelFeeds(e)
		case "quota":
			continue
		default:
			yt.logger().Error("unknown contents type: " + e.ContentType)
		}

		// update next check time
		e = yt.setNextCheckTime(e)
		err := yt.updateEntry(e)
		helpers.Relax(err)
	}
}

func (yt *YouTube) checkYoutubeChannelFeeds(e DB_Youtube_Entry) DB_Youtube_Entry {
	c, err := e.getChannelContent()
	if err != nil {
		yt.logger().Error(err)
		return e
	}

	// set iso8601 time which will be used search query filter "published after"
	lastSuccessfulCheckTime := time.Unix(e.LastSuccessfulCheckTime, 0)
	publishedAfter := lastSuccessfulCheckTime.
		Add(time.Duration(-1) * time.Hour).
		Format("2006-01-02T15:04:05-0700")

	// get updated feeds
	feeds, err := yt.getChannelFeeds(c.ID, publishedAfter)
	if err != nil {
		yt.logger().Error("check channel feeds error: " + err.Error() + " channel name: " + c.Name + "id: " + e.ID)
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
		if yt.isPosted(videoId, c.PostedVideos) {
			alreadyPostedVideos = append(alreadyPostedVideos, videoId)
			continue
		}

		// make a message and send to discord channel
		videoUrl := fmt.Sprintf(youtubeVideoBaseUrl, videoId)
		msg := yt.newMsg(videoUrl)

		_, err = cache.GetSession().ChannelMessageSendComplex(e.ChannelID, msg)
		if err != nil {
			yt.logger().Error(err)
			break
		}

		newPostedVideos = append(newPostedVideos, videoId)

		yt.logger().Info("Posting youtube video: " + feed.Snippet.Title)
	}

	if err == nil {
		e.LastSuccessfulCheckTime = e.NextCheckTime
		c.PostedVideos = alreadyPostedVideos
	}
	c.PostedVideos = append(c.PostedVideos, newPostedVideos...)
	e.Content = c

	return e
}

func (yt *YouTube) setNextCheckTime(e DB_Youtube_Entry) DB_Youtube_Entry {
	e.NextCheckTime = time.Now().
		Add(time.Duration(yt.quota.interval) * time.Second).
		Unix()

	return e
}

func (yt *YouTube) isPosted(id string, postedIds []string) (ok bool) {
	for _, postedId := range postedIds {
		if id == postedId {
			ok = true
			break
		}
	}
	return
}

type youtubeQuota struct {
	entry    DB_Youtube_Entry
	content  DB_Youtube_Content_Quota
	count    int64
	interval int64
	sync.Mutex
}

func (yt *YouTube) initQuota() {
	yt.quota.Lock()
	defer yt.quota.Unlock()
	defer func() {
		if err := yt.createQuota(); err != nil {
			yt.logger().Error(err)
		}
	}()

	// Set content type to quota.
	yt.quota.entry.ContentType = "quota"

	// Set entries count which will use in quota calculation.
	// If failed to get entries count from db, then assume
	// the server has about 200 entries(same with discord server count).
	entries, err := yt.readEntries(map[string]interface{}{})
	if err != nil {
		yt.quota.count = 200
	}
	yt.quota.count = int64(len(entries))

	// fill default quota information
	yt.quota.content.Daily = dailyQuota
	yt.quota.content.Left = dailyQuota
	yt.quota.content.ResetTime = yt.quota.calcResetTime().Unix()

	// read quota entry from db
	entries, err = yt.readEntries(map[string]interface{}{
		"content_type": "quota",
	})
	if err != nil {
		yt.logger().Error(err)
		return
	}

	if len(entries) < 1 {
		return
	}

	for _, e := range entries {
		yt.deleteEntry(e.ID)
	}

	oldQuota, err := entries[0].getQuotaContent()
	if err != nil {
		return
	}

	if yt.quota.content.ResetTime > oldQuota.ResetTime {
		return
	}

	yt.quota.content.Left = oldQuota.Left
}

func (yt *YouTube) updateQuota() error {
	if yt.quota.entry.ID == "" {
		return fmt.Errorf("no quota entry id")
	}

	yt.quota.entry.Content = yt.quota.content
	return yt.updateEntry(yt.quota.entry)
}

func (yt *YouTube) createQuota() error {
	yt.quota.entry.Content = yt.quota.content

	id, err := yt.createEntry(yt.quota.entry)
	if err == nil {
		yt.quota.entry.ID = id
	}

	return err
}

func (yq *youtubeQuota) calcResetTime() time.Time {
	now := time.Now()
	localZone := now.Location()

	// Youtube quota is reset when every midnight in pacific time
	pacific, err := time.LoadLocation("America/Los_Angeles")
	if err == nil {
		now = now.In(pacific)
	}

	y, m, d := now.Date()
	resetTime := time.Date(y, m, d+1, 0, 0, 0, 0, now.Location())

	return resetTime.In(localZone)
}

func (yq *youtubeQuota) sub(i int64) int64 {
	yq.Lock()
	defer yq.Unlock()

	if yq.content.Left < i {
		return -1
	}

	yq.content.Left -= i
	return yq.content.Left
}

func (yq *youtubeQuota) checkingTimeInterval() int64 {
	defaultTimeInterval := int64(5)

	now := time.Now().Unix()

	if now > yq.content.ResetTime {
		yq.content.ResetTime = yq.calcResetTime().Unix()
		yq.content.Left = dailyQuota
	}

	delta := yq.content.ResetTime - now
	if delta < 1 {
		return defaultTimeInterval
	}

	quotaPerSec := yq.content.Left / delta
	if quotaPerSec < 1 {
		return delta
	}

	// default interval(5sec) + (quota cost * checking entry count / available quota per seconds)
	return defaultTimeInterval + (channelsQuota * yq.count / quotaPerSec)
}

func (yq *youtubeQuota) incEntryCount() {
	yq.Lock()
	yq.count++
	yq.Unlock()
}

func (yq *youtubeQuota) decEntryCount() {
	yq.Lock()
	if yq.count > 0 {
		yq.count--
	}
	yq.Unlock()
}

func (yq *youtubeQuota) updateCheckingInterval() {
	yq.Lock()
	yq.interval = yq.checkingTimeInterval()
	yq.Unlock()
}

type DB_Youtube_Entry struct {
	// Common fields.
	ID                      string `gorethink:"id,omitempty"`
	ServerID                string `gorethink:"server_id"`
	ChannelID               string `gorethink:"channel_id"`
	NextCheckTime           int64  `gorethink:"next_check_time"`
	LastSuccessfulCheckTime int64  `gorethink:"last_successful_check_time"`

	// Contents specific data fields.
	// Contents can be channel or video or quota.
	ContentType string      `gorethink:"content_type"`
	Content     interface{} `gorethink:"content"`
}

type DB_Youtube_Content_Channel struct {
	ID           string   `gorethink:"content_channel_id"`
	Name         string   `gorethink:"content_channel_name"`
	PostedVideos []string `gorethink:"content_channel_posted_videos"`
}

type DB_Youtube_Content_Video struct {
	ID                 string `gorethink:"content_video_id"`
	ViewCountsPrevious uint64 `gorethink:"content_video_view_counts_previous"`
	ViewCountsInterval uint64 `gorethink:"content_video_view_counts_interval"`
	ViewCountsFinal    uint64 `gorethink:"content_video_view_counts_final"`
}

type DB_Youtube_Content_Quota struct {
	Daily     int64 `gorethink:"content_quota_daily"`
	Left      int64 `gorethink:"content_quota_left"`
	ResetTime int64 `gorethink:"content_quota_reset_time"`
}

func (ye *DB_Youtube_Entry) getChannelContent() (c DB_Youtube_Content_Channel, err error) {
	if ye.ContentType != "channel" {
		return c, fmt.Errorf("request content channel but the content type is %s", ye.ContentType)
	}

	// get content
	m, ok := ye.Content.(map[string]interface{})
	if ok == false {
		return c, fmt.Errorf("type assertion failed. [field name: Content], [type: %s]", reflect.ValueOf(ye.Content).String())
	}

	// get channel id
	id, ok := m["content_channel_id"].(string)
	if ok == false {
		return c, fmt.Errorf("type assertion failed. [field name: ID], [type: %s]", reflect.ValueOf(m["content_id"]).String())
	}
	c.ID = id

	// get channel name
	name, ok := m["content_channel_name"].(string)
	if ok == false {
		return c, fmt.Errorf("type assertion failed. [field name: Name], [type: %s]", reflect.ValueOf(m["content_name"]).String())
	}
	c.Name = name

	// get posted Videos
	s := reflect.ValueOf(m["content_channel_posted_videos"])
	if s.Kind() != reflect.Slice {
		return c, fmt.Errorf("type assertion failed. [field name: PostedVideo], [type: %s]", reflect.ValueOf(m["content_posted_videos"]).String())
	}

	c.PostedVideos = make([]string, s.Len())
	for i := 0; i < s.Len(); i++ {
		v, ok := s.Index(i).Interface().(string)
		if ok {
			c.PostedVideos = append(c.PostedVideos, v)
		}
	}

	return
}

func (ye *DB_Youtube_Entry) getQuotaContent() (c DB_Youtube_Content_Quota, err error) {
	if ye.ContentType != "quota" {
		return c, fmt.Errorf("request content quota but the content type is %s", ye.ContentType)
	}

	// get content
	m, ok := ye.Content.(map[string]interface{})
	if ok == false {
		return c, fmt.Errorf("type assertion failed. [field name: Content], [type: %s]", reflect.ValueOf(ye.Content).String())
	}

	// get daily quota
	daily, ok := m["content_quota_daily"].(float64)
	if ok == false {
		return c, fmt.Errorf("type assertion failed. [field name: Daily], [type: %s]", reflect.ValueOf(m["content_id"]).String())
	}
	c.Daily = int64(daily)

	// get left quota
	left, ok := m["content_quota_left"].(float64)
	if ok == false {
		return c, fmt.Errorf("type assertion failed. [field name: Left], [type: %s]", reflect.ValueOf(m["content_name"]).String())
	}
	c.Left = int64(left)

	// get reset time
	rt, ok := m["content_quota_reset_time"].(float64)
	if ok == false {
		return c, fmt.Errorf("type assertion failed. [field name: ResetTime], [type: %s]", reflect.ValueOf(m["content_name"]).String())
	}
	c.ResetTime = int64(rt)

	return
}
