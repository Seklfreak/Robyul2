package plugins

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	humanize "github.com/dustin/go-humanize"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"
	"google.golang.org/api/youtube/v3"
)

type YouTube struct {
	service        *youtube.Service
	client         *http.Client
	configFileName string
	config         *jwt.Config
}

const (
	YouTubeChannelBaseUrl string = "https://www.youtube.com/channel/%s"
	YouTubeVideoBaseUrl   string = "https://youtu.be/%s"
	YouTubeColor          string = "cd201f"

	youtubeConfigFileName string = "google.client_credentials_json_location"
)

func (yt *YouTube) Commands() []string {
	return []string{
		"youtube",
		"yt",
	}
}

func (yt *YouTube) Init(session *discordgo.Session) {
	yt.configFileName = youtubeConfigFileName

	err := yt.createConfig()
	helpers.Relax(err)

	yt.client = yt.config.Client(context.Background())

	yt.service, err = youtube.New(yt.client)
	helpers.Relax(err)
}

func (yt *YouTube) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
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
	default:
		// _youtube {args[0]: videoID}
		result = yt.search(args[0:])
	}

	if result.Content != "" {
		_, err := session.ChannelMessageSend(msg.ChannelID, result.Content)
		helpers.Relax(err)
	}
	if result.Embed != nil {
		_, err := session.ChannelMessageSendEmbed(msg.ChannelID, result.Embed)
		helpers.Relax(err)
	}
}

func (yt *YouTube) search(args []string) *discordgo.MessageSend {
	if yt.service == nil {
		return &discordgo.MessageSend{Content: "plugins.youtube.service-not-available"}
	}

	// _youtube {args[0]: videoID}
	if len(args) < 1 {
		return &discordgo.MessageSend{Content: "bot.argument.invalid"}
	}
	query := strings.Join(args, " ")

	call := yt.service.Search.List("id,snippet").
		Q(query).
		Type("channel,video").
		MaxResults(1)

	response, err := call.Do()
	if err != nil {
		return &discordgo.MessageSend{Content: err.Error()}
	}

	if len(response.Items) <= 0 {
		return &discordgo.MessageSend{Content: "plugins.youtube.video-not-found"}
	}
	item := response.Items[0]

	var id string
	data := &discordgo.MessageSend{}

	switch item.Id.Kind {
	case "youtube#video":
		id = item.Id.VideoId
		data.Content = fmt.Sprintf(YouTubeVideoBaseUrl, id)
		data.Embed = yt.getVideoInfo(id)
	case "youtube#channel":
		id = item.Id.ChannelId
		data.Content = fmt.Sprintf(YouTubeChannelBaseUrl, id)
		data.Embed = yt.getChannelInfo(id)
	default:
		data.Content = "unknown item kind: " + item.Kind
	}

	for _, arg := range args {
		if yt.checkUrlSame(arg, id) {
			data.Content = ""
		}
	}

	return data
}

func (yt *YouTube) checkUrlSame(arg, id string) bool {
	videoLongUrl := regexp.MustCompile(`^(https?\:\/\/)?(www\.)?(youtube\.com)\/watch\?v=` + id + `+$`)
	videoShortUrl := regexp.MustCompile(`^(https?\:\/\/)?(youtu\.be)\/` + id + `+$`)
	channelUrl := regexp.MustCompile(`^(https?\:\/\/)?(www\.)?(youtube\.com)\/channel\/` + id)

	if videoLongUrl.MatchString(arg) {
		return true
	}
	if videoShortUrl.MatchString(arg) {
		return true
	}
	if channelUrl.MatchString(arg) {
		return true
	}
	return false
}

func (yt *YouTube) getVideoInfo(videoId string) *discordgo.MessageEmbed {
	call := yt.service.Videos.List("statistics, snippet").
		Id(videoId).
		MaxResults(1)

	response, err := call.Do()
	if err != nil {
		return nil
	}

	if len(response.Items) <= 0 {
		return nil
	}
	video := response.Items[0]

	return &discordgo.MessageEmbed{
		Footer: &discordgo.MessageEmbedFooter{Text: "Video information of " + video.Snippet.Title},
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Views", Value: humanize.Comma(int64(video.Statistics.ViewCount)), Inline: true},
			{Name: "Likes", Value: humanize.Comma(int64(video.Statistics.LikeCount)), Inline: true},
			{Name: "Comments", Value: humanize.Comma(int64(video.Statistics.CommentCount)), Inline: true},
			{Name: "Published at", Value: video.Snippet.PublishedAt, Inline: true},
		},
		Color: helpers.GetDiscordColorFromHex(YouTubeColor),
	}
}

func (yt *YouTube) getChannelInfo(channelId string) *discordgo.MessageEmbed {
	call := yt.service.Channels.List("statistics, snippet").
		Id(channelId).
		MaxResults(1)

	response, err := call.Do()
	if err != nil {
		return nil
	}

	if len(response.Items) <= 0 {
		return nil
	}
	channel := response.Items[0]

	return &discordgo.MessageEmbed{
		Footer: &discordgo.MessageEmbedFooter{Text: "Channel information of " + channel.Snippet.Title},
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Views", Value: humanize.Comma(int64(channel.Statistics.ViewCount)), Inline: true},
			{Name: "Subscribers", Value: humanize.Comma(int64(channel.Statistics.SubscriberCount)), Inline: true},
			{Name: "Videos", Value: humanize.Comma(int64(channel.Statistics.VideoCount)), Inline: true},
			{Name: "Comments", Value: humanize.Comma(int64(channel.Statistics.CommentCount)), Inline: true},
			{Name: "Published at", Value: channel.Snippet.PublishedAt, Inline: true},
		},
		Color: helpers.GetDiscordColorFromHex(YouTubeColor),
	}
}

func (yt *YouTube) createConfig() error {
	config := yt.getConfig()

	authJSON, err := ioutil.ReadFile(config)
	if err != nil {
		return err
	}

	yt.config, err = google.JWTConfigFromJSON(authJSON, youtube.YoutubeReadonlyScope)
	return err
}

func (yt *YouTube) getConfig() string {
	return helpers.GetConfig().Path(yt.configFileName).Data().(string)
}
