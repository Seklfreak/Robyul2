package youtube

import (
	"fmt"
	"strings"
	"time"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Seklfreak/Robyul2/modules/plugins/youtube/service"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
)

type Handler struct {
	service   service.Service
	feedsLoop feeds
}

type action func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next action)

const (
	youtubeChannelBaseUrl = "https://www.youtube.com/channel/%s"
	youtubeVideoBaseUrl   = "https://youtu.be/%s"
	youtubeColor          = "FF0000"

	youtubeConfigFileName = "google.client_credentials_json_location"
)

func (h *Handler) Commands() []string {
	return []string{
		"youtube",
		"yt",
	}
}

func (h *Handler) Init(session *discordgo.Session) {
	defer helpers.Recover()

	h.service.Init(youtubeConfigFileName)
	h.feedsLoop.Init(&h.service)
}

func (h *Handler) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	defer helpers.Recover()

	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermYouTube) {
		return
	}

	session.ChannelTyping(msg.ChannelID)

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := h.actionStart
	for action != nil {
		action = action(args, msg, &result)
	}
}

func (h *Handler) actionStart(args []string, in *discordgo.Message, out **discordgo.MessageSend) action {
	if len(args) < 1 {
		*out = h.newMsg("bot.arguments.too-few")
		return h.actionFinish
	}

	switch args[0] {
	case "video":
		return h.actionVideo
	case "channel":
		return h.actionChannel
	case "service":
		return h.actionSystem
	default:
		return h.actionSearch
	}
}

func (h *Handler) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) action {
	_, err := helpers.SendComplex(in.ChannelID, *out)
	helpers.Relax(err)

	return nil
}

// _yt video <search by keywords...>
func (h *Handler) actionVideo(args []string, in *discordgo.Message, out **discordgo.MessageSend) action {
	if len(args) < 2 {
		*out = h.newMsg("bot.arguments.too-few")
		return h.actionFinish
	}

	/* TODO:
	switch args[1] {
	case "add":
		return h.actionAddVideo
	case "delete":
		return h.actionDeleteVideo
	case "list":
		return h.actionListVideo
	}
	*/

	item, err := h.service.SearchQuerySingle(args[1:], "video")
	if err != nil {
		logger().Error(err)
		*out = h.newMsg(err.Error())
		return h.actionFinish
	}

	if item == nil {
		*out = h.newMsg("plugins.youtube.video-not-found")
		return h.actionFinish
	}

	*out = h.getVideoInfo(item.Id.VideoId)
	return h.actionFinish
}

// _yt video add <video id/link/search keywords> <discord channel>
func (h *Handler) actionAddVideo(args []string, in *discordgo.Message, out **discordgo.MessageSend) action {
	*out = h.newMsg("bot.arguments.invalid")
	return h.actionFinish
}

// _yt video delete <video id> <discord channel>
func (h *Handler) actionDeleteVideo(args []string, in *discordgo.Message, out **discordgo.MessageSend) action {
	*out = h.newMsg("bot.arguments.invalid")
	return h.actionFinish
}

// _yt video list
func (h *Handler) actionListVideo(args []string, in *discordgo.Message, out **discordgo.MessageSend) action {
	*out = h.newMsg("bot.arguments.invalid")
	return h.actionFinish
}

// _yt channel <search by keywords...>
func (h *Handler) actionChannel(args []string, in *discordgo.Message, out **discordgo.MessageSend) action {
	if len(args) < 2 {
		*out = h.newMsg("bot.arguments.too-few")
		return h.actionFinish
	}

	switch args[1] {
	case "add":
		return h.actionAddChannel
	case "delete", "remove":
		return h.actionDeleteChannel
	case "list":
		return h.actionListChannel
	}

	item, err := h.service.SearchQuerySingle(args[1:], "channel")
	if err != nil {
		logger().Error(err)
		*out = h.newMsg(err.Error())
		return h.actionFinish
	}

	if item == nil {
		*out = h.newMsg("plugins.youtube.channel-not-found")
		return h.actionFinish
	}

	// Very few channels only have snippet.ChannelID
	// Maybe it's youtube API bug.
	channelId := item.Id.ChannelId
	if channelId == "" {
		channelId = item.Snippet.ChannelId
	}
	*out = h.getChannelInfo(channelId)

	return h.actionFinish
}

