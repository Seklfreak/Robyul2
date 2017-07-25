package plugins

import (
    "golang.org/x/net/context"
    "github.com/bwmarrin/discordgo"
    "io/ioutil"
    "golang.org/x/oauth2/google"
    "github.com/Seklfreak/Robyul2/helpers"
    "google.golang.org/api/youtube/v3"
    "fmt"
    "strings"
    "github.com/dustin/go-humanize"
)

type YouTube struct{}

var (
    youtubeService *youtube.Service
)

const (
    YouTubeChannelBaseUrl string = "https://www.youtube.com/channel/%s"
    YouTubeVideoBaseUrl   string = "https://youtu.be/%s"
    YouTubeColor          string = "cd201f"
)

func (m *YouTube) Commands() []string {
    return []string{
        "youtube",
        "yt",
    }
}

func (m *YouTube) Init(session *discordgo.Session) {
    ctx := context.Background()
    authJson, err := ioutil.ReadFile(helpers.GetConfig().Path("google.client_credentials_json_location").Data().(string))
    helpers.Relax(err)
    config, err := google.JWTConfigFromJSON(authJson, youtube.YoutubeReadonlyScope)
    helpers.Relax(err)
    client := config.Client(ctx)
    youtubeService, err = youtube.New(client)
    helpers.Relax(err)
}

func (m *YouTube) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    session.ChannelTyping(msg.ChannelID)
    videoID := strings.TrimSpace(content)

    response, err := youtubeService.Videos.List("id, snippet, statistics").Id(videoID).MaxResults(1).Do()
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
        Embed: videoEmbed,
    })
    helpers.Relax(err)
}
