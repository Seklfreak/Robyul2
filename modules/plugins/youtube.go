package plugins

import (
	"fmt"
	"io/ioutil"
	"net/http"
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
		// _youtube {args[0:]: search key words}
		result = yt.search(args[0:])
	}

	_, err := session.ChannelMessageSendComplex(msg.ChannelID, result)
	helpers.Relax(err)
}

func (yt *YouTube) search(args []string) (data *discordgo.MessageSend) {
	data = &discordgo.MessageSend{}

	if yt.service == nil {
		data.Content = helpers.GetText("plugins.youtube.service-not-availbale")
		return
	}

	// _youtube {args[0:]: search key words}
	if len(args) < 1 {
		data.Content = helpers.GetText("bot.argument.invalid")
		return
	}
	query := strings.Join(args, " ")

	call := yt.service.Search.List("id,snippet").
		Q(query).
		Type("channel,video").
		MaxResults(1)

	response, err := call.Do()
	if err != nil {
		data.Content = helpers.GetText(err.Error())
		return
	}

	if len(response.Items) <= 0 {
		data.Content = helpers.GetText("plugins.youtube.video-not-found")
		return
	}
	item := response.Items[0]

	switch item.Id.Kind {
	case "youtube#video":
		data.Embed = yt.getVideoInfo(item.Id.VideoId)
	case "youtube#channel":
		data.Embed = yt.getChannelInfo(item.Id.ChannelId)
	default:
		data.Content = helpers.GetText("plugins.youtube.unknown-item-kind")
	}

	return data
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
