package plugins

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Seklfreak/Robyul2/shardmanager"
	"github.com/bwmarrin/discordgo"
	"github.com/globalsign/mgo/bson"
)

type GuildAnnouncements struct{}

func (m *GuildAnnouncements) Commands() []string {
	return []string{
		"guildannouncements",
		"announcements",
		"greet",
		"greeter",
	}
}

func (m *GuildAnnouncements) Init(session *shardmanager.Manager) {

}

func (m *GuildAnnouncements) Uninit(session *shardmanager.Manager) {

}

func (m *GuildAnnouncements) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermGuildAnnouncements) {
		return
	}

	args := strings.Fields(content)
	if len(args) < 1 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
		return
	}

	switch args[0] {
	// [p]greeter join <#channel or channel id> <embed code>
	case "guild_join", "join":
		helpers.RequireAdmin(msg, func() {
			if len(args) < 2 {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
				return
			}

			targetChannel, err := helpers.GetChannelFromMention(msg, args[1])
			if err != nil || targetChannel.ID == "" {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				return
			}

			var embedCode string

			if len(args) >= 3 {
				embedCode = strings.TrimSpace(strings.Replace(content, strings.Join(args[:2], " "), "", 1))
			}

			if embedCode == "" {
				var entryBucket models.GreeterEntry
				err = helpers.MdbOne(
					helpers.MdbCollection(models.GreeterTable).Find(bson.M{
						"type": models.GreeterTypeJoin, "guildid": targetChannel.GuildID, "channelid": targetChannel.ID,
					}),
					&entryBucket,
				)
				if err == nil {
					helpers.MDbDelete(models.GreeterTable, entryBucket.Id)
				}

				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.guildannouncements.message-disabled"))
				helpers.Relax(err)
				return
			}

			err = helpers.MDbUpsert(
				models.GreeterTable,
				bson.M{"type": models.GreeterTypeJoin, "guildid": targetChannel.GuildID, "channelid": targetChannel.ID},
				models.GreeterEntry{
					GuildID:   targetChannel.GuildID,
					ChannelID: targetChannel.ID,
					Type:      models.GreeterTypeJoin,
					EmbedCode: embedCode,
				},
			)
			helpers.Relax(err)

			_, err = helpers.EventlogLog(time.Now(), targetChannel.GuildID, targetChannel.ID,
				models.EventlogTargetTypeChannel, msg.Author.ID,
				models.EventlogTypeRobyulGuildAnnouncementsJoinSet, "",
				nil,
				[]models.ElasticEventlogOption{
					{
						Key:   "join_text",
						Value: embedCode,
					},
				}, false)
			helpers.RelaxLog(err)

			_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.guildannouncements.message-edited"))
			helpers.Relax(err)
		})
		// [p]greeter leave <#channel or channel id> <embed code>
	case "guild_leave", "leave":
		helpers.RequireAdmin(msg, func() {
			if len(args) < 2 {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
				return
			}

			targetChannel, err := helpers.GetChannelFromMention(msg, args[1])
			if err != nil || targetChannel.ID == "" {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				return
			}

			var embedCode string

			if len(args) >= 3 {
				embedCode = strings.TrimSpace(strings.Replace(content, strings.Join(args[:2], " "), "", 1))
			}

			if embedCode == "" {
				var entryBucket models.GreeterEntry
				err = helpers.MdbOne(
					helpers.MdbCollection(models.GreeterTable).Find(bson.M{
						"type": models.GreeterTypeLeave, "guildid": targetChannel.GuildID, "channelid": targetChannel.ID,
					}),
					&entryBucket,
				)
				if err == nil {
					helpers.MDbDelete(models.GreeterTable, entryBucket.Id)
				}

				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.guildannouncements.message-disabled"))
				helpers.Relax(err)
				return
			}

			err = helpers.MDbUpsert(
				models.GreeterTable,
				bson.M{"type": models.GreeterTypeLeave, "guildid": targetChannel.GuildID, "channelid": targetChannel.ID},
				models.GreeterEntry{
					GuildID:   targetChannel.GuildID,
					ChannelID: targetChannel.ID,
					Type:      models.GreeterTypeLeave,
					EmbedCode: embedCode,
				},
			)
			helpers.Relax(err)

			_, err = helpers.EventlogLog(time.Now(), targetChannel.GuildID, targetChannel.ID,
				models.EventlogTargetTypeChannel, msg.Author.ID,
				models.EventlogTypeRobyulGuildAnnouncementsLeaveSet, "",
				nil,
				[]models.ElasticEventlogOption{
					{
						Key:   "leave_text",
						Value: embedCode,
					},
				}, false)
			helpers.RelaxLog(err)

			_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.guildannouncements.message-edited"))
			helpers.Relax(err)
		})
	case "ban": // [p]greeter ban <#channel or channel id> <embed code>
		helpers.RequireAdmin(msg, func() {
			if len(args) < 2 {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
				return
			}

			targetChannel, err := helpers.GetChannelFromMention(msg, args[1])
			if err != nil || targetChannel.ID == "" {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				return
			}

			var embedCode string

			if len(args) >= 3 {
				embedCode = strings.TrimSpace(strings.Replace(content, strings.Join(args[:2], " "), "", 1))
			}

			if embedCode == "" {
				var entryBucket models.GreeterEntry
				err = helpers.MdbOne(
					helpers.MdbCollection(models.GreeterTable).Find(bson.M{
						"type": models.GreeterTypeBan, "guildid": targetChannel.GuildID, "channelid": targetChannel.ID,
					}),
					&entryBucket,
				)
				if err == nil {
					helpers.MDbDelete(models.GreeterTable, entryBucket.Id)
				}

				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.guildannouncements.message-disabled"))
				helpers.Relax(err)
				return
			}

			err = helpers.MDbUpsert(
				models.GreeterTable,
				bson.M{"type": models.GreeterTypeBan, "guildid": targetChannel.GuildID, "channelid": targetChannel.ID},
				models.GreeterEntry{
					GuildID:   targetChannel.GuildID,
					ChannelID: targetChannel.ID,
					Type:      models.GreeterTypeBan,
					EmbedCode: embedCode,
				},
			)
			helpers.Relax(err)

			_, err = helpers.EventlogLog(time.Now(), targetChannel.GuildID, targetChannel.ID,
				models.EventlogTargetTypeChannel, msg.Author.ID,
				models.EventlogTypeRobyulGuildAnnouncementsBanSet, "",
				nil,
				[]models.ElasticEventlogOption{
					{
						Key:   "ban_text",
						Value: embedCode,
					},
				}, false)
			helpers.RelaxLog(err)

			_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.guildannouncements.message-edited"))
			helpers.Relax(err)
		})
	case "list":
		helpers.RequireMod(msg, func() {
			session.ChannelTyping(msg.ChannelID)

			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)

			var entryBucket []models.GreeterEntry
			err = helpers.MDbIter(helpers.MdbCollection(models.GreeterTable).Find(bson.M{"guildid": channel.GuildID})).All(&entryBucket)
			helpers.Relax(err)

			if entryBucket == nil || len(entryBucket) <= 0 {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.guildannouncements.list-none")) // TODO
				return
			}

			var message string
			for _, greeting := range entryBucket {
				switch greeting.Type {
				case models.GreeterTypeJoin:
					message += "on join in <#" + greeting.ChannelID + ">: `" + greeting.EmbedCode + "`\n"
					break
				case models.GreeterTypeLeave:
					message += "on leave in <#" + greeting.ChannelID + ">: `" + greeting.EmbedCode + "`\n"
					break
				case models.GreeterTypeBan:
					message += "on ban in <#" + greeting.ChannelID + ">: `" + greeting.EmbedCode + "`\n"
					break
				}
			}
			message += fmt.Sprintf("_found %d greeter configs in total_\n_To change a config just set a new config for the specific channel, it will replace the old config._", len(entryBucket))

			_, err = helpers.SendMessage(msg.ChannelID, message)
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			return
		})
		return
	}

}

