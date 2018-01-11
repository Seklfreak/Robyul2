package eventlog

import (
	"strings"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

type Handler struct {
}

type action func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next action)

func (h *Handler) Commands() []string {
	return []string{
		"toggle-eventlog",
		"eventlog",
	}
}

func (h *Handler) Init(session *discordgo.Session) {
	defer helpers.Recover()
}

func (h *Handler) Uninit(session *discordgo.Session) {
	defer helpers.Recover()
}

func (h *Handler) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	defer helpers.Recover()

	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermEventlog) {
		return
	}

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := h.actionStart
	if command == "toggle-eventlog" {
		action = h.actionToggleEventlog
	}
	for action != nil {
		action = action(args, msg, &result)
	}
}

func (h *Handler) actionStart(args []string, in *discordgo.Message, out **discordgo.MessageSend) action {
	cache.GetSession().ChannelTyping(in.ChannelID)

	if len(args) < 1 {
		*out = h.newMsg("bot.arguments.too-few")
		return h.actionFinish
	}

	switch args[0] {
	//case "foo
	//	return h.actionFoo
	}

	*out = h.newMsg("bot.arguments.invalid")
	return nil
}

// [p]toggle-eventlog
func (h *Handler) actionToggleEventlog(args []string, in *discordgo.Message, out **discordgo.MessageSend) action {
	cache.GetSession().ChannelTyping(in.ChannelID)
	if !helpers.IsAdmin(in) {
		*out = h.newMsg("admin.no_permission")
		return h.actionFinish
	}

	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)

	settings := helpers.GuildSettingsGetCached(channel.GuildID)
	var setMessage string
	if settings.EventlogDisabled {
		settings.EventlogDisabled = false
		setMessage = "plugins.eventlog.enabled"
	} else {
		settings.EventlogDisabled = true
		setMessage = "plugins.eventlog.disabled"
	}
	err = helpers.GuildSettingsSet(channel.GuildID, settings)
	helpers.Relax(err)

	*out = h.newMsg(setMessage)
	return h.actionFinish
}

func (h *Handler) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) action {
	_, err := helpers.SendComplex(in.ChannelID, *out)
	helpers.RelaxMessage(err, in.ChannelID, in.ID)

	return nil
}

func (h *Handler) newMsg(content string, replacements ...interface{}) *discordgo.MessageSend {
	if len(replacements) < 1 {
		return &discordgo.MessageSend{Content: helpers.GetText(content)}
	}
	return &discordgo.MessageSend{Content: helpers.GetTextF(content, replacements...)}
}

func (h *Handler) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {

}

func (h *Handler) OnMessageDelete(msg *discordgo.MessageDelete, session *discordgo.Session) {

}

func (h *Handler) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {

}

func (h *Handler) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {

}

func (h *Handler) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {

}

func (h *Handler) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {

}

func (h *Handler) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {

}

func (h *Handler) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}
