package plugins

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bradfitz/slice"
	"github.com/bwmarrin/discordgo"
	"github.com/getsentry/raven-go"
	rethink "github.com/gorethink/gorethink"
)

type GuildAnnouncements struct{}

type Announcement_Setting struct {
	Id                  string `rethink:"id,omitempty"`
	GuildID             string `rethink:"guildid"`
	GuildJoinChannelID  string `rethink:"guild_join_channelid"`
	GuildJoinText       string `rethink:"guild_join_text"`
	GuildJoinEnabled    bool   `rethink:"guild_join_enabled"`
	GuildLeaveChannelID string `rethink:"guild_leave_channelid"`
	GuildLeaveText      string `rethink:"guild_leave_text"`
	GuildLeaveEnabled   bool   `rethink:"guild_leave_enabled"`
}

func (m *GuildAnnouncements) Commands() []string {
	return []string{
		"guildannouncements",
	}
}

func (m *GuildAnnouncements) Init(session *discordgo.Session) {

}

func (m *GuildAnnouncements) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	args := strings.Fields(content)
	if len(args) >= 2 {
		switch args[0] {
		case "set":
			switch args[1] {
			case "guild_join":
				helpers.RequireAdmin(msg, func() {
					sourceChannel, err := helpers.GetChannel(msg.ChannelID)
					helpers.Relax(err)
					guildAnnouncementSetting := m.getEntryByOrCreateEmpty("guildid", sourceChannel.GuildID)
					guildAnnouncementSetting.GuildID = sourceChannel.GuildID
					var successMessage string
					// Add Text
					if len(args) >= 4 {
						targetChannel, err := helpers.GetChannelFromMention(msg, args[2])
						if err != nil || targetChannel.ID == "" {
							session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
							return
						}

						newText := strings.TrimSpace(strings.Replace(content, strings.Join(args[:3], " "), "", 1))
						guildAnnouncementSetting.GuildJoinEnabled = true
						guildAnnouncementSetting.GuildJoinChannelID = targetChannel.ID
						guildAnnouncementSetting.GuildJoinText = newText
						successMessage = helpers.GetText("plugins.guildannouncements.message-edited")
					} else {
						// Remove Text
						guildAnnouncementSetting.GuildJoinEnabled = false
						successMessage = helpers.GetText("plugins.guildannouncements.message-disabled")
					}
					m.setEntry(guildAnnouncementSetting)
					_, err = session.ChannelMessageSend(msg.ChannelID, successMessage)
					helpers.Relax(err)
				})
			case "guild_leave":
				helpers.RequireAdmin(msg, func() {
					sourceChannel, err := helpers.GetChannel(msg.ChannelID)
					helpers.Relax(err)

					guildAnnouncementSetting := m.getEntryByOrCreateEmpty("guildid", sourceChannel.GuildID)
					guildAnnouncementSetting.GuildID = sourceChannel.GuildID
					var successMessage string
					// Add Text
					if len(args) >= 4 {
						targetChannel, err := helpers.GetChannelFromMention(msg, args[2])
						if err != nil || targetChannel.ID == "" {
							session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
							return
						}

						newText := strings.TrimSpace(strings.Replace(content, strings.Join(args[:3], " "), "", 1))
						guildAnnouncementSetting.GuildLeaveEnabled = true
						guildAnnouncementSetting.GuildLeaveChannelID = targetChannel.ID
						guildAnnouncementSetting.GuildLeaveText = newText
						successMessage = helpers.GetText("plugins.guildannouncements.message-edited")
					} else {
						// Remove Text
						guildAnnouncementSetting.GuildLeaveEnabled = false
						successMessage = helpers.GetText("plugins.guildannouncements.message-disabled")
					}
					m.setEntry(guildAnnouncementSetting)
					_, err = session.ChannelMessageSend(msg.ChannelID, successMessage)
					helpers.Relax(err)
				})
			}
		}
	}

}

func (m *GuildAnnouncements) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {

}

