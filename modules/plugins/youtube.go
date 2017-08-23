package plugins

import (
	"io/ioutil"
	"net/http"
	"strings"

	"fmt"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
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

	_, err := session.ChannelMessageSendComplex(msg.ChannelID, result)
	helpers.Relax(err)
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
		SafeSearch("strict").
		MaxResults(1)

	response, err := call.Do()
	if err != nil {
		return &discordgo.MessageSend{Content: err.Error()}
	}

	if len(response.Items) <= 0 {
		return &discordgo.MessageSend{Content: "plugins.youtube.video-not-found"}
	}
	item := response.Items[0]

	var id, url string
	switch item.Id.Kind {
	case "youtube#video":
		id = item.Id.VideoId
		url = fmt.Sprintf(YouTubeVideoBaseUrl, id)
	case "youtube#channel":
		id = item.Id.ChannelId
		url = fmt.Sprintf(YouTubeChannelBaseUrl, id)
	default:
		return &discordgo.MessageSend{Content: "unknown item kind: " + item.Kind}
	}

	return &discordgo.MessageSend{Content: url}
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
