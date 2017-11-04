package plugins

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/bwmarrin/discordgo"
	rethink "github.com/gorethink/gorethink"
)

type Mirror struct{}

type DB_Mirror_Entry struct {
	ID                string `gorethink:"id,omitempty"`
	Type              string `gorethink:"type"`
	ConnectedChannels []Mirror_Channel
}

type Mirror_Channel struct {
	ChannelID           string
	ChannelWebhookID    string
	ChannelWebhookToken string
	GuildID             string
	ChannelWebhooks     []Mirror_Channel_Webhook
}

type Mirror_Channel_Webhook struct {
	WebhookID    string `gorethink:"webhook_id"`
	WebhookToken string `gorethink:"webhook_token"`
}

func (m *Mirror) Commands() []string {
	return []string{
		"mirror",
		"mirrors",
	}
}

const (
	mirrorUrlRegexText = `(<?https?:\/\/[^\s]+>?)`
)

var (
	mirrorUrlRegex *regexp.Regexp
	mirrors        []DB_Mirror_Entry
)

func (m *Mirror) Init(session *discordgo.Session) {
	mirrorUrlRegex = regexp.MustCompile(mirrorUrlRegexText)
	mirrors = m.GetMirrors()
}

func (m *Mirror) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	args := strings.Fields(content)
	if len(args) >= 1 {
		switch args[0] {
		case "create": // [p]mirror create
			session.ChannelTyping(msg.ChannelID)
			helpers.RequireRobyulMod(msg, func() {
				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)
				newMirrorEntry := m.getEntryByOrCreateEmpty("id", "")
				newMirrorEntry.ConnectedChannels = make([]Mirror_Channel, 0)
				m.setEntry(newMirrorEntry)

				cache.GetLogger().WithField("module", "mirror").Info(fmt.Sprintf("Created new Mirror by %s (#%s)", msg.Author.Username, msg.Author.ID))
				_, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.mirror.create-success",
					helpers.GetPrefixForServer(channel.GuildID), newMirrorEntry.ID))
				helpers.Relax(err)

				mirrors = m.GetMirrors()
				return
			})
			return
		case "toggle": // [p]mirror toggle <mirror id>
			session.ChannelTyping(msg.ChannelID)
			helpers.RequireRobyulMod(msg, func() {
				if len(args) < 2 {
					session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					return
				}

				mirrorID := args[1]
				mirrorEntry := m.getEntryBy("id", mirrorID)
				if mirrorEntry.ID == "" {
					session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					return
				}

				switch mirrorEntry.Type {
				case "text":
					mirrorEntry.Type = "link"
					break
				default:
					mirrorEntry.Type = "text"
					break
				}
				m.setEntry(mirrorEntry)

				go func() {
					mirrors = m.GetMirrors()
				}()

				_, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.mirror.toggle-success", mirrorEntry.Type))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			})
			return
		case "add-channel": // [p]mirror add-channel <mirror id> <channel> [<webhook id> <webhook token>]
			session.ChannelTyping(msg.ChannelID)
			// @TODO: more secure way to exchange token: create own webhook if no arguments passed
			helpers.RequireRobyulMod(msg, func() {
				session.ChannelMessageDelete(msg.ChannelID, msg.ID) // Delete command message to prevent people seeing the token
				progressMessage, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mirror.add-channel-progress"))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				if len(args) < 3 {
					_, err := session.ChannelMessageEdit(msg.ChannelID, progressMessage.ID, helpers.GetText("bot.arguments.too-few"))
					helpers.Relax(err)
					return
				}
				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)
				guild, err := helpers.GetGuild(channel.GuildID)
				helpers.Relax(err)

				mirrorID := args[1]
				mirrorEntry := m.getEntryBy("id", mirrorID)
				if mirrorEntry.ID == "" {
					_, err := session.ChannelMessageEdit(msg.ChannelID, progressMessage.ID, helpers.GetText("bot.arguments.invalid"))
					helpers.Relax(err)
					return
				}

				targetChannel, err := helpers.GetChannelFromMention(msg, args[2])
				if err != nil || targetChannel.ID == "" || targetChannel.GuildID != channel.GuildID {
					_, err := session.ChannelMessageEdit(msg.ChannelID, progressMessage.ID, helpers.GetText("bot.arguments.invalid"))
					helpers.Relax(err)
					return
				}

				newMirrorChannel := Mirror_Channel{
					ChannelID: targetChannel.ID,
					GuildID:   targetChannel.GuildID,
				}

				if len(args) >= 5 {
					targetChannelWebhookId := args[3]
					targetChannelWebhookToken := args[4]

					webhook, err := session.WebhookWithToken(targetChannelWebhookId, targetChannelWebhookToken)
					if err != nil || webhook.GuildID != targetChannel.GuildID || webhook.ChannelID != targetChannel.ID {
						_, err := session.ChannelMessageEdit(msg.ChannelID, progressMessage.ID, helpers.GetText("bot.arguments.invalid"))
						helpers.Relax(err)
						return
					}

					newMirrorChannel.ChannelWebhookID = targetChannelWebhookId
					newMirrorChannel.ChannelWebhookToken = targetChannelWebhookToken
				} else {
					firstWebhook, err := session.WebhookCreate(targetChannel.ID, "Robyul Mirror Webhook 1", "")
					if err != nil {
						if errD, ok := err.(*discordgo.RESTError); ok {
							if errD.Message.Code == discordgo.ErrCodeMissingPermissions {
								_, err = session.ChannelMessageEdit(msg.ChannelID, progressMessage.ID, helpers.GetText("plugins.mirror.add-channel-error-permissions"))
								helpers.Relax(err)
								return
							}
						}
					}
					helpers.Relax(err)

					newMirrorChannel.ChannelWebhooks = append(newMirrorChannel.ChannelWebhooks, Mirror_Channel_Webhook{
						WebhookID:    firstWebhook.ID,
						WebhookToken: firstWebhook.Token,
					})

					secondWebhook, err := session.WebhookCreate(targetChannel.ID, "Robyul Mirror Webhook 2", "")
					helpers.Relax(err)

					newMirrorChannel.ChannelWebhooks = append(newMirrorChannel.ChannelWebhooks, Mirror_Channel_Webhook{
						WebhookID:    secondWebhook.ID,
						WebhookToken: secondWebhook.Token,
					})
				}

				mirrorEntry.ConnectedChannels = append(mirrorEntry.ConnectedChannels, newMirrorChannel)

				m.setEntry(mirrorEntry)

				go func() {
					mirrors = m.GetMirrors()
				}()

				cache.GetLogger().WithField("module", "mirror").Info(fmt.Sprintf("Added Channel %s (#%s) on Server %s (#%s) to Mirror %s by %s (#%s)",
					targetChannel.Name, targetChannel.ID, guild.Name, guild.ID, mirrorEntry.ID, msg.Author.Username, msg.Author.ID))
				_, err = session.ChannelMessageEdit(msg.ChannelID, progressMessage.ID, helpers.GetText("plugins.mirror.add-channel-success"))
				helpers.Relax(err)
				return
			})
			return
		case "list": // [p]mirror list
			session.ChannelTyping(msg.ChannelID)
			helpers.RequireRobyulMod(msg, func() {
				session.ChannelTyping(msg.ChannelID)
				var entryBucket []DB_Mirror_Entry
				listCursor, err := rethink.Table("mirrors").Run(helpers.GetDB())
				helpers.Relax(err)
				defer listCursor.Close()
				err = listCursor.All(&entryBucket)

				if err == rethink.ErrEmptyResult || len(entryBucket) <= 0 {
					session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mirror.list-empty"))
					return
				}
				helpers.Relax(err)

				resultMessage := ":fax: Mirrors:\n"
				for _, entry := range entryBucket {
					entryType := entry.Type
					if entryType == "" {
						entryType = "link"
					}
					resultMessage += fmt.Sprintf(":satellite: Mirror `%s` (Mode: `%s`, %d channels):\n", entry.ID, entryType, len(entry.ConnectedChannels))
					for _, mirroredChannelEntry := range entry.ConnectedChannels {
						if mirroredChannelEntry.ChannelWebhookID != "" && mirroredChannelEntry.ChannelWebhookToken != "" {
							mirroredChannelEntry.ChannelWebhooks = append(mirroredChannelEntry.ChannelWebhooks, Mirror_Channel_Webhook{
								WebhookID:    mirroredChannelEntry.ChannelWebhookID,
								WebhookToken: mirroredChannelEntry.ChannelWebhookToken,
							})
						}
						mirroredChannel, err := helpers.GetChannel(mirroredChannelEntry.ChannelID)
						if err != nil {
							resultMessage += fmt.Sprintf(":arrow_forward: `N/A` (`#%s`) on `N/A` `(#%s)`: <#%s> (Webhooks: `%d`)\n",
								mirroredChannelEntry.ChannelID,
								mirroredChannelEntry.GuildID,
								mirroredChannelEntry.ChannelID,
								len(mirroredChannelEntry.ChannelWebhooks),
							)
							continue
						}
						mirroredChannelGuild, err := helpers.GetGuild(mirroredChannelEntry.GuildID)
						helpers.Relax(err)
						resultMessage += fmt.Sprintf(":arrow_forward: `#%s` (`#%s`) on `%s` `(#%s)`: <#%s> (Webhooks: `%d`)\n",
							mirroredChannel.Name, mirroredChannel.ID,
							mirroredChannelGuild.Name, mirroredChannelGuild.ID,
							mirroredChannel.ID,
							len(mirroredChannelEntry.ChannelWebhooks),
						)
					}
				}
				resultMessage += fmt.Sprintf("Found **%d** Mirrors in total.", len(entryBucket))
				for _, resultPage := range helpers.Pagify(resultMessage, "\n") {
					_, err = session.ChannelMessageSend(msg.ChannelID, resultPage)
					helpers.Relax(err)
				}
				return
			})
			return
		case "delete", "del": // [p]mirror delete <mirror id>
			session.ChannelTyping(msg.ChannelID)
			helpers.RequireRobyulMod(msg, func() {
				session.ChannelTyping(msg.ChannelID)
				if len(args) < 2 {
					_, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					helpers.Relax(err)
					return
				}
				entryId := args[1]
				entryBucket := m.getEntryBy("id", entryId)
				if entryBucket.ID == "" {
					session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mirror.delete-not-found"))
					return
				}
				m.deleteEntryById(entryBucket.ID)

				cache.GetLogger().WithField("module", "mirror").Info(fmt.Sprintf("Deleted Mirror %s by %s (#%s)",
					entryBucket.ID, msg.Author.Username, msg.Author.ID))
				_, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mirror.delete-success"))
				helpers.Relax(err)

				mirrors = m.GetMirrors()
				return
			})
			return
		case "refresh": // [p]mirror refresh
			session.ChannelTyping(msg.ChannelID)
			helpers.RequireRobyulMod(msg, func() {
				session.ChannelTyping(msg.ChannelID)
				mirrors = m.GetMirrors()
				_, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.mirror.refreshed-config"))
				helpers.Relax(err)
				return
			})
			return
		}
	}
}

