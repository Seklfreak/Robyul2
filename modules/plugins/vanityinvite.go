package plugins

import (
	"strings"

	"fmt"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

type vanityInviteAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next vanityInviteAction)

type VanityInvite struct{}

func (vi VanityInvite) Commands() []string {
	return []string{
		"vanity-invite",
		"custom-invite",
	}
}

func (vi VanityInvite) Init(session *discordgo.Session) {
	// TODO: loop that creates missing invites, confirms old invites are still working
}

func (vi VanityInvite) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	defer helpers.Recover()

	session.ChannelTyping(msg.ChannelID)

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := vi.actionStart
	for action != nil {
		action = action(args, msg, &result)
	}
}

func (vi VanityInvite) actionStart(args []string, in *discordgo.Message, out **discordgo.MessageSend) vanityInviteAction {
	cache.GetSession().ChannelTyping(in.ChannelID)

	if len(args) >= 1 {
		switch args[0] {
		case "set":
			return vi.actionSet
		case "remove", "delete":
			return vi.actionRemove
		case "set-log":
			return vi.actionSetLog
		}
	}

	return vi.actionStatus
}

// [p]custom-invite set <vanity name> <#channel or channel id>
func (vi VanityInvite) actionSet(args []string, in *discordgo.Message, out **discordgo.MessageSend) vanityInviteAction {
	if !helpers.IsAdmin(in) {
		*out = vi.newMsg("admin.no_permission")
		return vi.actionFinish
	}

	if len(args) < 3 {
		*out = vi.newMsg("bot.arguments.too-few")
		return vi.actionFinish
	}

	targetChannel, err := helpers.GetChannelFromMention(in, args[2])
	if err != nil || targetChannel.ID == "" {
		*out = vi.newMsg("bot.arguments.invalid")
		return vi.actionFinish
	}

	vanityEntryByGuildID, _ := helpers.GetVanityUrlByGuildID(targetChannel.GuildID)
	if vanityEntryByGuildID.VanityName != "" && vanityEntryByGuildID.VanityName != strings.ToLower(args[1]) {
		if !helpers.ConfirmEmbed(in.ChannelID, in.Author,
			helpers.GetTextF("plugins.vanityinvite.set-change-confirm", vanityEntryByGuildID.VanityNamePretty),
			"✅", "🚫") {
			return nil
		}
	}

	_, err = cache.GetSession().ChannelInviteCreate(targetChannel.ID, discordgo.Invite{
		MaxAge: 60 * 1, // 1 minute
	})
	if err != nil {
		if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingPermissions {
			*out = vi.newMsg("plugins.vanityinvite.set-error-noinviteperm")
			return vi.actionFinish
		}
	}
	helpers.Relax(err)

	err = helpers.UpdateOrInsertVanityUrl(args[1], targetChannel.GuildID, targetChannel.ID, in.Author.ID)
	if err != nil {
		if strings.Contains(err.Error(), "invalid vanity name") {
			*out = vi.newMsg("plugins.vanityinvite.set-error-invalidname")
			return vi.actionFinish
		}
		if strings.Contains(err.Error(), "vanity name already in use") {
			*out = vi.newMsg("plugins.vanityinvite.set-error-duplicate")
			return vi.actionFinish
		}
	}
	helpers.Relax(err)

	*out = vi.newMsg(helpers.GetTextF("plugins.vanityinvite.set-success",
		args[1], targetChannel.ID,
		helpers.GetConfig().Path("website.vanityurl_domain").Data().(string),
		args[1],
		fmt.Sprintf(helpers.GetConfig().Path("website.vanityurl_stats_base_url").Data().(string), targetChannel.GuildID),
	))
	return vi.actionFinish
}

// [p]custom-invite
func (vi VanityInvite) actionStatus(args []string, in *discordgo.Message, out **discordgo.MessageSend) vanityInviteAction {
	if !helpers.IsMod(in) {
		*out = vi.newMsg("mod.no_permission")
		return vi.actionFinish
	}

	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)

	vanityInvite, _ := helpers.GetVanityUrlByGuildID(channel.GuildID)
	if vanityInvite.VanityName == "" {
		*out = vi.newMsg(helpers.GetTextF("plugins.vanityinvite.status-none"))
		return vi.actionFinish
	}

	*out = vi.newMsg(helpers.GetTextF("plugins.vanityinvite.status",
		vanityInvite.VanityNamePretty, vanityInvite.ChannelID,
		helpers.GetConfig().Path("website.vanityurl_domain").Data().(string),
		vanityInvite.VanityNamePretty,
		fmt.Sprintf(helpers.GetConfig().Path("website.vanityurl_stats_base_url").Data().(string), channel.GuildID),
	))
	return vi.actionFinish
}

// [p]custom-invite remove
func (vi VanityInvite) actionRemove(args []string, in *discordgo.Message, out **discordgo.MessageSend) vanityInviteAction {
	if !helpers.IsAdmin(in) {
		*out = vi.newMsg("admin.no_permission")
		return vi.actionFinish
	}

	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)

	vanityInvite, _ := helpers.GetVanityUrlByGuildID(channel.GuildID)
	if vanityInvite.VanityName == "" {
		*out = vi.newMsg(helpers.GetTextF("plugins.vanityinvite.remove-none"))
		return vi.actionFinish
	}

	if !helpers.ConfirmEmbed(in.ChannelID, in.Author,
		helpers.GetTextF("plugins.vanityinvite.remove-confirm", vanityInvite.VanityNamePretty),
		"✅", "🚫") {
		return nil
	}

	err = helpers.RemoveVanityUrl(vanityInvite)
	helpers.Relax(err)

	*out = vi.newMsg(helpers.GetTextF("plugins.vanityinvite.remove-success"))
	return vi.actionFinish
}

// [p]custom-invite set-log <#channel or channel id>
func (vi VanityInvite) actionSetLog(args []string, in *discordgo.Message, out **discordgo.MessageSend) vanityInviteAction {
	if !helpers.IsRobyulMod(in.Author.ID) {
		*out = vi.newMsg("robyulmod.no_permission")
		return vi.actionFinish
	}

	var err error
	var targetChannel *discordgo.Channel
	if len(args) >= 2 {
		targetChannel, err = helpers.GetChannelFromMention(in, args[1])
		helpers.Relax(err)
	}

	if targetChannel != nil && targetChannel.ID != "" {
		err = helpers.SetBotConfigString(models.VanityInviteLogChannelKey, targetChannel.ID)
	} else {
		err = helpers.SetBotConfigString(models.VanityInviteLogChannelKey, "")
	}

	*out = vi.newMsg("plugins.vanityinvite.setlog-success")
	return vi.actionFinish
}

func (vi VanityInvite) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) vanityInviteAction {
	_, err := helpers.SendComplex(in.ChannelID, *out)
	helpers.Relax(err)

	return nil
}

func (vi VanityInvite) newMsg(content string) *discordgo.MessageSend {
	return &discordgo.MessageSend{Content: helpers.GetText(content)}
}

func (vi VanityInvite) logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "vanityinvite")
}
