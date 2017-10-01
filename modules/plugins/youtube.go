package plugins

import (
	"errors"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
	"sync"
	"time"

	"reflect"

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

	// make sure initalize routine only works when no left yt.Action() jobs
	sync.RWMutex
}

type DB_Youtube_Entry struct {
	// Common fields.
	ID                      string `gorethink:"id,omitempty"`
	ServerID                string `gorethink:"server_id"`
	ChannelID               string `gorethink:"channel_id"`
	NextCheckTime           int64  `gorethink:"next_check_time"`
	LastSuccessfulCheckTime int64  `gorethink:"last_successful_check_time"`
	CheckTimeInterval       int64  `gorethink:"check_time_interval"`

	// Content specific data fields.
	// Content can be about channel or video.
	ContentType string      `gorethink:"content_type"`
	Content     interface{} `gorethink:"content"`
}

type DB_Youtube_Content_Channel struct {
	ID          string   `gorethink:"content_id"`
	PostedVideo []string `gorethink:"content_posted_videos"`
}

type DB_Youtube_Content_Video struct {
	ID                 string `gorethink:"content_id"`
	ViewCountsPrevious uint64 `gorethink:"content_view_counts_previous"`
	ViewCountsInterval uint64 `gorethink:"content_view_counts_interval"`
	ViewCountsFinal    uint64 `gorethink:"content_view_counts_final"`
}

type youtubeAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next youtubeAction)

const (
	YouTubeChannelBaseUrl string = "https://www.youtube.com/channel/%s"
	YouTubeVideoBaseUrl   string = "https://youtu.be/%s"
	YouTubeColor          string = "cd201f"

	youtubeConfigFileName string = "google.client_credentials_json_location"
	youtubeDbTableName    string = "youtube"

	// for yt.regexpSet
	videoLongUrl   string = `^(https?\:\/\/)?(www\.)?(youtube\.com)\/watch\?v=(.[A-Za-z0-9_]*)`
	videoShortUrl  string = `^(https?\:\/\/)?(youtu\.be)\/(.[A-Za-z0-9_]*)`
	channelIdUrl   string = `^(https?\:\/\/)?(www\.)?(youtube\.com)\/channel\/(.[A-Za-z0-9_]*)`
	channelUserUrl string = `^(https?\:\/\/)?(www\.)?(youtube\.com)\/user\/(.[A-Za-z0-9_]*)`
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

	// _yt video <search by keywords...>

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

func (yt *YouTube) actionAddVideo(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	if len(args) < 4 {
		*out = yt.newMsg("bot.arguments.too-few")
		return yt.actionFinish
	}

	// --- REMOVE THIS BLOCK WHEN THE IMPLEMENTATION IS COMPLETE --- //
	*out = yt.newMsg("bot.arguments.invalid")
	return yt.actionFinish
	// --- REMOVE THIS BLOCK WHEN THE IMPLEMENTATION IS COMPLETE --- //

	// _yt video add <video id/link> <discord channel>

	testContent := DB_Youtube_Content_Video{
		ID:                 "test",
		ViewCountsPrevious: 0,
		ViewCountsInterval: 0,
		ViewCountsFinal:    0,
	}

	testEntry := DB_Youtube_Entry{
		ServerID:                "test",
		ChannelID:               "test",
		NextCheckTime:           201709021604,
		LastSuccessfulCheckTime: 201709021604,

		ContentType: "video",
		Content:     testContent,
	}

	id, err := yt.createEntry(testEntry)
	if err != nil {
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}

	*out = yt.newMsg("Add test video, ID: " + id)
	return yt.actionFinish
}

func (yt *YouTube) actionDeleteVideo(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	if len(args) < 3 {
		*out = yt.newMsg("bot.arguments.too-few")
		return yt.actionFinish
	}

	// --- REMOVE THIS BLOCK WHEN THE IMPLEMENTATION IS COMPLETE --- //
	*out = yt.newMsg("bot.arguments.invalid")
	return yt.actionFinish
	// --- REMOVE THIS BLOCK WHEN THE IMPLEMENTATION IS COMPLETE --- //

	// _yt video delete <video id/link> <discord channel>

	// check permission
	if helpers.IsMod(in) == false {
		*out = yt.newMsg("mod.no_permission")
		return yt.actionFinish
	}

	err := yt.deleteEntry(args[2])
	if err != nil {
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}

	*out = yt.newMsg("Delete video, ID: " + args[2])
	return yt.actionFinish
}

func (yt *YouTube) actionListVideo(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {

	// --- REMOVE THIS BLOCK WHEN THE IMPLEMENTATION IS COMPLETE --- //
	*out = yt.newMsg("bot.arguments.invalid")
	return yt.actionFinish
	// --- REMOVE THIS BLOCK WHEN THE IMPLEMENTATION IS COMPLETE --- //

	// _yt video list

	// check permission
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
		"content_type": "video",
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
		msg += e.ID + " "
	}

	*out = yt.newMsg(msg)
	return yt.actionFinish
}

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

	// _yt channel <search by keywords...>

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

	*out = yt.getChannelInfo(item.Id.ChannelId)
	return yt.actionFinish
}

