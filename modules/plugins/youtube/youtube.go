package youtube

import (
	"fmt"
	"strings"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	humanize "github.com/dustin/go-humanize"
)

type YouTube struct {
	service   service
	feedsLoop feeds
}

type youtubeAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next youtubeAction)

const (
	youtubeChannelBaseUrl string = "https://www.youtube.com/channel/%s"
	youtubeVideoBaseUrl   string = "https://youtu.be/%s"
	youtubeColor          string = "cd201f"

	youtubeConfigFileName string = "google.client_credentials_json_location"
)

func (yt *YouTube) Commands() []string {
	return []string{
		"youtube",
		"yt",
	}
}

func (yt *YouTube) Init(session *discordgo.Session) {
	defer helpers.Recover()

	yt.service.Init(youtubeConfigFileName)
	yt.feedsLoop.Init(&yt.service)
}

func (yt *YouTube) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
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
	case "quota":
		return yt.actionQuota
	default:
		return yt.actionSearch
	}
}

func (yt *YouTube) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	_, err := cache.GetSession().ChannelMessageSendComplex(in.ChannelID, *out)
	helpers.Relax(err)

	return nil
}

// DEBUG PURPOSE
func (yt *YouTube) actionQuota(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	if helpers.IsBotAdmin(in.Author.ID) == false {
		*out = yt.newMsg("botadmin.no_permission")
		return yt.actionFinish
	}

	q, c, i := yt.service.GetQuotaInfo()
	msg := fmt.Sprintf("next reset: `%s`, left quota: `%s`, channel count: `%s`, time interval: `%s`",
		time.Unix(q.ResetTime, 0).Format(time.ANSIC),
		humanize.Comma(q.Left),
		humanize.Comma(c),
		humanize.Comma(i))

	*out = yt.newMsg(msg)

	return yt.actionFinish
}

// _yt video <search by keywords...>
func (yt *YouTube) actionVideo(args []string, in *discordgo.Message, out **discordgo.MessageSend) youtubeAction {
	if len(args) < 2 {
		*out = yt.newMsg("bot.arguments.too-few")
		return yt.actionFinish
	}

	/* TODO:
	switch args[1] {
	case "add":
		return yt.actionAddVideo
	case "delete":
		return yt.actionDeleteVideo
	case "list":
		return yt.actionListVideo
	}
	*/

	item, err := yt.service.SearchQuerySingle(args[1:], "video")
	if err != nil {
		logger().Error(err)
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

	item, err := yt.service.SearchQuerySingle(args[1:], "channel")
	if err != nil {
		logger().Error(err)
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
	yc, err := yt.service.SearchQuerySingle(args[2:len(args)-1], "channel")
	if err != nil {
		logger().Error(err)
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}

	if yc == nil {
		*out = yt.newMsg("plugins.youtube.channel-not-found")
		return yt.actionFinish
	}

	entry := models.YoutubeChannelEntry{
		ServerID:                dc.GuildID,
		ChannelID:               dc.ID,
		NextCheckTime:           time.Now().Unix(),
		LastSuccessfulCheckTime: time.Now().Unix(),

		YoutubeChannelID:   yc.Id.ChannelId,
		YoutubeChannelName: yc.Snippet.ChannelTitle,
	}

	// Very few channels only have snippet.ChannelID
	// Maybe it's youtube API bug.
	if entry.YoutubeChannelID == "" {
		entry.YoutubeChannelID = yc.Snippet.ChannelId
	}

	if entry.YoutubeChannelID == "" || entry.YoutubeChannelName == "" {
		*out = yt.newMsg("plugins.youtube.channel-not-found")
		return yt.actionFinish
	}

	// insert entry into the db
	_, err = createEntry(entry)
	if err != nil {
		logger().Error(err)
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}

	yt.service.IncQuotaEntryCount()

	*out = yt.newMsg("plugins.youtube.channel-added-success", yc.Snippet.ChannelTitle, dc.ID)
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

	n, err := deleteEntry(args[2])
	if err != nil {
		logger().Error(err)
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}

	if n < 1 {
		*out = yt.newMsg("plugins.youtube.channel-delete-not-found-error")
		return yt.actionFinish
	}

	yt.service.DecQuotaEntryCount()

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

	entries, err := readEntries(map[string]interface{}{
		"server_id": ch.GuildID,
	})
	if err != nil {
		logger().Error(err)
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}

	if len(entries) < 1 {
		*out = yt.newMsg("plugins.youtube.no-entry")
		return yt.actionFinish
	}

	// TODO: pagify
	msg := ""
	for _, e := range entries {
		msg += helpers.GetTextF("plugins.youtube.channel-list-entry", e.ID, e.YoutubeChannelName, e.ChannelID)
	}

	for _, resultPage := range helpers.Pagify(msg, "\n") {
		_, err := cache.GetSession().ChannelMessageSend(in.ChannelID, resultPage)
		helpers.Relax(err)
	}

	*out = yt.newMsg("plugins.youtube.channel-list-sum", len(entries))
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
	item, err := yt.service.SearchQuerySingle(args[0:], "channel, video")
	if err != nil {
		logger().Error(err)
		*out = yt.newMsg(err.Error())
		return yt.actionFinish
	}

	if item == nil {
		*out = yt.newMsg("plugins.youtube.not-found")
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

// getVideoInfo returns information of given video id through *discordgo.MessageSend.
func (yt *YouTube) getVideoInfo(videoId string) (data *discordgo.MessageSend) {
	video, err := yt.service.GetVideoSingle(videoId)
	if err != nil {
		logger().Error(err)
		return yt.newMsg(err.Error())
	}

	if video == nil {
		return yt.newMsg("plugins.youtube.video-not-found")
	}

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
			{Name: "Published at", Value: humanizeTime(video.Snippet.PublishedAt), Inline: true},
		},
		Color: helpers.GetDiscordColorFromHex(youtubeColor),
	}

	data.Embed.Fields = verifyEmbedFields(data.Embed.Fields)

	return
}

// getChannelInfo returns information of given channel id through *discordgo.MessageSend.
func (yt *YouTube) getChannelInfo(channelId string) (data *discordgo.MessageSend) {
	channel, err := yt.service.GetChannelSingle(channelId)
	if err != nil {
		logger().Error(err)
		return yt.newMsg(err.Error())
	}

	if channel == nil {
		return yt.newMsg("plugins.youtube.channel-not-found")
	}

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
			{Name: "Published at", Value: humanizeTime(channel.Snippet.PublishedAt), Inline: true},
		},
		Color: helpers.GetDiscordColorFromHex(youtubeColor),
	}

	data.Embed.Fields = verifyEmbedFields(data.Embed.Fields)

	return
}

func (yt *YouTube) newMsg(content string, replacements ...interface{}) *discordgo.MessageSend {
	if len(replacements) < 1 {
		return &discordgo.MessageSend{Content: helpers.GetText(content)}
	}
	return &discordgo.MessageSend{Content: helpers.GetTextF(content, replacements...)}
}