func (m *Mirror) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {
TryNextMirror:
	for _, mirrorEntry := range mirrors {
		for _, mirroredChannelEntry := range mirrorEntry.ConnectedChannels {
			if mirroredChannelEntry.ChannelID == msg.ChannelID {
				// ignore bot messages
				if msg.Author.Bot == true {
					continue TryNextMirror
				}
				sourceChannel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)
				// ignore commands
				prefix := helpers.GetPrefixForServer(sourceChannel.GuildID)
				if prefix != "" {
					if strings.HasPrefix(content, prefix) {
						return
					}
				}
				var linksToRepost []string
				// get mirror attachements
				if len(msg.Attachments) > 0 {
					for _, attachement := range msg.Attachments {
						linksToRepost = append(linksToRepost, attachement.URL)
					}
				}
				// get mirror links
				if strings.Contains(msg.Content, "http") {
					linksFound := galleryUrlRegex.FindAllString(msg.Content, -1)
					if len(linksFound) > 0 {
						for _, linkFound := range linksFound {
							if strings.HasPrefix(linkFound, "<") == false && strings.HasSuffix(linkFound, ">") == false {
								linksToRepost = append(linksToRepost, linkFound)
							}
						}
					}
				}
				// get full content message
				newContent := msg.Content
				if len(msg.Attachments) > 0 {
					for _, attachement := range msg.Attachments {
						newContent += "\n" + attachement.URL
					}
				}
				switch mirrorEntry.Type {
				case "text":
					m.postMirrorMessage(mirrorEntry, msg.ChannelID, msg.Author, newContent)
					break
				default:
					// post mirror links
					if len(linksToRepost) > 0 {
						sourceGuild, err := helpers.GetGuild(sourceChannel.GuildID)
						helpers.Relax(err)
						for _, linkToRepost := range linksToRepost {
							m.postMirrorMessage(mirrorEntry, msg.ChannelID, msg.Author,
								fmt.Sprintf("posted %s in `#%s` on the `%s` server (<#%s>)",
									linkToRepost, sourceChannel.Name, sourceGuild.Name, sourceChannel.ID,
								),
							)
						}
					}
					break
				}
			}
		}
	}

}