// _yt channel add <channel id/link/search keywords> <discord channel>
func (h *Handler) actionAddChannel(args []string, in *discordgo.Message, out **discordgo.MessageSend) action {
	if len(args) < 4 {
		*out = h.newMsg("bot.arguments.too-few")
		return h.actionFinish
	}

	// check permission
	if helpers.IsMod(in) == false {
		*out = h.newMsg("mod.no_permission")
		return h.actionFinish
	}

	// check discord channel
	dc, err := helpers.GetChannelFromMention(in, args[len(args)-1])
	if err != nil {
		*out = h.newMsg("bot.arguments.invalid")
		return h.actionFinish
	}

	// search channel
	yc, err := h.service.SearchQuerySingle(args[2:len(args)-1], "channel")
	if err != nil {
		logger().Error(err)
		*out = h.newMsg(err.Error())
		return h.actionFinish
	}

	if yc == nil {
		*out = h.newMsg("plugins.youtube.channel-not-found")
		return h.actionFinish
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
		*out = h.newMsg("plugins.youtube.channel-not-found")
		return h.actionFinish
	}

	// insert entry into the db
	entryID, err := createEntry(entry)
	if err != nil {
		logger().Error(err)
		*out = h.newMsg(err.Error())
		return h.actionFinish
	}

	h.service.IncQuotaEntryCount()

	_, err = helpers.EventlogLog(time.Now(), dc.GuildID, entryID,
		models.EventlogTargetTypeRobyulYouTubeChannelFeed, in.Author.ID,
		models.EventlogTypeRobyulYouTubeChannelFeedAdd, "",
		nil,
		[]models.ElasticEventlogOption{
			{
				Key:   "youtube_channel_channelid",
				Value: dc.ID,
			},
			{
				Key:   "youtube_channel_ytchannelid",
				Value: yc.Id.ChannelId,
			},
			{
				Key:   "youtube_channel_ytchannelname",
				Value: yc.Snippet.ChannelTitle,
			},
		}, false)
	helpers.RelaxLog(err)

	*out = h.newMsg("plugins.youtube.channel-added-success", yc.Snippet.ChannelTitle, dc.ID)
	return h.actionFinish
}

// _yt channel delete <channel id> <discord channel>
func (h *Handler) actionDeleteChannel(args []string, in *discordgo.Message, out **discordgo.MessageSend) action {
	if len(args) < 3 {
		*out = h.newMsg("bot.arguments.too-few")
		return h.actionFinish
	}

	if helpers.IsMod(in) == false {
		*out = h.newMsg("mod.no_permission")
		return h.actionFinish
	}

	channel, err := helpers.GetChannel(in.ChannelID)
	if err != nil {
		logger().Error(err)
		*out = h.newMsg(err.Error())
		return h.actionFinish
	}

	n, err := deleteEntry(args[2])
	if err != nil {
		logger().Error(err)
		*out = h.newMsg(err.Error())
		return h.actionFinish
	}

	if n < 1 {
		*out = h.newMsg("plugins.youtube.channel-delete-not-found-error")
		return h.actionFinish
	}

	h.service.DecQuotaEntryCount()

	_, err = helpers.EventlogLog(time.Now(), channel.GuildID, args[2],
		models.EventlogTargetTypeRobyulYouTubeChannelFeed, in.Author.ID,
		models.EventlogTypeRobyulYouTubeChannelFeedRemove, "",
		nil,
		nil, false)
	helpers.RelaxLog(err)

	*out = h.newMsg("Delete channel, ID: " + args[2])
	return h.actionFinish
}

