package eventlog

import (
	"strings"

	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
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

	session.AddHandler(h.OnChannelCreate)
	session.AddHandler(h.OnChannelDelete)
	session.AddHandler(h.OnGuildRoleCreate)
	session.AddHandler(h.OnGuildRoleDelete)

	go auditlogBackfillLoop()
	logger().Info("started auditlogBackfillLoop loop (1m)")
}

func (h *Handler) Uninit(session *discordgo.Session) {
	defer helpers.Recover()
}

func (h *Handler) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
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

	switch strings.ToLower(args[0]) {
	case "set-log", "set-log-channel":
		return h.actionSetLogChannel
	}

	*out = h.newMsg("bot.arguments.invalid")
	return nil
}

// [p]eventlog set-log [<#channel or channel id>]
func (h *Handler) actionSetLogChannel(args []string, in *discordgo.Message, out **discordgo.MessageSend) action {
	if !helpers.IsMod(in) {
		*out = h.newMsg("mod.no_permission")
		return h.actionFinish
	}

	sourceChannel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)

	targetChannel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)
	if len(args) >= 2 {
		targetChannel, err = helpers.GetChannelFromMention(in, args[1])
		helpers.Relax(err)
	}

	var setMessage string

	newLogChannelIDs := make([]string, 0)
	settings := helpers.GuildSettingsGetCached(sourceChannel.GuildID)
	var removed bool
	for _, currentLogChannelID := range settings.EventlogChannelIDs {
		if currentLogChannelID == targetChannel.ID {
			removed = true
			setMessage = helpers.GetTextF("plugins.eventlog.channel-removed", targetChannel.ID)
			continue
		}
		newLogChannelIDs = append(newLogChannelIDs, currentLogChannelID)
	}

	if !removed {
		newLogChannelIDs = append(newLogChannelIDs, targetChannel.ID)
		setMessage = helpers.GetTextF("plugins.eventlog.channel-added", targetChannel.ID)
	}

	_, err = helpers.EventlogLog(time.Now(), sourceChannel.GuildID, sourceChannel.GuildID,
		models.EventlogTargetTypeGuild, in.Author.ID,
		models.EventlogTypeRobyulEventlogConfigUpdate, "",
		[]models.ElasticEventlogChange{
			{
				Key:      "eventlog_log_channelids",
				OldValue: strings.Join(settings.EventlogChannelIDs, ";"),
				NewValue: strings.Join(newLogChannelIDs, ";"),
				Type:     models.EventlogTargetTypeChannel,
			},
		},
		nil, false)
	helpers.RelaxLog(err)

	settings.EventlogChannelIDs = newLogChannelIDs
	err = helpers.GuildSettingsSet(sourceChannel.GuildID, settings)
	helpers.Relax(err)

	*out = h.newMsg(setMessage)
	return h.actionFinish
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

	var beforeEnabled, afterEnabled bool

	settings := helpers.GuildSettingsGetCached(channel.GuildID)
	var setMessage string
	if settings.EventlogDisabled {
		settings.EventlogDisabled = false
		beforeEnabled = false
		afterEnabled = true
		setMessage = "plugins.eventlog.enabled"
	} else {
		settings.EventlogDisabled = true
		beforeEnabled = true
		afterEnabled = false
		setMessage = "plugins.eventlog.disabled"
	}

	if !helpers.GuildSettingsGetCached(channel.GuildID).EventlogDisabled {
		_, err = helpers.EventlogLog(time.Now(), channel.GuildID, channel.GuildID,
			models.EventlogTargetTypeGuild, in.Author.ID,
			models.EventlogTypeRobyulEventlogConfigUpdate, "",
			[]models.ElasticEventlogChange{
				{
					Key:      "eventlog_enabled",
					OldValue: helpers.StoreBoolAsString(beforeEnabled),
					NewValue: helpers.StoreBoolAsString(afterEnabled),
				},
			},
			nil, false)
		helpers.RelaxLog(err)
	}

	err = helpers.GuildSettingsSet(channel.GuildID, settings)
	helpers.Relax(err)

	if !helpers.GuildSettingsGetCached(channel.GuildID).EventlogDisabled {
		_, err = helpers.EventlogLog(time.Now(), channel.GuildID, channel.GuildID,
			models.EventlogTargetTypeGuild, in.Author.ID,
			models.EventlogTypeRobyulEventlogConfigUpdate, "",
			[]models.ElasticEventlogChange{
				{
					Key:      "eventlog_enabled",
					OldValue: helpers.StoreBoolAsString(beforeEnabled),
					NewValue: helpers.StoreBoolAsString(afterEnabled),
				},
			},
			nil, false)
		helpers.RelaxLog(err)
	}

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