func (m *GuildAnnouncements) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {

}

func (m *GuildAnnouncements) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {
	go func() {
		defer helpers.Recover()

		_, err := helpers.GetGuildWithoutApi(member.GuildID)
		if err != nil {
			return
		}

		var entryBucket []models.GreeterEntry
		err = helpers.MDbIterWithoutLogging(helpers.MdbCollection(models.GreeterTable).
			Find(bson.M{"guildid": member.GuildID, "type": models.GreeterTypeJoin})).All(&entryBucket)
		helpers.Relax(err)

		if entryBucket == nil || len(entryBucket) <= 0 {
			return
		}

		for _, guildAnnouncementSetting := range entryBucket {
			ourSetting := guildAnnouncementSetting
			go func() {
				defer helpers.Recover()
				messageSend := &discordgo.MessageSend{
					Content: ourSetting.EmbedCode,
				}
				if helpers.IsEmbedCode(ourSetting.EmbedCode) {
					ptext, embed, err := helpers.ParseEmbedCode(ourSetting.EmbedCode)
					if err == nil {
						messageSend.Content = ptext
						messageSend.Embed = embed
					}
				}
				messageSend = m.ReplaceMemberText(messageSend, member)
				if messageSend == nil {
					return
				}
				helpers.SendComplex(ourSetting.ChannelID, messageSend)
			}()
		}
		// cache.GetLogger().WithField("module", "guildannouncements").Info(fmt.Sprintf("User %s (%s) joined Guild %s (#%s)", member.User.Username, member.User.ID, guild.Name, guild.ID))
	}()
}

