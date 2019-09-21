package plugins

import (
	"fmt"
	"strings"

	"sync"

	"time"

	"strconv"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Seklfreak/Robyul2/shardmanager"
	"github.com/bwmarrin/discordgo"
	"github.com/globalsign/mgo/bson"
	"github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack"
)

type Mirror struct{}

func (m *Mirror) Commands() []string {
	return []string{
		"mirror",
		"mirrors",
	}
}

var (
	mirrors []models.MirrorEntry
	// one lock for every channel ID
	mirrorChannelLocks = make(map[string]*sync.Mutex, 0)

	whitelistedBotIDs = []string{
		"470154919463354370", // redvelvet-feed (turtles)
	}
)

func (m *Mirror) Init(session *shardmanager.Manager) {
	var err error
	mirrors, err = m.GetMirrors()
	helpers.Relax(err)

	session.AddHandler(m.OnMessage)
	session.AddHandler(m.OnMessageDelete)
}

func (m *Mirror) Uninit(session *shardmanager.Manager) {

}

func (m *Mirror) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermMirror) {
		return
	}

	args := strings.Fields(content)
	if len(args) >= 1 {
		switch args[0] {
		case "create": // [p]mirror create
			session.ChannelTyping(msg.ChannelID)
			helpers.RequireRobyulMod(msg, func() {
				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)

				newID, err := helpers.MDbInsert(models.MirrorsTable, models.MirrorEntry{
					Type: models.MirrorTypeLink,
				})
				helpers.Relax(err)

				mirrors, err = m.GetMirrors()
				helpers.Relax(err)

				_, err = helpers.EventlogLog(time.Now(), channel.GuildID, helpers.MdbIdToHuman(newID),
					models.EventlogTargetTypeRobyulMirror, msg.Author.ID,
					models.EventlogTypeRobyulMirrorCreate, "",
					nil,
					nil, false)
				helpers.RelaxLog(err)

				cache.GetLogger().WithField("module", "mirror").Info(fmt.Sprintf("Created new Mirror by %s (#%s)", msg.Author.Username, msg.Author.ID))
				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.mirror.create-success",
					helpers.GetPrefixForServer(channel.GuildID), helpers.MdbIdToHuman(newID)))
				helpers.Relax(err)
				return
			})
			return
		case "toggle": // [p]mirror toggle <mirror id>
			session.ChannelTyping(msg.ChannelID)
			helpers.RequireRobyulMod(msg, func() {
				if len(args) < 2 {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					return
				}

				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)

				var mirrorEntry models.MirrorEntry
				err = helpers.MdbOne(
					helpers.MdbCollection(models.MirrorsTable).Find(bson.M{"_id": helpers.HumanToMdbId(args[1])}),
					&mirrorEntry,
				)
				if helpers.IsMdbNotFound(err) {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					return
				}
				helpers.Relax(err)

				beforeType := mirrorEntry.Type

				var typeText string
				switch mirrorEntry.Type {
				case models.MirrorTypeLink:
					mirrorEntry.Type = models.MirrorTypeText
					typeText = "text"
					break
				default:
					mirrorEntry.Type = models.MirrorTypeLink
					typeText = "link"
					break
				}
				err = helpers.MDbUpdate(models.MirrorsTable, mirrorEntry.ID, mirrorEntry)
				helpers.Relax(err)

				mirrors, err = m.GetMirrors()
				helpers.Relax(err)

				_, err = helpers.EventlogLog(time.Now(), channel.GuildID, helpers.MdbIdToHuman(mirrorEntry.ID),
					models.EventlogTargetTypeRobyulMirror, msg.Author.ID,
					models.EventlogTypeRobyulMirrorUpdate, "",
					[]models.ElasticEventlogChange{
						{
							Key:      "mirror_type",
							OldValue: strconv.Itoa(int(beforeType)),
							NewValue: strconv.Itoa(int(mirrorEntry.Type)),
							Type:     models.EventlogTargetTypeRobyulMirrorType,
						},
					},
					nil, false)
				helpers.RelaxLog(err)

				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.mirror.toggle-success", typeText))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			})
			return
		case "add-channel": // [p]mirror add-channel <mirror id> <channel> [<webhook id> <webhook token>]
			session.ChannelTyping(msg.ChannelID)
			// @TODO: more secure way to exchange token: create own webhook if no arguments passed
			helpers.RequireRobyulMod(msg, func() {
				progressMessages, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mirror.add-channel-progress"))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				if len(progressMessages) <= 0 {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.errors.generic-nomessage"))
					return
				}
				progressMessage := progressMessages[0]
				if len(args) < 3 {
					_, err := helpers.EditMessage(msg.ChannelID, progressMessage.ID, helpers.GetText("bot.arguments.too-few"))
					helpers.Relax(err)
					return
				}
				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)
				guild, err := helpers.GetGuild(channel.GuildID)
				helpers.Relax(err)

				var mirrorEntry models.MirrorEntry
				err = helpers.MdbOne(
					helpers.MdbCollection(models.MirrorsTable).Find(bson.M{"_id": helpers.HumanToMdbId(args[1])}),
					&mirrorEntry,
				)
				if helpers.IsMdbNotFound(err) {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					return
				}
				helpers.Relax(err)

				targetChannel, err := helpers.GetChannelFromMention(msg, args[2])
				if err != nil || targetChannel.ID == "" || targetChannel.GuildID != channel.GuildID {
					_, err := helpers.EditMessage(msg.ChannelID, progressMessage.ID, helpers.GetText("bot.arguments.invalid"))
					helpers.Relax(err)
					return
				}

				newMirrorChannel := models.MirrorChannelEntry{
					ChannelID: targetChannel.ID,
					GuildID:   targetChannel.GuildID,
				}

				mirrorEntry.ConnectedChannels = append(mirrorEntry.ConnectedChannels, newMirrorChannel)

				err = helpers.MDbUpdate(models.MirrorsTable, mirrorEntry.ID, mirrorEntry)
				helpers.Relax(err)

				mirrors, err = m.GetMirrors()
				helpers.Relax(err)

				_, err = helpers.EventlogLog(time.Now(), channel.GuildID, helpers.MdbIdToHuman(mirrorEntry.ID),
					models.EventlogTargetTypeRobyulMirror, msg.Author.ID,
					models.EventlogTypeRobyulMirrorUpdate, "",
					nil,
					[]models.ElasticEventlogOption{
						{
							Key:   "mirror_channelids_added",
							Value: newMirrorChannel.ChannelID,
							Type:  models.EventlogTargetTypeChannel,
						},
					}, false)
				helpers.RelaxLog(err)

				cache.GetLogger().WithField("module", "mirror").Info(fmt.Sprintf("Added Channel %s (#%s) on Server %s (#%s) to Mirror %s by %s (#%s)",
					targetChannel.Name, targetChannel.ID, guild.Name, guild.ID, mirrorEntry.ID, msg.Author.Username, msg.Author.ID))
				_, err = helpers.EditMessage(msg.ChannelID, progressMessage.ID, helpers.GetText("plugins.mirror.add-channel-success"))
				helpers.Relax(err)
				return
			})
			return
		case "list": // [p]mirror list
			session.ChannelTyping(msg.ChannelID)
			helpers.RequireRobyulMod(msg, func() {
				session.ChannelTyping(msg.ChannelID)
				var entryBucket []models.MirrorEntry
				err := helpers.MDbIter(helpers.MdbCollection(models.MirrorsTable).Find(nil)).All(&entryBucket)
				helpers.Relax(err)

				if len(entryBucket) <= 0 {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mirror.list-empty"))
					return
				}

				resultMessage := ":fax: Mirrors:\n"
				for _, entry := range entryBucket {
					var entryTypeText string
					switch entry.Type {
					case models.MirrorTypeLink:
						entryTypeText = "link"
					case models.MirrorTypeText:
						entryTypeText = "text"
					}
					resultMessage += fmt.Sprintf(":satellite: Mirror `%s` (Mode: `%s`, %d channels):\n",
						helpers.MdbIdToHuman(entry.ID), entryTypeText, len(entry.ConnectedChannels))
					for _, mirroredChannelEntry := range entry.ConnectedChannels {
						mirroredChannel, err := helpers.GetChannel(mirroredChannelEntry.ChannelID)
						if err != nil {
							resultMessage += fmt.Sprintf(":arrow_forward: `N/A` (`#%s`) on `N/A` `(#%s)`: <#%s>\n",
								mirroredChannelEntry.ChannelID,
								mirroredChannelEntry.GuildID,
								mirroredChannelEntry.ChannelID,
							)
							continue
						}
						mirroredChannelGuild, err := helpers.GetGuild(mirroredChannelEntry.GuildID)
						helpers.Relax(err)
						resultMessage += fmt.Sprintf(":arrow_forward: `#%s` (`#%s`) on `%s` `(#%s)`: <#%s>\n",
							mirroredChannel.Name, mirroredChannel.ID,
							mirroredChannelGuild.Name, mirroredChannelGuild.ID,
							mirroredChannel.ID,
						)
					}
				}
				resultMessage += fmt.Sprintf("Found **%d** Mirrors in total.", len(entryBucket))
				for _, resultPage := range helpers.Pagify(resultMessage, "\n") {
					_, err = helpers.SendMessage(msg.ChannelID, resultPage)
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
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					helpers.Relax(err)
					return
				}

				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)

				var mirrorEntry models.MirrorEntry
				err = helpers.MdbOne(
					helpers.MdbCollection(models.MirrorsTable).Find(bson.M{"_id": helpers.HumanToMdbId(args[1])}),
					&mirrorEntry,
				)
				if helpers.IsMdbNotFound(err) {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					return
				}
				helpers.Relax(err)

				err = helpers.MDbDelete(models.MirrorsTable, mirrorEntry.ID)
				helpers.Relax(err)

				mirrors, err = m.GetMirrors()
				helpers.Relax(err)

				_, err = helpers.EventlogLog(time.Now(), channel.GuildID, helpers.MdbIdToHuman(mirrorEntry.ID),
					models.EventlogTargetTypeRobyulMirror, msg.Author.ID,
					models.EventlogTypeRobyulMirrorDelete, "",
					nil,
					nil, false)
				helpers.RelaxLog(err)

				cache.GetLogger().WithField("module", "mirror").Info(fmt.Sprintf("Deleted Mirror %s by %s (#%s)",
					helpers.MdbIdToHuman(mirrorEntry.ID), msg.Author.Username, msg.Author.ID))
				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mirror.delete-success"))
				helpers.Relax(err)
				return
			})
			return
		case "refresh": // [p]mirror refresh
			session.ChannelTyping(msg.ChannelID)
			helpers.RequireRobyulMod(msg, func() {
				var err error
				session.ChannelTyping(msg.ChannelID)
				mirrors, err = m.GetMirrors()
				helpers.Relax(err)
				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mirror.refreshed-config"))
				helpers.Relax(err)
				return
			})
			return
		}
	}
}

