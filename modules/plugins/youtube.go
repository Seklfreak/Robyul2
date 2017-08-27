package plugins

import (
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

const (
	YouTubeChannelBaseUrl string = "https://www.youtube.com/channel/%s"
	YouTubeVideoBaseUrl   string = "https://youtu.be/%s"
	YouTubeColor          string = "cd201f"

	youtubeConfigFileName string = "google.client_credentials_json_location"

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

	args := strings.Fields(content)
	if len(args) < 1 {
		_, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		helpers.Relax(err)
		return
	}

	var result *discordgo.MessageSend
	switch args[0] {
	case "video", "channel":
		// _youtube {args[0]: video/channel} {args[1:]: keywords}
		if len(args) < 2 {
			result = yt.newMsg("bot.arguments.invalid")
			break
		}
		result = yt.search(args[1:], args[0])
	case "service":
		// _youtube {args[0]: service} {args[1]: command}
		if len(args) < 2 {
			result = yt.newMsg("bot.arguments.invalid")
			break
		}
		result = yt.system(args[1], msg.Author.ID)
	default:
		// _youtube {args[0:]: search key words...}
		result = yt.search(args[0:], "video, channel")
	}

	_, err := session.ChannelMessageSendComplex(msg.ChannelID, result)
	helpers.Relax(err)
}

func (yt *YouTube) system(command, authorId string) (data *discordgo.MessageSend) {
	if helpers.IsBotAdmin(authorId) == false {
		return yt.newMsg("botadmin.no_permission")
	}

	if command != "restart" {
		return yt.newMsg("bot.arguments.invalid")
	}

	go yt.Init(nil)

	return yt.newMsg("plugins.youtube.service-restart")
}

func (yt *YouTube) search(keywords []string, searchType string) (data *discordgo.MessageSend) {
	if yt.service == nil {
		yt.logger().Error("youtube service not available")
		return yt.newMsg("plugins.youtube.service-not-available")
	}

	// extract ID from valid youtube url
	for i, w := range keywords {
		keywords[i], _ = yt.getIdFromUrl(w)
	}

	// _youtube {args[0:]: search key words}
	if len(keywords) < 1 {
		return yt.newMsg("bot.arguments.invalid")
	}
	query := strings.Join(keywords, " ")

	call := yt.service.Search.List("id,snippet").
		Q(query).
		Type(searchType).
		MaxResults(1)

	response, err := call.Do()
	if err != nil {
		yt.logger().Error(err)
		return yt.newMsg(err.Error())
	}

	if len(response.Items) <= 0 {
		return yt.newMsg("plugins.youtube.video-not-found")
	}
	item := response.Items[0]

	switch item.Id.Kind {
	case "youtube#video":
		data = yt.getVideoInfo(item.Id.VideoId)
	case "youtube#channel":
		data = yt.getChannelInfo(item.Id.ChannelId)
	default:
		yt.logger().Error("unknown item kind")
		data = yt.newMsg("plugins.youtube.video-not-found")
	}

	return data
}

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
