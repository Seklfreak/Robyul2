package google

import (
	"strings"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

type Handler struct {
}

type action func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next action)

const (
	GoogleFriendlyUrl = "https://www.google.com/search"
)

func (h *Handler) Commands() []string {
	return []string{
		"google",
		"g",
		"image",
		"img",
	}
}

func (h *Handler) Init(session *discordgo.Session) {
	defer helpers.Recover()
}

func (h *Handler) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	defer helpers.Recover()

	session.ChannelTyping(msg.ChannelID)

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := h.actionSearch

	switch command {
	case "image", "img":
		action = h.actionImageSearch
		break
	}

	for action != nil {
		action = action(args, msg, &result)
	}
}

// [p]google|g <query>
func (h *Handler) actionSearch(args []string, in *discordgo.Message, out **discordgo.MessageSend) action {
	parts := strings.Split(in.Content, " ")
	if len(parts) < 2 {
		*out = h.newMsg("bot.arguments.too-few")
		return h.actionFinish
	}

	query := strings.TrimSpace(strings.Replace(in.Content, parts[0], "", 1))

	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)

	nsfw := channel.NSFW

	results, err := search(query, nsfw)
	if err != nil {
		if strings.Contains(err.Error(), "no search results") {
			*out = h.newMsg("plugins.google.search-no-results")
			return h.actionFinish
		}
	}
	helpers.Relax(err)

	if len(results) <= 0 {
		*out = h.newMsg("plugins.google.search-no-results")
		return h.actionFinish
	}

	embed := linkResultEmbed(results[0])

	*out = &discordgo.MessageSend{
		Content: helpers.GetText("<" + GoogleFriendlyUrl + "?" + getSearchQueries(query, nsfw, true) + ">"),
		Embed:   embed,
	}
	return h.actionFinish
}

// [p]image|img <query>
func (h *Handler) actionImageSearch(args []string, in *discordgo.Message, out **discordgo.MessageSend) action {
	parts := strings.Split(in.Content, " ")
	if len(parts) < 2 {
		*out = h.newMsg("bot.arguments.too-few")
		return h.actionFinish
	}

	query := strings.TrimSpace(strings.Replace(in.Content, parts[0], "", 1))

	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)

	nsfw := channel.NSFW

	results, err := imageSearch(query, nsfw)
	if err != nil {
		if strings.Contains(err.Error(), "no search results") {
			*out = h.newMsg("plugins.google.search-no-results")
			return h.actionFinish
		}
	}
	helpers.Relax(err)

	if len(results) <= 0 {
		*out = h.newMsg("plugins.google.search-no-results")
		return h.actionFinish
	}

	embed := imageResultEmbed(results[0])

	*out = &discordgo.MessageSend{
		Content: helpers.GetText("<" + GoogleFriendlyUrl + "?" + getImageSearchQuries(query, nsfw, true) + ">"),
		Embed:   embed,
	}
	return h.actionFinish
}

func (h *Handler) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) action {
	_, err := helpers.SendComplex(in.ChannelID, *out)
	helpers.Relax(err)

	return nil
}

func (h *Handler) newMsg(content string, replacements ...interface{}) *discordgo.MessageSend {
	if len(replacements) < 1 {
		return &discordgo.MessageSend{Content: helpers.GetText(content)}
	}
	return &discordgo.MessageSend{Content: helpers.GetTextF(content, replacements...)}
}
