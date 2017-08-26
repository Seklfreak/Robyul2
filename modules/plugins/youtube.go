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
	"github.com/bwmarrin/discordgo"
	humanize "github.com/dustin/go-humanize"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"
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
	videoLongUrl   string = `^(https?\:\/\/)?(www\.)?(youtube\.com)\/watch\?v=(.[A-Za-z0-9]*)`
	videoShortUrl  string = `^(https?\:\/\/)?(youtu\.be)\/(.[A-Za-z0-9]*)`
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

	config, err := yt.createConfig()
	helpers.Relax(err)

	client := config.Client(context.Background())

	yt.service, err = youtube.New(client)
	helpers.Relax(err)
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
	case "service":
		// _youtube {args[0]: service} {args[1]: command}
		if len(args) < 2 {
			result = &discordgo.MessageSend{Content: helpers.GetText("bot.arguments.invalid")}
		} else {
			result = yt.system(args[1], msg.Author.ID)
		}
	default:
		// _youtube {args[0:]: search key words...}
		result = yt.search(args[0:])
	}

	_, err := session.ChannelMessageSendComplex(msg.ChannelID, result)
	helpers.Relax(err)
}

func (yt *YouTube) system(command, authorId string) (data *discordgo.MessageSend) {
	if helpers.IsBotAdmin(authorId) == false {
		return &discordgo.MessageSend{Content: helpers.GetText("botadmin.no_permission")}
	}

	switch command {
	case "stop":
		go yt.stop()
	case "start":
		fallthrough
	case "restart":
		go yt.Init(nil)
	default:
		return &discordgo.MessageSend{Content: helpers.GetText("bot.arguments.invalid")}
	}

	return &discordgo.MessageSend{Content: helpers.GetText("plugins.youtube.service-" + command)}
}

func (yt *YouTube) stop() {
	yt.Lock()
	defer yt.Unlock()

	yt.service = nil
}

func (yt *YouTube) search(keywords []string) (data *discordgo.MessageSend) {
	data = &discordgo.MessageSend{}

	if yt.service == nil {
		data.Content = helpers.GetText("plugins.youtube.service-not-available")
		cache.GetLogger().WithField("module", "youtube").Error("youtube service not available")
		return
	}

	// extract ID from valid youtube url
	for i, w := range keywords {
		keywords[i], _ = yt.getIdFromUrl(w)
	}

	// _youtube {args[0:]: search key words}
	if len(keywords) < 1 {
		data.Content = helpers.GetText("bot.argument.invalid")
		return
	}
	query := strings.Join(keywords, " ")

	call := yt.service.Search.List("id,snippet").
		Q(query).
		Type("channel,video").
		MaxResults(1)

	response, err := call.Do()
	if err != nil {
		data.Content = helpers.GetText(err.Error())
		cache.GetLogger().WithField("module", "youtube").Error(err)
		return
	}

	if len(response.Items) <= 0 {
		data.Content = helpers.GetText("plugins.youtube.video-not-found")
		return
	}
	item := response.Items[0]

	switch item.Id.Kind {
	case "youtube#video":
		data = yt.getVideoInfo(item.Id.VideoId)
	case "youtube#channel":
		data = yt.getChannelInfo(item.Id.ChannelId)
	default:
		data.Content = helpers.GetText("plugins.youtube.video-not-found")
		cache.GetLogger().WithField("module", "youtube").Error("unknown item kind")
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
	data = &discordgo.MessageSend{}

	call := yt.service.Videos.List("statistics, snippet").
		Id(videoId).
		MaxResults(1)

	response, err := call.Do()
	if err != nil {
		data.Content = helpers.GetText(err.Error())
		cache.GetLogger().WithField("module", "youtube").Error(err)
		return
	}

	if len(response.Items) <= 0 {
		data.Content = helpers.GetText("plugins.youtube.video-not-found")
		return
	}
	video := response.Items[0]

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

	return
}

func (yt *YouTube) getChannelInfo(channelId string) (data *discordgo.MessageSend) {
	data = &discordgo.MessageSend{}

	call := yt.service.Channels.List("statistics, snippet").
		Id(channelId).
		MaxResults(1)

	response, err := call.Do()
	if err != nil {
		data.Content = helpers.GetText(err.Error())
		cache.GetLogger().WithField("module", "youtube").Error(err)
		return
	}

	if len(response.Items) <= 0 {
		data.Content = helpers.GetText("plugins.youtube.video-not-found")
		return
	}
	channel := response.Items[0]

	data.Embed = &discordgo.MessageEmbed{
		Footer:      &discordgo.MessageEmbedFooter{Text: "YouTube"},
		Title:       channel.Snippet.Title,
		URL:         fmt.Sprintf(YouTubeChannelBaseUrl, channel.Id),
		Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: channel.Snippet.Thumbnails.High.Url},
		Author:      &discordgo.MessageEmbedAuthor{URL: fmt.Sprintf(YouTubeChannelBaseUrl, channel.Id)},
		Description: channel.Snippet.Description,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Views", Value: humanize.Comma(int64(channel.Statistics.ViewCount)), Inline: true},
			{Name: "Subscribers", Value: humanize.Comma(int64(channel.Statistics.SubscriberCount)), Inline: true},
			{Name: "Videos", Value: humanize.Comma(int64(channel.Statistics.VideoCount)), Inline: true},
			{Name: "Comments", Value: humanize.Comma(int64(channel.Statistics.CommentCount)), Inline: true},
			{Name: "Published at", Value: yt.humanizeTime(channel.Snippet.PublishedAt), Inline: true},
		},
		Color: helpers.GetDiscordColorFromHex(YouTubeColor),
	}

	return
}

func (yt *YouTube) humanizeTime(t string) string {
	parsedTime, err := time.Parse(time.RFC3339, t)
	if err != nil {
		cache.GetLogger().WithField("module", "youtube").Error(err)
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

func (yt *YouTube) createConfig() (*jwt.Config, error) {
	config := helpers.GetConfig().Path(youtubeConfigFileName).Data().(string)

	authJSON, err := ioutil.ReadFile(config)
	if err != nil {
		return nil, err
	}

	return google.JWTConfigFromJSON(authJSON, youtube.YoutubeReadonlyScope)
}