func (m *Mirror) postMirrorMessage(mirrorEntry DB_Mirror_Entry, sourceChannelID string, author *discordgo.User, message string) {
	for _, channelToMirrorToEntry := range mirrorEntry.ConnectedChannels {
		if channelToMirrorToEntry.ChannelID != sourceChannelID {
			robyulIsOnTargetGuild := false
			for _, guild := range cache.GetSession().State.Guilds {
				if guild.ID == channelToMirrorToEntry.GuildID {
					robyulIsOnTargetGuild = true
				}
			}
			if robyulIsOnTargetGuild {
				var webhookID, webhookToken string
				if channelToMirrorToEntry.ChannelWebhookID != "" && channelToMirrorToEntry.ChannelWebhookToken != "" {
					channelToMirrorToEntry.ChannelWebhooks = append(channelToMirrorToEntry.ChannelWebhooks, Mirror_Channel_Webhook{
						WebhookID:    channelToMirrorToEntry.ChannelWebhookID,
						WebhookToken: channelToMirrorToEntry.ChannelWebhookToken,
					})
				}
				for _, channelWebhook := range channelToMirrorToEntry.ChannelWebhooks {
					webhookID = channelWebhook.WebhookID
					webhookToken = channelWebhook.WebhookToken
				}

				err := cache.GetSession().WebhookExecute(webhookID, webhookToken,
					false, &discordgo.WebhookParams{
						Content:   message,
						Username:  author.Username,
						AvatarURL: helpers.GetAvatarUrl(author),
					})
				helpers.RelaxLog(err)
				metrics.MirrorsPostsSent.Add(1)
			}
		}
	}
}

