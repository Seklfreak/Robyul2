package plugins

import (
	"strings"

	"fmt"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
)

type friendAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next friendAction)

type Friend struct{}

func (f *Friend) Commands() []string {
	return []string{
		"friend",
		"friends",
	}
}

func (f *Friend) Init(session *discordgo.Session) {

}

func (f *Friend) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	defer helpers.Recover()

	session.ChannelTyping(msg.ChannelID)

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := f.actionStart
	for action != nil {
		action = action(args, msg, &result)
	}
}

func (f *Friend) actionStart(args []string, in *discordgo.Message, out **discordgo.MessageSend) friendAction {
	if len(args) < 1 {
		*out = f.newMsg("bot.arguments.too-few")
		return f.actionFinish
	}

	switch args[0] {
	case "list":
		return f.actionList
	case "invite":
		return f.actionInvite
	}

	*out = f.newMsg("bot.arguments.invalid")
	return f.actionFinish
}

func (f *Friend) actionInvite(args []string, in *discordgo.Message, out **discordgo.MessageSend) friendAction {
	if helpers.IsRobyulMod(in.Author.ID) == false {
		*out = f.newMsg(helpers.GetText("robyulmod.no_permission"))
		return f.actionFinish
	}

	channel, err := helpers.GetChannel(in.ChannelID)
	f.Relax(err)

	if cache.GetFriend(channel.GuildID) != nil {
		*out = f.newMsg(helpers.GetText("plugins.friends.invite-error-already-on-server"))
		return f.actionFinish
	}

	friend, err := helpers.InviteFriend()
	if err != nil {
		if strings.Contains(err.Error(), "No friend with free slots available, please add more friends!") {
			f.logger().Error(err.Error())
			*out = f.newMsg(helpers.GetText("plugins.friends.invite-error-no-friend-available"))
			return f.actionFinish
		} else {
			f.Relax(err)
		}
	}

	guild, err := helpers.GetGuild(channel.GuildID)
	f.Relax(err)

	var invite *discordgo.Invite
	for _, guildChannel := range guild.Channels {
		invite, err = cache.GetSession().ChannelInviteCreate(guildChannel.ID, discordgo.Invite{
			MaxAge:    60 * 15,
			MaxUses:   1,
			Temporary: false,
		})
		if err == nil {
			break
		}
	}

	if invite == nil {
		*out = f.newMsg(helpers.GetText("plugins.friends.invite-error-invite-creation-failed"))
		return f.actionFinish
	}

	_, err = helpers.FriendRequest(friend, "POST", "invites/"+invite.Code)
	f.Relax(err)

	*out = f.newMsg(helpers.GetTextF("plugins.friends.invite-success", friend.State.User.Username))
	return f.actionFinish
}

func (f *Friend) actionList(args []string, in *discordgo.Message, out **discordgo.MessageSend) friendAction {
	if helpers.IsRobyulMod(in.Author.ID) == false {
		*out = f.newMsg(helpers.GetText("robyulmod.no_permission"))
		return f.actionFinish
	}

	message := "__**Robyul's friends**__\n"
	friends := cache.GetFriends()
	totalGuilds := 0

	for _, friend := range friends {
		message += fmt.Sprintf(
			"**%s** `(#%s)` (on %d guilds)\n",
			friend.State.User.Username, friend.State.User.ID, len(friend.State.Guilds))
		totalGuilds += len(friend.State.Guilds)
	}

	message += fmt.Sprintf("_in total %d friends on %d guilds_\n", len(friends), totalGuilds)

	*out = f.newMsg(message)
	return f.actionFinish
}

func (f *Friend) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) friendAction {
	_, err := cache.GetSession().ChannelMessageSendComplex(in.ChannelID, *out)
	helpers.Relax(err)

	return nil
}

func (f *Friend) newMsg(content string) *discordgo.MessageSend {
	return &discordgo.MessageSend{Content: helpers.GetText(content)}
}

func (f *Friend) Relax(err error) {
	if err != nil {
		panic(err)
	}
}

func (f *Friend) logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "friends")
}