func (m *GuildAnnouncements) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {
	go func() {
		defer helpers.Recover()

		_, err := helpers.GetGuildWithoutApi(member.GuildID)
		if err != nil {
			return
		}

		var entryBucket []models.GreeterEntry
		err = helpers.MDbIterWithoutLogging(helpers.MdbCollection(models.GreeterTable).
			Find(bson.M{"guildid": member.GuildID, "type": models.GreeterTypeLeave})).All(&entryBucket)
		helpers.Relax(err)

		if entryBucket == nil || len(entryBucket) <= 0 {
			return
		}

		for _, guildAnnouncementSetting := range entryBucket {
			ourSetting := guildAnnouncementSetting
			go func() {
				defer helpers.Recover()

				messageSend := &discordgo.MessageSend{
					Content: ourSetting.EmbedCode,
				}
				if helpers.IsEmbedCode(ourSetting.EmbedCode) {
					ptext, embed, err := helpers.ParseEmbedCode(ourSetting.EmbedCode)
					if err == nil {
						messageSend.Content = ptext
						messageSend.Embed = embed
					}
				}
				messageSend = m.ReplaceMemberText(messageSend, member)
				if messageSend == nil {
					return
				}
				helpers.SendComplex(ourSetting.ChannelID, messageSend)
			}()
		}
		// cache.GetLogger().WithField("module", "guildannouncements").Infof("User %s (%s) left Guild %s (#%s)", member.User.Username, member.User.ID, guild.Name, guild.ID)
	}()
}

func (m *GuildAnnouncements) ReplaceMemberText(message *discordgo.MessageSend, member *discordgo.Member) *discordgo.MessageSend {
	guild, err := helpers.GetGuild(member.GuildID)
	if errD, ok := err.(*discordgo.RESTError); ok {
		if errD.Message.Code != discordgo.ErrCodeMissingAccess { // It's probably Robyul leaving a server :nayoungpout:
			return nil
		} else {
			helpers.Relax(err)
		}
	} else {
		helpers.Relax(err)
	}

	userNumber := -1
	if guild != nil {
		if guild.Members != nil {
			userNumber = len(guild.Members)
		}
		if guild.MemberCount > userNumber {
			userNumber = guild.MemberCount
		}
	}

	return helpers.ReplaceMessageSend(
		message,
		[]*helpers.ReplaceValues{
			{Before: "{USER_USERNAME}", After: member.User.Username},
			{Before: "{USER_ID}", After: member.User.ID},
			{Before: "{USER_DISCRIMINATOR}", After: member.User.Discriminator},
			{Before: "{USER_NUMBER}", After: strconv.Itoa(userNumber)},
			{Before: "{USER_MENTION}", After: member.User.Mention()},
			{Before: "{USER_AVATARURL}", After: member.User.AvatarURL("")},
			{Before: "{GUILD_NAME}", After: guild.Name},
			{Before: "{GUILD_ID}", After: guild.ID},
		},
	)
}

func (m *GuildAnnouncements) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {
	go func() {
		defer helpers.Recover()

		_, err := helpers.GetGuildWithoutApi(user.GuildID)
		if err != nil {
			return
		}

		var entryBucket []models.GreeterEntry
		err = helpers.MDbIterWithoutLogging(helpers.MdbCollection(models.GreeterTable).
			Find(bson.M{"guildid": user.GuildID, "type": models.GreeterTypeBan})).All(&entryBucket)
		helpers.Relax(err)

		if entryBucket == nil || len(entryBucket) <= 0 {
			return
		}

		for _, guildAnnouncementSetting := range entryBucket {
			ourSetting := guildAnnouncementSetting
			go func() {
				defer helpers.Recover()
				member := new(discordgo.Member)
				member.User = user.User
				member.GuildID = user.GuildID
				messageSend := &discordgo.MessageSend{
					Content: ourSetting.EmbedCode,
				}
				if helpers.IsEmbedCode(ourSetting.EmbedCode) {
					ptext, embed, err := helpers.ParseEmbedCode(ourSetting.EmbedCode)
					if err == nil {
						messageSend.Content = ptext
						messageSend.Embed = embed
					}
				}
				messageSend = m.ReplaceMemberText(messageSend, member)
				if messageSend == nil {
					return
				}
				helpers.SendComplex(ourSetting.ChannelID, messageSend)
			}()
		}
		// cache.GetLogger().WithField("module", "guildannouncements").Info(fmt.Sprintf("User %s (%s) banned on Guild %s (#%s)", user.User.Username, user.User.ID, guild.Name, guild.ID))
	}()
}

func (m *GuildAnnouncements) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {

}
func (m *GuildAnnouncements) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {

}
func (m *GuildAnnouncements) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}
func (m *GuildAnnouncements) OnMessageDelete(msg *discordgo.MessageDelete, session *discordgo.Session) {

}