func (m *Mirror) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {
}

func (m *Mirror) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {
}

func (m *Mirror) getEntryBy(key string, id string) DB_Mirror_Entry {
	var entryBucket DB_Mirror_Entry
	listCursor, err := rethink.Table("mirrors").Filter(
		rethink.Row.Field(key).Eq(id),
	).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer listCursor.Close()
	err = listCursor.One(&entryBucket)

	if err == rethink.ErrEmptyResult {
		return entryBucket
	} else if err != nil {
		panic(err)
	}

	return entryBucket
}

func (m *Mirror) getEntryByOrCreateEmpty(key string, id string) DB_Mirror_Entry {
	var entryBucket DB_Mirror_Entry
	listCursor, err := rethink.Table("mirrors").Filter(
		rethink.Row.Field(key).Eq(id),
	).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer listCursor.Close()
	err = listCursor.One(&entryBucket)

	if err == rethink.ErrEmptyResult {
		insert := rethink.Table("mirrors").Insert(DB_Mirror_Entry{})
		res, e := insert.RunWrite(helpers.GetDB())
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

func (m *Mirror) setEntry(entry DB_Mirror_Entry) {
	_, err := rethink.Table("mirrors").Update(entry).Run(helpers.GetDB())
	helpers.Relax(err)
}

func (m *Mirror) deleteEntryById(id string) {
	_, err := rethink.Table("mirrors").Filter(
		rethink.Row.Field("id").Eq(id),
	).Delete().RunWrite(helpers.GetDB())
	helpers.Relax(err)
}

func (m *Mirror) GetMirrors() []DB_Mirror_Entry {
	var entryBucket []DB_Mirror_Entry
	listCursor, err := rethink.Table("mirrors").Run(helpers.GetDB())
	helpers.Relax(err)
	defer listCursor.Close()
	err = listCursor.All(&entryBucket)

	helpers.Relax(err)
	return entryBucket
}

func (m *Mirror) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {

}
func (m *Mirror) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {

}
func (m *Mirror) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {

}
func (m *Mirror) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}
func (m *Mirror) OnMessageDelete(msg *discordgo.MessageDelete, session *discordgo.Session) {

}