func (m *Mirror) OnMessage(session *discordgo.Session, msg *discordgo.MessageCreate) {
	defer helpers.Recover()

	// ignore bot messages except whitelisted bots
	if msg.Author.Bot {
		var isWhitelisted bool
		for _, whitelistedBotID := range whitelistedBotIDs {
			if msg.Author.ID == whitelistedBotID {
				isWhitelisted = true
			}
		}
		if !isWhitelisted {
			return
		}
	}

	for _, mirrorEntry := range mirrors {
		for _, mirroredChannelEntry := range mirrorEntry.ConnectedChannels {
			if mirroredChannelEntry.ChannelID == msg.ChannelID {
				sourceChannel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)
				// ignore commands
				prefix := helpers.GetPrefixForServer(sourceChannel.GuildID)
				if prefix != "" {
					if strings.HasPrefix(msg.Content, prefix) {
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
				case models.MirrorTypeText:
					m.postMirrorMessage(mirrorEntry, msg.Message, msg.Author, newContent)
					break
				default:
					// post mirror links
					if len(linksToRepost) > 0 {
						sourceGuild, err := helpers.GetGuild(sourceChannel.GuildID)
						helpers.Relax(err)
						for _, linkToRepost := range linksToRepost {
							m.postMirrorMessage(mirrorEntry, msg.Message, msg.Author,
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

func (m *Mirror) postMirrorMessage(mirrorEntry models.MirrorEntry, sourceMessage *discordgo.Message, author *discordgo.User, message string) {
	for _, channelToMirrorToEntry := range mirrorEntry.ConnectedChannels {
		if channelToMirrorToEntry.ChannelID != sourceMessage.ChannelID {
			robyulIsOnTargetGuild := false
			for _, shard := range cache.GetSession().Sessions {
				for _, guild := range shard.State.Guilds {
					if guild.ID == channelToMirrorToEntry.GuildID {
						robyulIsOnTargetGuild = true
					}
				}
			}
			if robyulIsOnTargetGuild {
				webhook, err := helpers.GetWebhook(channelToMirrorToEntry.GuildID, channelToMirrorToEntry.ChannelID)
				if err != nil {
					continue
				}
				result, err := helpers.WebhookExecuteWithResult(
					webhook.ID, webhook.Token,
					&discordgo.WebhookParams{
						Content:   message,
						Username:  author.Username,
						AvatarURL: helpers.GetAvatarUrl(author),
					})
				helpers.RelaxLog(err)
				metrics.MirrorsPostsSent.Add(1)
				err = m.rememberPostedMessage(sourceMessage, result)
				helpers.RelaxLog(err)
			}
		}
	}
}

func (m *Mirror) GetMirrors() (entryBucket []models.MirrorEntry, err error) {
	err = helpers.MDbIter(helpers.MdbCollection(models.MirrorsTable).Find(nil)).All(&entryBucket)
	return entryBucket, err
}

type Mirror_PostedMessage struct {
	ChannelID string
	MessageID string
}

func (m *Mirror) getRememberedMessageKey(sourceMessageID string) (key string) {
	return fmt.Sprintf("robyul2-discord:mirror:postedmessage:%s", sourceMessageID)
}

func (m *Mirror) rememberPostedMessage(sourceMessage *discordgo.Message, mirroredMessage *discordgo.Message) error {
	redis := cache.GetRedisClient()
	key := m.getRememberedMessageKey(sourceMessage.ID)

	item := new(Mirror_PostedMessage)
	item.ChannelID = mirroredMessage.ChannelID
	item.MessageID = mirroredMessage.ID

	itemBytes, err := msgpack.Marshal(&item)
	if err != nil {
		return err
	}

	_, err = redis.LPush(key, itemBytes).Result()
	if err != nil {
		return err
	}

	_, err = redis.Expire(key, time.Hour*1).Result()
	return err
}

func (m *Mirror) getRememberedMessages(sourceMessage *discordgo.Message) ([]Mirror_PostedMessage, error) {
	redis := cache.GetRedisClient()
	key := m.getRememberedMessageKey(sourceMessage.ID)

	length, err := redis.LLen(key).Result()
	if err != nil {
		return []Mirror_PostedMessage{}, err
	}

	if length <= 0 {
		return []Mirror_PostedMessage{}, err
	}

	result, err := redis.LRange(key, 0, length-1).Result()
	if err != nil {
		return []Mirror_PostedMessage{}, err
	}

	rememberedMessages := make([]Mirror_PostedMessage, 0)
	for _, messageData := range result {
		var message Mirror_PostedMessage
		err = msgpack.Unmarshal([]byte(messageData), &message)
		if err != nil {
			continue
		}
		rememberedMessages = append(rememberedMessages, message)
	}

	return rememberedMessages, nil
}

func (m *Mirror) OnMessageDelete(session *discordgo.Session, msg *discordgo.MessageDelete) {
	defer helpers.Recover()

	var err error
	var rememberedMessages []Mirror_PostedMessage

	for _, mirror := range mirrors {
		for _, mirrorChannel := range mirror.ConnectedChannels {
			if mirrorChannel.ChannelID == msg.ChannelID {
				rememberedMessages, err = m.getRememberedMessages(msg.Message)
				helpers.Relax(err)

				for _, messageData := range rememberedMessages {
					err = session.ChannelMessageDelete(messageData.ChannelID, messageData.MessageID)
					if err != nil {
						cache.GetLogger().WithFields(logrus.Fields{
							"module":            "mirror",
							"sourceChannelID":   msg.ChannelID,
							"sourceMessageID":   msg.ID,
							"sourceAuthorID":    msg.Author.ID,
							"mirroredChannelID": messageData.ChannelID,
							"mirroredMessageID": messageData.MessageID,
						}).Warn(
							"Deleting mirrored message failed:", err.Error(),
						)
					}
				}
			}
		}
	}
}