// _yt channel list
func (h *Handler) actionListChannel(args []string, in *discordgo.Message, out **discordgo.MessageSend) action {
	if helpers.IsMod(in) == false {
		*out = h.newMsg("mod.no_permission")
		return h.actionFinish
	}

	ch, err := helpers.GetChannel(in.ChannelID)
	if err != nil {
		*out = h.newMsg(err.Error())
		return h.actionFinish
	}

	entries, err := readEntries(map[string]interface{}{
		"server_id": ch.GuildID,
	})
	if err != nil {
		logger().Error(err)
		*out = h.newMsg(err.Error())
		return h.actionFinish
	}

	if len(entries) < 1 {
		*out = h.newMsg("plugins.youtube.no-entry")
		return h.actionFinish
	}

	// TODO: pagify
	msg := ""
	for _, e := range entries {
		msg += helpers.GetTextF("plugins.youtube.channel-list-entry", e.ID, e.YoutubeChannelName, e.ChannelID)
	}

	for _, resultPage := range helpers.Pagify(msg, "\n") {
		_, err := helpers.SendMessage(in.ChannelID, resultPage)
		helpers.Relax(err)
	}

	*out = h.newMsg("plugins.youtube.channel-list-sum", len(entries))
	return h.actionFinish
}

// _yt system restart
func (h *Handler) actionSystem(args []string, in *discordgo.Message, out **discordgo.MessageSend) action {
	if len(args) < 2 {
		*out = h.newMsg("bot.arguments.too-few")
		return h.actionFinish
	}

	if args[1] != "restart" {
		*out = h.newMsg("bot.arguments.invalid")
		return h.actionFinish
	}

	if helpers.IsBotAdmin(in.Author.ID) == false {
		*out = h.newMsg("botadmin.no_permission")
		return h.actionFinish
	}

	go h.Init(nil)

	*out = h.newMsg("plugins.youtube.service-restart")
	return h.actionFinish
}

// _yt <video or channel search by keywords...>
func (h *Handler) actionSearch(args []string, in *discordgo.Message, out **discordgo.MessageSend) action {
	item, err := h.service.SearchQuerySingle(args[0:], "channel, video")
	if err != nil {
		logger().Error(err)
		*out = h.newMsg(err.Error())
		return h.actionFinish
	}

	if item == nil {
		*out = h.newMsg("plugins.youtube.not-found")
		return h.actionFinish
	}

	switch item.Id.Kind {
	case "youtube#video":
		*out = h.getVideoInfo(item.Id.VideoId)
	case "youtube#channel":
		// Very few channels only have snippet.ChannelID
		// Maybe it's youtube API bug.
		channelId := item.Id.ChannelId
		if channelId == "" {
			channelId = item.Snippet.ChannelId
		}
		*out = h.getChannelInfo(channelId)
	default:
		*out = h.newMsg("plugins.youtube.video-not-found")
	}

	return h.actionFinish
}

// getVideoInfo returns information of given video id through *discordgo.MessageSend.
func (h *Handler) getVideoInfo(videoId string) (data *discordgo.MessageSend) {
	video, err := h.service.GetVideoSingle(videoId)
	if err != nil {
		logger().Error(err)
		return h.newMsg(err.Error())
	}

	if video == nil {
		return h.newMsg("plugins.youtube.video-not-found")
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
func (h *Handler) getChannelInfo(channelId string) (data *discordgo.MessageSend) {
	channel, err := h.service.GetChannelSingle(channelId)
	if err != nil {
		logger().Error(err)
		return h.newMsg(err.Error())
	}

	if channel == nil {
		return h.newMsg("plugins.youtube.channel-not-found")
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

func (h *Handler) newMsg(content string, replacements ...interface{}) *discordgo.MessageSend {
	if len(replacements) < 1 {
		return &discordgo.MessageSend{Content: helpers.GetText(content)}
	}
	return &discordgo.MessageSend{Content: helpers.GetTextF(content, replacements...)}
}
