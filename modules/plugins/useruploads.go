package plugins

import (
	"strings"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

type useruploadsAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next useruploadsAction)

type Useruploads struct{}

func (m *Useruploads) Commands() []string {
	return []string{
		"useruploads",
	}
}

func (m *Useruploads) Init(session *discordgo.Session) {
}

func (m *Useruploads) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	session.ChannelTyping(msg.ChannelID)

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := m.actionStart
	for action != nil {
		action = action(args, msg, &result)
	}
}

func (m *Useruploads) actionStart(args []string, in *discordgo.Message, out **discordgo.MessageSend) useruploadsAction {
	cache.GetSession().ChannelTyping(in.ChannelID)

	if len(args) < 1 {
		*out = m.newMsg(helpers.GetText("bot.arguments.too-few"))
		return m.actionFinish
	}

	switch args[0] {
	case "disable":
		return m.actionDisable
	}

	*out = m.newMsg(helpers.GetText("bot.arguments.invalid"))
	return m.actionFinish
}

func (m *Useruploads) actionDisable(args []string, in *discordgo.Message, out **discordgo.MessageSend) useruploadsAction {
	if !helpers.IsRobyulMod(in.Author.ID) {
		*out = m.newMsg(helpers.GetText("robyulmod.no_permission"))
		return m.actionFinish
	}

	if len(args) < 2 {
		*out = m.newMsg(helpers.GetText("bot.arguments.too-few"))
		return m.actionFinish
	}

	targetUser, err := helpers.GetUserFromMention(args[1])
	helpers.Relax(err)

	err = helpers.UseruploadsDisableUser(targetUser.ID, in.Author.ID)
	helpers.Relax(err)

	*out = m.newMsg(helpers.GetTextF("plugins.useruploads.disable-success"))
	return m.actionFinish
}

func (m *Useruploads) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) useruploadsAction {
	_, err := helpers.SendComplex(in.ChannelID, *out)
	helpers.Relax(err)

	return nil
}

func (m *Useruploads) newMsg(content string) *discordgo.MessageSend {
	return &discordgo.MessageSend{Content: helpers.GetText(content)}
}

func (m *Useruploads) Relax(err error) {
	if err != nil {
		panic(err)
	}
}

func (m *Useruploads) logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "useruploads")
}