func (yt *YouTube) actionAddChannel(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	if len(args) < 4 {
		*out = yt.newMsg("bot.arguments.too-few")
		return yt.actionFinish
	}

	// --- REMOVE THIS BLOCK WHEN THE IMPLEMENTATION IS COMPLETE --- //
	*out = yt.newMsg("bot.arguments.invalid")
	return yt.actionFinish
	// --- REMOVE THIS BLOCK WHEN THE IMPLEMENTATION IS COMPLETE --- //

	// _yt channel add <video id/link> <discord channel>

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
		ID: yc.Id.ChannelId,
	}

	// fill db entry
	entry := DB_Youtube_Entry{
		ServerID:                dc.GuildID,
		ChannelID:               dc.ID,
		NextCheckTime:           time.Now().Unix(),
		LastSuccessfulCheckTime: time.Now().Unix(),
		CheckTimeInterval:       1,

		ContentType: "channel",
		Content:     content,
	}

	// insert entry into the db
	_, err = yt.createEntry(entry)
	if err != nil {
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}

	*out = yt.newMsg("Added youtube channel <" + yc.Snippet.ChannelTitle + "> to the discord channel " + dc.ID)
	return yt.actionFinish
}

func (yt *YouTube) actionDeleteChannel(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	if len(args) < 3 {
		*out = yt.newMsg("bot.arguments.too-few")
		return yt.actionFinish
	}

	// --- REMOVE THIS BLOCK WHEN THE IMPLEMENTATION IS COMPLETE --- //
	*out = yt.newMsg("bot.arguments.invalid")
	return yt.actionFinish
	// --- REMOVE THIS BLOCK WHEN THE IMPLEMENTATION IS COMPLETE --- //

	// _yt channel delete <video id/link> <discord channel>

	// check permission
	if helpers.IsMod(in) == false {
		*out = yt.newMsg("mod.no_permission")
		return yt.actionFinish
	}

	err := yt.deleteEntry(args[2])
	if err != nil {
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}

	*out = yt.newMsg("Delete channel, ID: " + args[2])
	return yt.actionFinish
}

func (yt *YouTube) actionListChannel(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {

	// --- REMOVE THIS BLOCK WHEN THE IMPLEMENTATION IS COMPLETE --- //
	*out = yt.newMsg("bot.arguments.invalid")
	return yt.actionFinish
	// --- REMOVE THIS BLOCK WHEN THE IMPLEMENTATION IS COMPLETE --- //

	// _yt channel list

	// check permission
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
		msg += e.ID + " "
	}

	*out = yt.newMsg(msg)
	return yt.actionFinish
}

func (yt *YouTube) actionSystem(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	if len(args) < 2 {
		*out = yt.newMsg("bot.arguments.too-few")
		return yt.actionFinish
	}

	// _yt system restart

	if args[1] != "restart" {
		*out = yt.newMsg("bot.arguments.invalid")
		return yt.actionFinish
	}

	if err := yt.restart(in.Author.ID); err != nil {
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}
	*out = yt.newMsg("plugins.youtube.service-restart")

	return yt.actionFinish
}

