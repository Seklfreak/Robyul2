package plugins

import (
	"strings"

	"fmt"
	"time"

	"bytes"

	"regexp"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

type dmAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next dmAction)

type DM struct{}

const (
	DMReceiveChannelIDKey = "dm:receive:channel-id"
)

func (dm *DM) Commands() []string {
	return []string{
		"dm",
		"dms",
	}
}

func (dm *DM) Init(session *discordgo.Session) {

}

func (dm *DM) Uninit(session *discordgo.Session) {

}

func (dm *DM) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	defer helpers.Recover()

	session.ChannelTyping(msg.ChannelID)

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := dm.actionStart
	for action != nil {
		action = action(args, msg, &result)
	}
}

func (dm *DM) actionStart(args []string, in *discordgo.Message, out **discordgo.MessageSend) dmAction {
	cache.GetSession().ChannelTyping(in.ChannelID)

	if len(args) < 1 {
		*out = dm.newMsg("bot.arguments.too-few")
		return dm.actionFinish
	}

	switch args[0] {
	case "send":
		return dm.actionSend
	case "receive":
		return dm.actionReceive
	}

	*out = dm.newMsg("bot.arguments.invalid")
	return dm.actionFinish
}

func (dm *DM) actionSend(args []string, in *discordgo.Message, out **discordgo.MessageSend) dmAction {
	if !helpers.IsRobyulMod(in.Author.ID) {
		*out = dm.newMsg("robyulmod.no_permission")
		return dm.actionFinish
	}

	if !(len(args) >= 3 || (len(args) >= 2 && len(in.Attachments) > 0)) {
		*out = dm.newMsg("bot.arguments.too-few")
		return dm.actionFinish
	}

	targetUser, err := helpers.GetUserFromMention(args[1])
	if err != nil {
		*out = dm.newMsg("bot.arguments.invalid")
		return dm.actionFinish
	}

	dmChannel, err := cache.GetSession().UserChannelCreate(targetUser.ID)
	helpers.Relax(err)

	parts := strings.Split(in.Content, args[1])
	if len(parts) < 2 {
		*out = dm.newMsg("bot.arguments.too-few")
		return dm.actionFinish
	}
	dmMessage := strings.TrimSpace(strings.Join(parts[1:], args[1]))

	dmMessageSend := &discordgo.MessageSend{
		Content: dmMessage,
	}
	var dmAttachmentUrl string
	if len(in.Attachments) > 0 {
		dmAttachmentUrl = in.Attachments[0].URL
		dmFile := helpers.NetGet(dmAttachmentUrl)
		dmMessageSend.File = &discordgo.File{Name: in.Attachments[0].Filename, Reader: bytes.NewReader(dmFile)}
	}

	_, err = helpers.SendComplex(dmChannel.ID, dmMessageSend)
	if err != nil {
		if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeCannotSendMessagesToThisUser {
			*out = dm.newMsg("plugins.dm.send-error-cannot-dm")
			return dm.actionFinish
		}
	}
	helpers.Relax(err)
	dm.logger().WithField("RecipientUserID", args[1]).WithField("AuthorUserID", in.Author.ID).
		Info("send a DM: " + dmMessage + " Attachment: " + dmAttachmentUrl)

	*out = dm.newMsg(helpers.GetTextF("plugins.dm.send-success", targetUser.Username))
	return dm.actionFinish
}

func (dm *DM) actionReceive(args []string, in *discordgo.Message, out **discordgo.MessageSend) dmAction {
	if !helpers.IsRobyulMod(in.Author.ID) {
		*out = dm.newMsg("robyulmod.no_permission")
		return dm.actionFinish
	}

	var err error
	var targetChannel *discordgo.Channel
	if len(args) >= 2 {
		targetChannel, err = helpers.GetChannelFromMention(in, args[1])
		helpers.Relax(err)
	}

	if targetChannel != nil && targetChannel.ID != "" {
		err = helpers.SetBotConfigString(DMReceiveChannelIDKey, targetChannel.ID)
	} else {
		err = helpers.SetBotConfigString(DMReceiveChannelIDKey, "")
	}

	*out = dm.newMsg("plugins.dm.receive-success")
	return dm.actionFinish
}

