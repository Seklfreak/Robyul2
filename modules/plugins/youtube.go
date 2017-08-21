package plugins

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
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
	session.ChannelTyping(msg.ChannelID)
	videoID := strings.TrimSpace(content)

	response, err := yt.service.Videos.List("id, snippet, statistics").Id(videoID).MaxResults(1).Do()
	helpers.Relax(err)

	if len(response.Items) <= 0 {
		_, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.youtube.video-not-found"))
		helpers.Relax(err)
		return
	}
	video := response.Items[0]

	videoEmbed := &discordgo.MessageEmbed{
		Footer: &discordgo.MessageEmbedFooter{Text: "YouTube"},
		Author: &discordgo.MessageEmbedAuthor{
			Name: video.Snippet.ChannelTitle,
			URL:  fmt.Sprintf(YouTubeChannelBaseUrl, video.Snippet.ChannelId)},
		Title: video.Snippet.Title,
		URL:   fmt.Sprintf(YouTubeVideoBaseUrl, video.Id),
		Image: &discordgo.MessageEmbedImage{URL: video.Snippet.Thumbnails.High.Url},
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Views", Value: humanize.Comma(int64(video.Statistics.ViewCount)), Inline: true},
			{Name: "Likes", Value: humanize.Comma(int64(video.Statistics.LikeCount)), Inline: true},
		},
		Color: helpers.GetDiscordColorFromHex(YouTubeColor),
	}

	_, err = session.ChannelMessageSendComplex(msg.ChannelID, &discordgo.MessageSend{
		Content: fmt.Sprintf("<%s>", fmt.Sprintf(YouTubeVideoBaseUrl, video.Id)),
		Embed:   videoEmbed,
	})
	helpers.Relax(err)
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