func (yt *YouTube) actionSearch(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {

	// _yt <video or channel search by keywords...>

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
		*out = yt.getChannelInfo(item.Id.ChannelId)
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

	if len(items) <= 0 {
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
	call := yt.service.Videos.List("statistics, snippet").
		Id(videoId).
		MaxResults(1)

	response, err := call.Do()
	if err != nil {
		yt.logger().Error(err)
		return yt.newMsg(err.Error())
	}

	if len(response.Items) <= 0 {
		return yt.newMsg("plugins.youtube.video-not-found")
	}
	video := response.Items[0]

	data = &discordgo.MessageSend{}

	data.Embed = &discordgo.MessageEmbed{
		Footer: &discordgo.MessageEmbedFooter{Text: "YouTube"},
		Author: &discordgo.MessageEmbedAuthor{
			Name: video.Snippet.ChannelTitle,
			URL:  fmt.Sprintf(YouTubeChannelBaseUrl, video.Snippet.ChannelId),
		},
		Title: video.Snippet.Title,
		URL:   fmt.Sprintf(YouTubeVideoBaseUrl, video.Id),
		Image: &discordgo.MessageEmbedImage{URL: video.Snippet.Thumbnails.High.Url},
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Views", Value: humanize.Comma(int64(video.Statistics.ViewCount)), Inline: true},
			{Name: "Likes", Value: humanize.Comma(int64(video.Statistics.LikeCount)), Inline: true},
			{Name: "Comments", Value: humanize.Comma(int64(video.Statistics.CommentCount)), Inline: true},
			{Name: "Published at", Value: yt.humanizeTime(video.Snippet.PublishedAt), Inline: true},
		},
		Color: helpers.GetDiscordColorFromHex(YouTubeColor),
	}

	data.Embed.Fields = yt.verifyEmbedFields(data.Embed.Fields)

	return
}

// getChannelInfo returns information of given channel id through *discordgo.MessageSend.
func (yt *YouTube) getChannelInfo(channelId string) (data *discordgo.MessageSend) {
	call := yt.service.Channels.List("statistics, snippet").
		Id(channelId).
		MaxResults(1)

	response, err := call.Do()
	if err != nil {
		yt.logger().Error(err)
		return yt.newMsg(err.Error())
	}

	if len(response.Items) <= 0 {
		return yt.newMsg("plugins.youtube.channel-not-found")
	}
	channel := response.Items[0]

	data = &discordgo.MessageSend{}

	data.Embed = &discordgo.MessageEmbed{
		Footer:      &discordgo.MessageEmbedFooter{Text: "YouTube"},
		Title:       channel.Snippet.Title,
		URL:         fmt.Sprintf(YouTubeChannelBaseUrl, channel.Id),
		Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: channel.Snippet.Thumbnails.High.Url},
		Description: channel.Snippet.Description,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Views", Value: humanize.Comma(int64(channel.Statistics.ViewCount)), Inline: true},
			{Name: "Videos", Value: humanize.Comma(int64(channel.Statistics.VideoCount)), Inline: true},
			{Name: "Subscribers", Value: humanize.Comma(int64(channel.Statistics.SubscriberCount)), Inline: true},
			{Name: "Comments", Value: humanize.Comma(int64(channel.Statistics.CommentCount)), Inline: true},
			{Name: "Published at", Value: yt.humanizeTime(channel.Snippet.PublishedAt), Inline: true},
		},
		Color: helpers.GetDiscordColorFromHex(YouTubeColor),
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

// restart youtube service.
func (yt *YouTube) restart(authorId string) error {
	if helpers.IsBotAdmin(authorId) == false {
		return errors.New("botadmin.no_permission")
	}

	go yt.Init(nil)

	return nil
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

func (yt *YouTube) deleteEntry(id string) (err error) {
	query := rethink.Table(youtubeDbTableName).Filter(rethink.Row.Field("id").Eq(id)).Delete()

	_, err = query.RunWrite(helpers.GetDB())
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

	for ; ; time.Sleep(1 * time.Minute) {
		yt.checkYoutubeFeeds()
	}
}

func (yt *YouTube) checkYoutubeFeeds() {
	yt.RLock()
	defer yt.RUnlock()

	t := time.Now().Unix()
	entries, err := yt.readEntries(rethink.Row.Field("next_check_time").Le(t))
	if err != nil {
		yt.logger().Error(err.Error() + " occurs in checkYoutubeFeeds()")
		return
	}

	for _, e := range entries {
		switch e.ContentType {
		case "channel":
			yt.checkYoutubeChannelFeeds(e)
		default:
			yt.logger().Error("unknown contents type: " + e.ContentType)
			continue
		}
	}
}

func (yt *YouTube) checkYoutubeChannelFeeds(e DB_Youtube_Entry) {
	c, ok := e.Content.(map[string]interface{})
	if ok == false {
		yt.logger().Error("contents type mismatch: " + e.ID)
		return
	}

	// get content id
	id, ok := c["content_id"].(string)
	if ok == false {
		yt.logger().Error("wrong content_id type, ID: " + e.ID)
		return
	}

	// get posted Videos
	s := reflect.ValueOf(c["content_posted_videos"])
	if s.Kind() != reflect.Slice {
		yt.logger().Error("wrong content_posted_videos type: " + s.Kind().String() + ", ID: " + id)
		return
	}

	postedVideos := make([]string, s.Len())
	for i := 0; i < s.Len(); i++ {
		postedVideo, ok := s.Index(i).Interface().(string)
		if ok {
			postedVideos = append(postedVideos, postedVideo)
		}
	}

	// get checking time
	nextCheckTime := time.Unix(e.NextCheckTime, 0)
	lastSuccessfulCheckTime := time.Unix(e.LastSuccessfulCheckTime, 0)

	// retrieves updated video one hour before scheduled time
	checkingTime := lastSuccessfulCheckTime.Add(-64 * time.Minute)

	// youtube 'activities' api call requires ISO8601 time format.
	checkingTimeIso8601 := checkingTime.Format("2006-01-02T15:04:05-0700")

	feeds, err := yt.getChannelFeeds(id, checkingTimeIso8601)
	if err != nil {
		yt.logger().Error("check channel feeds error: " + err.Error())
		return
	}

	newPostedVideos := make([]string, 0)
	alreadyPostedVideos := make([]string, 0)

	isErrored := false
	for i := len(feeds) - 1; i >= 0; i-- {
		feed := feeds[i]

		if feed.Snippet.Type != "upload" {
			continue
		}
		videoId := feed.ContentDetails.Upload.VideoId

		isPosted := false
		for _, v := range postedVideos {
			if videoId == v {
				alreadyPostedVideos = append(alreadyPostedVideos, v)
				isPosted = true
				break
			}
		}

		if isPosted {
			continue
		}

		videoUrl := fmt.Sprintf(YouTubeVideoBaseUrl, videoId)

		msg := yt.newMsg(videoUrl)

		_, err = cache.GetSession().ChannelMessageSendComplex(e.ChannelID, msg)
		if err != nil {
			yt.logger().Error(err)
			isErrored = true
			break
		}

		newPostedVideos = append(newPostedVideos, videoId)

		yt.logger().Info("Posting youtube video: " + feed.Snippet.Title)
	}

	if isErrored {
		postedVideos = append(postedVideos, newPostedVideos...)
		c["content_posted_videos"] = postedVideos
	} else {
		if len(newPostedVideos) > 0 {
			e.CheckTimeInterval = 1
		} else if e.CheckTimeInterval < 64 {
			e.CheckTimeInterval *= 2
		}
		e.LastSuccessfulCheckTime = e.NextCheckTime
		e.NextCheckTime = nextCheckTime.Add(time.Duration(e.CheckTimeInterval) * time.Minute).Unix()
		c["content_posted_videos"] = append(alreadyPostedVideos, newPostedVideos...)
	}
	e.Content = c

	if err := yt.updateEntry(e); err != nil {
		yt.logger().Error(err)
	}
}