func (dm *DM) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) dmAction {
	_, err := helpers.SendComplex(in.ChannelID, *out)
	helpers.RelaxMessage(err, in.ChannelID, in.ID)

	return nil
}

func (dm *DM) newMsg(content string) *discordgo.MessageSend {
	return &discordgo.MessageSend{Content: helpers.GetText(content)}
}

func (dm *DM) logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "dm")
}

func (dm *DM) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {
	if msg.Author.Bot == true {
		return
	}

	channel, err := helpers.GetChannel(msg.ChannelID)
	helpers.Relax(err)

	if channel.Type != discordgo.ChannelTypeDM {
		return
	}

	response := dm.DmResponse(msg)
	if response != nil {
		helpers.SendComplex(msg.ChannelID, response)
	}

	dmChannelID, _ := helpers.GetBotConfigString(DMReceiveChannelIDKey)
	if dmChannelID != "" {
		err = dm.repostDM(dmChannelID, msg, response)
		helpers.RelaxLog(err)
	}
}

func (dm *DM) DmResponse(msg *discordgo.Message) (response *discordgo.MessageSend) {
	if msg == nil {
		return
	}

	var content string

	switch {
	case regexp.MustCompile("(?i)^(.)?(HELP|COMMAND).*").MatchString(msg.Content):
		content = helpers.GetText("dm.help")
		break
	case regexp.MustCompile("(?i)^(.)?INVITE.*").MatchString(msg.Content):
		content = helpers.GetText("dm.invite")
		break
	case regexp.MustCompile("(?i)^(.)?ABOUT.*").MatchString(msg.Content):
		content = helpers.GetText("dm.about")
		break
	}

	if content != "" {
		return &discordgo.MessageSend{
			Content: content,
		}
	}

	return nil
}

func (dm *DM) repostDM(channelID string, message *discordgo.Message, response *discordgo.MessageSend) (err error) {
	received, err := message.Timestamp.Parse()
	if err != nil {
		received = time.Now()
	}

	channel, err := helpers.GetChannel(channelID)
	if err != nil {
		return err
	}

	content := message.Content
	for _, attachment := range message.Attachments {
		content += "\n" + attachment.URL
	}
	content = strings.TrimSpace(content)

	if content == "" || content == "." {
		return nil
	}

	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name: fmt.Sprintf("@%s#%s DM'd Robyul:", message.Author.Username, message.Author.Discriminator),
		},
		Description: content,
		Color:       0x0FADED,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("User ID: %s | Received at %s",
				message.Author.ID, received.Format(time.ANSIC)),
		},
		Fields: []*discordgo.MessageEmbedField{},
	}
	if message.Author.Avatar != "" {
		embed.Author.IconURL = message.Author.AvatarURL("128")
	}

	if response != nil {
		responseText := response.Content
		for _, fileResp := range response.Files {
			responseText += "\nAttachment: `" + fileResp.Name + "`"
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Robyul responded:",
			Value:  responseText,
			Inline: false,
		})
	}

	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name: "Reply:",
		Value: fmt.Sprintf("`%sdm send %s <your message>`",
			helpers.GetPrefixForServer(channel.GuildID), message.Author.ID),
		Inline: false,
	})

	_, err = helpers.SendEmbed(channel.ID, embed)
	return err
}

func (dm *DM) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {

}

func (dm *DM) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {

}

func (dm *DM) OnMessageDelete(msg *discordgo.MessageDelete, session *discordgo.Session) {

}

func (dm *DM) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {

}

func (dm *DM) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {

}

func (dm *DM) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {

}

func (dm *DM) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}
