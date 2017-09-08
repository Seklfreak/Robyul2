package plugins

import (
	"errors"
	"fmt"
	"io/ioutil"
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
	service   *youtube.Service
	regexpSet []*regexp.Regexp

	// make sure initalize routine only works when no left yt.Action() jobs
	sync.RWMutex
}

type DB_Youtube_Entry struct {
	// Common fields.
	// Timestamp is updated when operation succeed with this entry.
	ID        string `gorethink:"id,omitempty"`
	ServerID  string `gorethink:"serverid"`
	ChannelID string `gorethink:"channelid"`
	Timestamp uint64 `gorethink:"timestamp"`

	// Content specific data fields.
	// Content can be about channel or video.
	ContentType string      `gorethink:"content_type"`
	Content     interface{} `gorethink:"content"`
}

type DB_Youtube_Content_Channel struct {
	ID        string `gorethink:"content_id"`
	Timestamp string `gorethink:"content_timestamp"`
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

	item, err := yt.searchSingle(args[1:], "video")
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
		ServerID:  "test",
		ChannelID: "test",
		Timestamp: 201709021604,

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

	entries, err := yt.readEntries("content_type", "video")
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

	item, err := yt.searchSingle(args[1:], "channel")
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

	testContent := DB_Youtube_Content_Channel{
		ID:        "test",
		Timestamp: "teststamp",
	}

	testEntry := DB_Youtube_Entry{
		ServerID:  "test",
		ChannelID: "test",
		Timestamp: 201709021604,

		ContentType: "channel",
		Content:     testContent,
	}

	id, err := yt.createEntry(testEntry)
	if err != nil {
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}

	*out = yt.newMsg("Add test channel, ID: " + id)
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

	entries, err := yt.readEntries("content_type", "channel")
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

	item, err := yt.searchSingle(args[0:], "channel, video")
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

// searchSingle retuns single search result with given type @searchType.
// returns (nil, nil) when there is no matching results.
func (yt *YouTube) searchSingle(keywords []string, searchType string) (*youtube.SearchResult, error) {
	if yt.service == nil {
		return nil, errors.New("plugins.youtube.service-not-available")
	}

	call := yt.service.Search.List("id, snippet").
		Type(searchType).
		MaxResults(1)

	items, err := yt.search(keywords, call)
	if err != nil {
		return nil, err
	}

	if len(items) <= 0 {
		return nil, nil
	}

	return items[0], nil
}

// search returns search results with given keywords and searchListCall.
func (yt *YouTube) search(keywords []string, call *youtube.SearchListCall) ([]*youtube.SearchResult, error) {
	// extract ID from valid youtube url
	for i, w := range keywords {
		keywords[i], _ = yt.getIdFromUrl(w)
	}

	query := strings.Join(keywords, " ")

	call = call.Q(query)

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

func (yt *YouTube) readEntries(field, equal string) (entry []DB_Youtube_Entry, err error) {
	query := rethink.Table(youtubeDbTableName).Filter(rethink.Row.Field(field).Eq(equal))

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