func (m *GuildAnnouncements) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {
	guild, err := helpers.GetGuild(member.GuildID)
	if err != nil {
		raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
	}
	helpers.Relax(err)
	for _, guildAnnouncementSetting := range m.GetAnnouncementSettings() {
		if guildAnnouncementSetting.GuildJoinEnabled == true && guildAnnouncementSetting.GuildID == guild.ID {
			guildJoinChannelID := guildAnnouncementSetting.GuildJoinChannelID
			guildJoinText := m.ReplaceMemberText(guildAnnouncementSetting.GuildJoinText, member)
			if guildJoinText != "" {
				go func() {
					_, err := session.ChannelMessageSend(guildJoinChannelID, guildJoinText)
					if err != nil {
						cache.GetLogger().WithField("module", "guildannouncements").Error(fmt.Sprintf("Error Sending Join Message in %s #%s: %s",
							guild.Name, guild.ID, err.Error()))
					}
				}()
			}
		}
	}
	cache.GetLogger().WithField("module", "guildannouncements").Info(fmt.Sprintf("User %s (%s) joined Guild %s (#%s)", member.User.Username, member.User.ID, guild.Name, guild.ID))

}
func (m *GuildAnnouncements) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {
	guild, err := helpers.GetGuild(member.GuildID)
	if err != nil {
		raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
	}
	for _, guildAnnouncementSetting := range m.GetAnnouncementSettings() {
		if guildAnnouncementSetting.GuildLeaveEnabled == true && guildAnnouncementSetting.GuildID == guild.ID {
			guildLeaveChannelID := guildAnnouncementSetting.GuildLeaveChannelID
			guildLeaveText := m.ReplaceMemberText(guildAnnouncementSetting.GuildLeaveText, member)
			if guildLeaveText != "" {
				go func() {
					_, err := session.ChannelMessageSend(guildLeaveChannelID, guildLeaveText)
					if err != nil {
						cache.GetLogger().WithField("module", "guildannouncements").Error(fmt.Sprintf("Error Sending Leave Message in %s #%s: %s",
							guild.Name, guild.ID, err.Error()))
					}
				}()
			}
		}
	}
	cache.GetLogger().WithField("module", "guildannouncements").Info(fmt.Sprintf("User %s (%s) left Guild %s (#%s)", member.User.Username, member.User.ID, guild.Name, guild.ID))
}

func (m *GuildAnnouncements) GetAnnouncementSettings() []Announcement_Setting {
	var entryBucket []Announcement_Setting
	cursor, err := rethink.Table("guild_announcements").Run(helpers.GetDB())
	helpers.Relax(err)

	err = cursor.All(&entryBucket)
	helpers.Relax(err)

	return entryBucket
}

func (m *GuildAnnouncements) ReplaceMemberText(text string, member *discordgo.Member) string {
	guild, err := cache.GetSession().Guild(member.GuildID)
	if errD, ok := err.(*discordgo.RESTError); ok {
		if errD.Message.Code != 50001 { // It's probably Robyul leaving a server :sob:
			return ""
		} else {
			helpers.Relax(err)
		}
	} else {
		helpers.Relax(err)
	}
	allMembers := make([]*discordgo.Member, 0)
	for _, botGuild := range cache.GetSession().State.Guilds {
		if botGuild.ID == guild.ID {
			for _, member := range guild.Members {
				if member.JoinedAt == "" {
					member, err := helpers.GetGuildMember(member.GuildID, member.User.ID)
					if err == nil && member.JoinedAt != "" {
						allMembers = append(allMembers, member)
					}
				} else {
					allMembers = append(allMembers, member)
				}
			}
		}
	}
	slice.Sort(allMembers[:], func(i, j int) bool {
		iMemberTime, err := discordgo.Timestamp(allMembers[i].JoinedAt).Parse()
		helpers.Relax(err)
		jMemberTime, err := discordgo.Timestamp(allMembers[j].JoinedAt).Parse()
		helpers.Relax(err)
		return iMemberTime.Before(jMemberTime)
	})

	userNumber := -1
	for i, sortedMember := range allMembers[:] {
		if sortedMember.User.ID == member.User.ID {
			userNumber = i + 1
			break
		}
	}

	text = strings.Replace(text, "{USER_USERNAME}", member.User.Username, -1)
	text = strings.Replace(text, "{USER_ID}", member.User.ID, -1)
	text = strings.Replace(text, "{USER_DISCRIMINATOR}", member.User.Discriminator, -1)
	text = strings.Replace(text, "{USER_NUMBER}", strconv.Itoa(userNumber), -1)
	text = strings.Replace(text, "{USER_MENTION}", fmt.Sprintf("<@%s>", member.User.ID), -1)
	text = strings.Replace(text, "{GUILD_NAME}", guild.Name, -1)
	text = strings.Replace(text, "{GUILD_ID}", guild.ID, -1)
	return text
}

func (m *GuildAnnouncements) getEntryByOrCreateEmpty(key string, id string) Announcement_Setting {
	var entryBucket Announcement_Setting
	listCursor, err := rethink.Table("guild_announcements").Filter(
		rethink.Row.Field(key).Eq(id),
	).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer listCursor.Close()
	err = listCursor.One(&entryBucket)

	// If user has no DB entries create an empty document
	if err == rethink.ErrEmptyResult {
		insert := rethink.Table("guild_announcements").Insert(Announcement_Setting{})
		res, e := insert.RunWrite(helpers.GetDB())
		// If the creation was successful read the document
		if e != nil {
			panic(e)
		} else {
			return m.getEntryByOrCreateEmpty("id", res.GeneratedKeys[0])
		}
	} else if err != nil {
		panic(err)
	}

	return entryBucket
}

func (m *GuildAnnouncements) setEntry(entry Announcement_Setting) {
	_, err := rethink.Table("guild_announcements").Update(entry).Run(helpers.GetDB())
	helpers.Relax(err)
}
func (m *GuildAnnouncements) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {

}
func (m *GuildAnnouncements) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {

}
func (m *GuildAnnouncements) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {

}
func (m *GuildAnnouncements) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}
func (m *GuildAnnouncements) OnMessageDelete(msg *discordgo.MessageDelete, session *discordgo.Session) {

}
