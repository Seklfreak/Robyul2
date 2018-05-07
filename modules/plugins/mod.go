package plugins

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"image/png"

	"github.com/Jeffail/gabs"
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/emojis"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bradfitz/slice"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/getsentry/raven-go"
	"github.com/globalsign/mgo/bson"
	redisCache "github.com/go-redis/cache"
	"github.com/olebedev/when"
	"github.com/olebedev/when/rules/common"
	"github.com/olebedev/when/rules/en"
	"github.com/renstrom/fuzzysearch/fuzzy"
	"github.com/xuri/excelize"
)

type Mod struct {
	parser *when.Parser
}

func (m *Mod) Commands() []string {
	return []string{
		"cleanup",
		"mute",
		"unmute",
		"ban",
		"kick",
		"serverlist",
		"echo",
		"inspect",
		"inspect-extended",
		"auto-inspects-channel",
		"search-user",
		"audit-log",
		"invites",
		"leave-server",
		"say",
		"edit",
		"upload",
		"get",
		"create-invite",
		"prefix",
		"react",
		"toggle-chatlog",
		"pending-unmutes",
		"pending-mutes",
		"batch-roles",
		"set-bot-dp",
		"pin",
	}
}

type CacheInviteInformation struct {
	GuildID         string
	CreatedByUserID string
	Code            string
	CreatedAt       time.Time
	Uses            int
}

var (
	invitesCache map[string][]CacheInviteInformation
)

func (m *Mod) Init(session *discordgo.Session) {
	m.parser = when.New(nil)
	m.parser.Add(en.All...)
	m.parser.Add(common.All...)

	invitesCache = make(map[string][]CacheInviteInformation, 0)
	go func() {
		defer helpers.Recover()

		for _, guild := range session.State.Guilds {
			if helpers.GetMemberPermissions(guild.ID, cache.GetSession().State.User.ID)&discordgo.PermissionManageServer != discordgo.PermissionManageServer &&
				helpers.GetMemberPermissions(guild.ID, cache.GetSession().State.User.ID)&discordgo.PermissionAdministrator != discordgo.PermissionAdministrator {
				continue
			}

		RetryGetGuildInvites:
			invites, err := session.GuildInvites(guild.ID)
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok {
					if strings.Contains(errD.Message.Message, "500: Internal Server Error") {
						cache.GetLogger().WithField("module", "mod").Info("internal server error getting invites for #" + guild.ID + " retrying in 10 seconds")
						time.Sleep(10 * time.Second)
						goto RetryGetGuildInvites
					}
				}
				helpers.RelaxLog(err)
				continue
			}

			cacheInvites := make([]CacheInviteInformation, 0)
			for _, invite := range invites {
				createdAt, err := invite.CreatedAt.Parse()
				if err != nil {
					continue
				}
				inviterID := ""
				if invite.Inviter != nil {
					inviterID = invite.Inviter.ID
				}
				cacheInvites = append(cacheInvites, CacheInviteInformation{
					GuildID:         invite.Guild.ID,
					CreatedByUserID: inviterID,
					Code:            invite.Code,
					CreatedAt:       createdAt,
					Uses:            invite.Uses,
				})
			}

			invitesCache[guild.ID] = cacheInvites
		}
		cache.GetLogger().WithField("module", "mod").Info(fmt.Sprintf("got invite link cache of %d servers", len(invitesCache)))
	}()
	go m.cacheBans()
}

func (m *Mod) Uninit(session *discordgo.Session) {

}

func (m *Mod) cacheBans() {
	defer helpers.Recover()

	time.Sleep(5 * time.Minute)

	var key string
	var guildBansCached int
	cacheCodec := cache.GetRedisCacheCodec()
	cache.GetLogger().WithField("module", "mod").Debug("started bans caching for redis")
	guildBansCached = 0
	for _, botGuild := range cache.GetSession().State.Guilds {
		key = fmt.Sprintf("robyul2-discord:api:bans:%s", botGuild.ID)

		if helpers.GetMemberPermissions(botGuild.ID, cache.GetSession().State.User.ID)&discordgo.PermissionBanMembers != discordgo.PermissionBanMembers &&
			helpers.GetMemberPermissions(botGuild.ID, cache.GetSession().State.User.ID)&discordgo.PermissionAdministrator != discordgo.PermissionAdministrator {
			err := cacheCodec.Set(&redisCache.Item{
				Key:        key,
				Object:     make([]discordgo.GuildBan, 0),
				Expiration: time.Hour * 24 * 30 * 365, // TODO
			})
			helpers.RelaxLog(err)
			continue
		}

		guildBans, err := cache.GetSession().GuildBans(botGuild.ID)
		if err != nil {
			helpers.RelaxLog(err)
			continue
		}

		err = cacheCodec.Set(&redisCache.Item{
			Key:        key,
			Object:     &guildBans,
			Expiration: time.Hour * 24 * 30 * 365, // TODO
		})
		if err != nil {
			raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
		} else {
			guildBansCached += 1
		}
	}
	cache.GetLogger().WithField("module", "mod").Debug(fmt.Sprintf("cached bans for %d guilds in redis", guildBansCached))
}

func (m *Mod) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermMod) {
		return
	}

	regexNumberOnly := regexp.MustCompile(`^\d+$`)

	switch command {
	case "cleanup":
		helpers.RequireMod(msg, func() {
			args := strings.Fields(content)
			if len(args) > 0 {
				switch args[0] {
				case "after": // [p]cleanup after <after message id> [<until message id>]
					if len(args) < 2 {
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
						return
					} else {

						channel, err := helpers.GetChannel(msg.ChannelID)
						helpers.Relax(err)

						afterMessageId := args[1]
						untilMessageId := ""
						if regexNumberOnly.MatchString(afterMessageId) == false {
							helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
							return
						}
						if len(args) >= 3 {
							untilMessageId = args[2]
							if regexNumberOnly.MatchString(untilMessageId) == false {
								helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
								return
							}
						}

						var messagesToDeleteIds []string

						nextAfterID := afterMessageId
					AllMessagesLoop:
						for {
							messagesToDelete, _ := session.ChannelMessages(msg.ChannelID, 100, "", nextAfterID, "")
							slice.Sort(messagesToDelete, func(i, j int) bool {
								return messagesToDelete[i].Timestamp < messagesToDelete[j].Timestamp
							})
							for _, messageToDelete := range messagesToDelete {
								messagesToDeleteIds = append(messagesToDeleteIds, messageToDelete.ID)
								nextAfterID = messageToDelete.ID
								if messageToDelete.ID == untilMessageId {
									break AllMessagesLoop
								}
							}
							if len(messagesToDelete) <= 0 {
								break AllMessagesLoop
							}
						}

						msgAlreadyIn := false
						for _, messageID := range messagesToDeleteIds {
							if messageID == msg.ID {
								msgAlreadyIn = true
							}
						}
						if msgAlreadyIn == false {
							messagesToDeleteIds = append(messagesToDeleteIds, msg.ID)
						}

						messagesToDeleteIds = m.removeDuplicates(messagesToDeleteIds)

						if len(messagesToDeleteIds) <= 10 {
							err := session.ChannelMessagesBulkDelete(msg.ChannelID, messagesToDeleteIds)
							cache.GetLogger().WithField("module", "mod").Info(fmt.Sprintf("Deleted %d messages (command issued by %s (#%s))", len(messagesToDeleteIds), msg.Author.Username, msg.Author.ID))
							if err != nil {
								if errD, ok := err.(*discordgo.RESTError); ok {
									if errD.Message.Code == discordgo.ErrCodeMessageProvidedTooOldForBulkDelete {
										_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.deleting-messages-failed-too-old"))
										helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
										return
									} else if errD.Message.Code == discordgo.ErrCodeMissingPermissions {
										_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.deleting-messages-failed-no-permissions"))
										helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
										return
									} else {
										helpers.Relax(errD)
									}
								} else {
									helpers.Relax(err)
								}
								return
							} else {
								_, err = helpers.EventlogLog(time.Now(), channel.GuildID, "",
									"", msg.Author.ID,
									models.EventlogTypeRobyulCleanup, "",
									nil,
									[]models.ElasticEventlogOption{
										{
											Key:   "cleanup_aftermessageid",
											Value: afterMessageId,
											Type:  models.EventlogTargetTypeMessage,
										},
										{
											Key:   "cleanup_untilmessageid",
											Value: untilMessageId,
											Type:  models.EventlogTargetTypeMessage,
										},
										{
											Key:   "cleanup_deleted_messages",
											Value: strconv.Itoa(len(messagesToDeleteIds)),
										},
									}, false)
								helpers.RelaxLog(err)
							}
						} else {
							if helpers.ConfirmEmbed(msg.ChannelID, msg.Author, helpers.GetTextF("plugins.mod.deleting-message-bulkdelete-confirm", len(messagesToDeleteIds)), "✅", "🚫") == true {
								for i := 0; i < len(messagesToDeleteIds); i += 100 {
									batch := messagesToDeleteIds[i:m.Min(i+100, len(messagesToDeleteIds))]
									err := session.ChannelMessagesBulkDelete(msg.ChannelID, batch)
									cache.GetLogger().WithField("module", "mod").Info(fmt.Sprintf("Deleted %d messages (command issued by %s (#%s))", len(batch), msg.Author.Username, msg.Author.ID))
									if err != nil {
										if errD, ok := err.(*discordgo.RESTError); ok {
											if errD.Message.Code == discordgo.ErrCodeMessageProvidedTooOldForBulkDelete {
												_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.deleting-messages-failed-too-old"))
												helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
												return
											} else if errD.Message.Code == discordgo.ErrCodeMissingPermissions {
												_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.deleting-messages-failed-no-permissions"))
												helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
												return
											} else {
												helpers.Relax(errD)
											}
										} else {
											helpers.Relax(err)
										}
										return
									} else {
										_, err = helpers.EventlogLog(time.Now(), channel.GuildID, "",
											"", msg.Author.ID,
											models.EventlogTypeRobyulCleanup, "",
											nil,
											[]models.ElasticEventlogOption{
												{
													Key:   "cleanup_aftermessageid",
													Value: afterMessageId,
													Type:  models.EventlogTargetTypeMessage,
												},
												{
													Key:   "cleanup_untilmessageid",
													Value: untilMessageId,
													Type:  models.EventlogTargetTypeMessage,
												},
												{
													Key:   "cleanup_deleted_messages",
													Value: strconv.Itoa(len(messagesToDeleteIds)),
												},
											}, false)
										helpers.RelaxLog(err)
									}
								}
							} else {
								session.ChannelMessageDelete(msg.ChannelID, msg.ID)
							}
							return
						}
					}
				case "messages": // [p]cleanup messages <n>
					if len(args) < 2 {
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
						return
					} else {
						channel, err := helpers.GetChannel(msg.ChannelID)
						helpers.Relax(err)

						if regexNumberOnly.MatchString(args[1]) == false {
							helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
							return
						}
						numOfMessagesToDelete, err := strconv.Atoi(args[1])
						if err != nil {
							helpers.SendMessage(msg.ChannelID, fmt.Sprintf(helpers.GetTextF("bot.errors.general"), err.Error()))
							return
						}
						if numOfMessagesToDelete < 1 {
							helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
							return
						}

						var messagesToDeleteIds []string

						messagesLeft := numOfMessagesToDelete + 1
						lastBeforeID := ""
						for {
							messagesToGet := messagesLeft
							if messagesLeft > 100 {
								messagesToGet = 100
							}
							messagesLeft -= messagesToGet

							messagesToDelete, _ := session.ChannelMessages(msg.ChannelID, messagesToGet, lastBeforeID, "", "")
							slice.Sort(messagesToDelete, func(i, j int) bool {
								return messagesToDelete[i].Timestamp < messagesToDelete[j].Timestamp
							})
							for _, messageToDelete := range messagesToDelete {
								messagesToDeleteIds = append(messagesToDeleteIds, messageToDelete.ID)
								lastBeforeID = messageToDelete.ID
							}

							if messagesLeft <= 0 {
								break
							}
						}

						messagesToDeleteIds = m.removeDuplicates(messagesToDeleteIds)

						if len(messagesToDeleteIds) <= 10 {
							err := session.ChannelMessagesBulkDelete(msg.ChannelID, messagesToDeleteIds)
							cache.GetLogger().WithField("module", "mod").Info(fmt.Sprintf("Deleted %d messages (command issued by %s (#%s))", len(messagesToDeleteIds), msg.Author.Username, msg.Author.ID))
							if err != nil {
								if errD, ok := err.(*discordgo.RESTError); ok {
									if errD.Message.Code == discordgo.ErrCodeMessageProvidedTooOldForBulkDelete {
										_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.deleting-messages-failed-too-old"))
										helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
										return
									} else if errD.Message.Code == discordgo.ErrCodeMissingPermissions {
										_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.deleting-messages-failed-no-permissions"))
										helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
										return
									} else {
										helpers.Relax(errD)
									}
								} else {
									helpers.Relax(err)
								}
								return
							} else {
								_, err = helpers.EventlogLog(time.Now(), channel.GuildID, "",
									"", msg.Author.ID,
									models.EventlogTypeRobyulCleanup, "",
									nil,
									[]models.ElasticEventlogOption{
										{
											Key:   "cleanup_deleted_messages",
											Value: strconv.Itoa(len(messagesToDeleteIds)),
										},
									}, false)
								helpers.RelaxLog(err)
							}
						} else {
							if helpers.ConfirmEmbed(msg.ChannelID, msg.Author, helpers.GetTextF("plugins.mod.deleting-message-bulkdelete-confirm", len(messagesToDeleteIds)-1), "✅", "🚫") == true {
								for i := 0; i < len(messagesToDeleteIds); i += 100 {
									batch := messagesToDeleteIds[i:m.Min(i+100, len(messagesToDeleteIds))]
									err := session.ChannelMessagesBulkDelete(msg.ChannelID, batch)
									cache.GetLogger().WithField("module", "mod").Info(fmt.Sprintf("Deleted %d messages (command issued by %s (#%s))", len(batch), msg.Author.Username, msg.Author.ID))
									if err != nil {
										if errD, ok := err.(*discordgo.RESTError); ok {
											if errD.Message.Code == 50034 {
												_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.deleting-messages-failed-too-old"))
												helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
												return
											} else if errD.Message.Code == 50013 {
												_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.deleting-messages-failed-too-old"))
												helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
												return
											} else {
												helpers.Relax(errD)
											}
										} else {
											helpers.Relax(err)
										}
										return
									} else {
										_, err = helpers.EventlogLog(time.Now(), channel.GuildID, "",
											"", msg.Author.ID,
											models.EventlogTypeRobyulCleanup, "",
											nil,
											[]models.ElasticEventlogOption{
												{
													Key:   "cleanup_deleted_messages",
													Value: strconv.Itoa(len(messagesToDeleteIds)),
												},
											}, false)
										helpers.RelaxLog(err)
									}
								}
							} else {
								session.ChannelMessageDelete(msg.ChannelID, msg.ID)
							}
							return
						}
					}
				}
			}
		})
		return
	case "pending-unmutes", "pending-mutes": // [p]pending-unmutes
		helpers.RequireMod(msg, func() {
			session.ChannelTyping(msg.ChannelID)

			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)

			key := "delayed_tasks"
			delayedTasks, err := cache.GetMachineryRedisClient().ZCard(key).Result()
			helpers.Relax(err)

			tasksJson, err := cache.GetMachineryRedisClient().ZRange(key, 0, delayedTasks).Result()
			helpers.Relax(err)

			resultText := ""

			for _, taskJson := range tasksJson {
				task, err := gabs.ParseJSON([]byte(taskJson))
				helpers.Relax(err)

				if task.Path("Name").Data().(string) != "unmute_user" {
					continue
				}

				guildID := task.Path("Args").Index(0).Path("Value").Data().(string)
				userID := task.Path("Args").Index(1).Path("Value").Data().(string)
				etaString := task.Path("ETA").Data().(string)
				eta, err := time.Parse(time.RFC3339, etaString)
				helpers.Relax(err)

				if guildID != channel.GuildID {
					continue
				}

				user, err := helpers.GetUser(userID)
				if err != nil {
					user = new(discordgo.User)
					user.Username = "N/A"
					user.ID = userID
				}

				resultText += fmt.Sprintf("Unmuting %s (`#%s`) at %s UTC\n", user.Username, user.ID, eta.Format(time.ANSIC))
			}

			if resultText == "" {
				resultText = "Found no pending unmutes."
			} else {
				resultText = "Found the following pending unmutes:\n" + resultText
			}

			for _, page := range helpers.Pagify(resultText, "\n") {
				_, err = helpers.SendMessage(msg.ChannelID, page)
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			}
		})
	case "mute": // [p]mute server <User>
		helpers.RequireMod(msg, func() {
			session.ChannelTyping(msg.ChannelID)
			args := strings.Fields(content)
			if len(args) >= 1 {
				targetUser, err := helpers.GetUserFromMention(args[0])
				if err != nil {
					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
					return
				}
				var timeToUnmuteAt time.Time
				if len(args) > 1 {
					timeText := strings.TrimSpace(strings.Replace(content, strings.Join(args[:0], " "), "", 1))
					timeText = strings.Replace(timeText, "for", "in", 1)
					r, err := m.parser.Parse(timeText, time.Now())
					if err != nil || r == nil {
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
						return
					}
					timeToUnmuteAt = r.Time
				}

				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)

				err = helpers.MuteUser(channel.GuildID, targetUser.ID, timeToUnmuteAt)

				successText := helpers.GetTextF("plugins.mod.user-muted-success", targetUser.Username, targetUser.ID)

				var options []models.ElasticEventlogOption
				if time.Now().Before(timeToUnmuteAt) {
					successText = helpers.GetTextF("plugins.mod.user-muted-success-timed", targetUser.Username, targetUser.ID, timeToUnmuteAt.Format(time.ANSIC)+" UTC")
					options = []models.ElasticEventlogOption{
						{
							Key:   "mute_until",
							Value: timeToUnmuteAt.Format(models.ISO8601),
						},
					}
				}

				_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetUser.ID,
					models.EventlogTargetTypeUser, msg.Author.ID,
					models.EventlogTypeRobyulMute, "",
					nil,
					options, false)
				helpers.RelaxLog(err)

				_, err = helpers.SendMessage(msg.ChannelID, successText)
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			} else {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
				return
			}
		})
		return
	case "unmute": // [p]unmute <User>
		helpers.RequireMod(msg, func() {
			session.ChannelTyping(msg.ChannelID)
			args := strings.Fields(content)
			if len(args) >= 1 {
				targetUser, _ := helpers.GetUserFromMention(args[0])
				if targetUser == nil {
					targetUser = new(discordgo.User)
					targetUser.ID = args[0]
					targetUser.Username = "N/A"
				}
				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)

				err = helpers.UnmuteUser(channel.GuildID, targetUser.ID)
				if err != nil {
					if errD, ok := err.(*discordgo.RESTError); ok && errD.Message != nil {
						if errD.Message.Code == discordgo.ErrCodeMissingPermissions {
							helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.mod.user-unmuted-error-permissions"))
							return
						}
					}
				}
				helpers.Relax(err)

				_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetUser.ID,
					models.EventlogTargetTypeUser, msg.Author.ID,
					models.EventlogTypeRobyulUnmute, "",
					nil,
					nil, false)
				helpers.RelaxLog(err)

				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.mod.user-unmuted-success", targetUser.Username, targetUser.ID))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			} else {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
				return
			}
		})
		return
	case "ban": // [p]ban <User> [<Days>] [<Reason>], checks for IsMod and Ban Permissions
		helpers.RequireMod(msg, func() {
			args := strings.Fields(content)
			if len(args) >= 1 {
				// Days Argument
				days := 0
				var err error
				if len(args) >= 2 && regexNumberOnly.MatchString(args[1]) {
					days, err = strconv.Atoi(args[1])
					if err != nil {
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
						return
					}
					if days > 7 {
						_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.mod.user-banned-error-too-many-days"))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					}
				}

				targetUser, err := helpers.GetUserFromMention(args[0])
				if err != nil {
					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
					return
				}
				// Bot can ban?
				botCanBan := false
				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)
				guild, err := helpers.GetGuild(channel.GuildID)
				helpers.Relax(err)
				guildMemberBot, err := helpers.GetGuildMember(guild.ID, session.State.User.ID)
				helpers.Relax(err)
				for _, role := range guild.Roles {
					for _, userRole := range guildMemberBot.Roles {
						if userRole == role.ID && (role.Permissions&discordgo.PermissionBanMembers == discordgo.PermissionBanMembers || role.Permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator) {
							botCanBan = true
						}
					}
				}

				if botCanBan == false {
					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.mod.bot-disallowed"))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}
				// User can ban?
				userCanBan := false
				guildMemberUser, err := helpers.GetGuildMember(guild.ID, msg.Author.ID)
				helpers.Relax(err)
				for _, role := range guild.Roles {
					for _, userRole := range guildMemberUser.Roles {
						if userRole == role.ID && (role.Permissions&discordgo.PermissionBanMembers == discordgo.PermissionBanMembers || role.Permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator) {
							userCanBan = true
						}
					}
				}
				if msg.Author.ID == guild.OwnerID {
					userCanBan = true
				}
				if userCanBan == false {
					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.mod.disallowed"))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}
				// Get Reason
				reasonText := fmt.Sprintf("Issued by: %s#%s (#%s) | Delete Days: %d | Reason: ",
					msg.Author.Username, msg.Author.Discriminator, msg.Author.ID, days)
				if len(args) > 1 {
					if regexNumberOnly.MatchString(args[1]) && len(args) > 1 {
						reasonText += strings.TrimSpace(strings.Replace(content, strings.Join(args[:2], " "), "", 1))
					} else {
						reasonText += strings.TrimSpace(strings.Replace(content, strings.Join(args[:1], " "), "", 1))
					}
				}
				if strings.HasSuffix(reasonText, "Reason: ") {
					reasonText += "None given"
				}
				// Ban user
				err = session.GuildBanCreateWithReason(guild.ID, targetUser.ID, reasonText, days)
				if err != nil {
					if err, ok := err.(*discordgo.RESTError); ok && err.Message != nil {
						if err.Message.Code == 0 {
							_, err := helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.mod.user-banned-failed-too-low"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						} else {
							helpers.Relax(err)
						}
					} else {
						helpers.Relax(err)
					}
				}
				cache.GetLogger().WithField("module", "mod").Info(fmt.Sprintf("Banned User %s (#%s) on Guild %s (#%s) by %s (#%s)", targetUser.Username, targetUser.ID, guild.Name, guild.ID, msg.Author.Username, msg.Author.ID))
				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.mod.user-banned-success", targetUser.Username, targetUser.ID))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			} else {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
				return
			}
		})
		return
	case "kick": // [p]kick <User> [<Reason>], checks for IsMod and Kick Permissions
		helpers.RequireMod(msg, func() {
			args := strings.Fields(content)
			if len(args) >= 1 {
				targetUser, err := helpers.GetUserFromMention(args[0])
				if err != nil {
					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
					return
				}
				// Bot can kick?
				botCanKick := false
				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)
				guild, err := helpers.GetGuild(channel.GuildID)
				helpers.Relax(err)
				guildMemberBot, err := helpers.GetGuildMember(guild.ID, session.State.User.ID)
				helpers.Relax(err)
				for _, role := range guild.Roles {
					for _, userRole := range guildMemberBot.Roles {
						if userRole == role.ID && (role.Permissions&discordgo.PermissionKickMembers == discordgo.PermissionKickMembers || role.Permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator) {
							botCanKick = true
						}
					}
				}
				if botCanKick == false {
					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.mod.bot-disallowed"))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}
				// User can kick?
				userCanKick := false
				guildMemberUser, err := helpers.GetGuildMember(guild.ID, msg.Author.ID)
				helpers.Relax(err)
				for _, role := range guild.Roles {
					for _, userRole := range guildMemberUser.Roles {
						if userRole == role.ID && (role.Permissions&discordgo.PermissionKickMembers == discordgo.PermissionKickMembers || role.Permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator) {
							userCanKick = true
						}
					}
				}
				if msg.Author.ID == guild.OwnerID {
					userCanKick = true
				}
				if userCanKick == false {
					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.mod.disallowed"))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}
				// Get Reason
				reasonText := fmt.Sprintf("Issued by: %s#%s (#%s) | Reason: ",
					msg.Author.Username, msg.Author.Discriminator, msg.Author.ID)
				reasonText += strings.TrimSpace(strings.Replace(content, strings.Join(args[:1], " "), "", 1))
				if strings.HasSuffix(reasonText, "Reason: ") {
					reasonText += "None given"
				}
				// Kick user
				err = session.GuildMemberDeleteWithReason(guild.ID, targetUser.ID, reasonText)
				if err != nil {
					if err, ok := err.(*discordgo.RESTError); ok && err.Message != nil {
						if err.Message.Code == 0 {
							_, err := helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.mod.user-kicked-failed-too-low"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						} else {
							helpers.Relax(err)
						}
					} else {
						helpers.Relax(err)
					}
				}
				cache.GetLogger().WithField("module", "mod").Info(fmt.Sprintf("Kicked User %s (#%s) on Guild %s (#%s) by %s (#%s)", targetUser.Username, targetUser.ID, guild.Name, guild.ID, msg.Author.Username, msg.Author.ID))
				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.mod.user-kicked-success", targetUser.Username, targetUser.ID))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			} else {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
				return
			}
		})
		return
	case "serverlist": // [p]serverlist
		helpers.RequireRobyulMod(msg, func() {
			session.ChannelTyping(msg.ChannelID)

			if strings.Contains(content, "xlsx") {
				sheetname := "Serverlist"

				xlsx := excelize.NewFile()
				index := xlsx.NewSheet(sheetname)
				xlsx.SetActiveSheet(index)
				xlsx.SetCellValue(sheetname, "A1", "Server Name")
				xlsx.SetCellValue(sheetname, "B1", "Server ID")
				xlsx.SetCellValue(sheetname, "C1", "Users")
				xlsx.SetCellValue(sheetname, "D1", "Channels")
				xlsx.SetCellValue(sheetname, "E1", "Serverregion")
				xlsx.SetCellValue(sheetname, "F1", "Serverowner Username")
				xlsx.SetCellValue(sheetname, "G1", "Serverowner ID")

				var row string
				for i, guild := range session.State.Guilds {
					users := make(map[string]string)
					for _, u := range guild.Members {
						users[u.User.ID] = u.User.Username
					}
					owner, err := helpers.GetUser(guild.OwnerID)
					if err != nil || owner == nil {
						owner = new(discordgo.User)
						owner.Username = "N/A"
					}

					row = strconv.Itoa(i + 2)

					xlsx.SetCellValue(sheetname, "A"+row, guild.Name)
					xlsx.SetCellValue(sheetname, "B"+row, "#"+guild.ID)
					xlsx.SetCellValue(sheetname, "C"+row, len(users))
					xlsx.SetCellValue(sheetname, "D"+row, len(guild.Channels))
					xlsx.SetCellValue(sheetname, "E"+row, guild.Region)
					xlsx.SetCellValue(sheetname, "F"+row, "@"+owner.Username)
					xlsx.SetCellValue(sheetname, "G"+row, "#"+guild.OwnerID)
				}
				buf := new(bytes.Buffer)
				err := xlsx.Write(buf)
				helpers.Relax(err)

				_, err = helpers.SendComplex(
					msg.ChannelID, &discordgo.MessageSend{
						Content: fmt.Sprintf("<@%s> Your serverlist is ready:", msg.Author.ID),
						Files: []*discordgo.File{
							{
								Name:   "robyul-serverlist.xlsx",
								Reader: bytes.NewReader(buf.Bytes()),
							},
						},
					})
				helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
				return
			}

			resultText := ""
			totalMembers := 0
			totalChannels := 0
			for _, guild := range session.State.Guilds {
				users := make(map[string]string)
				for _, u := range guild.Members {
					users[u.User.ID] = u.User.Username
				}

				resultText += fmt.Sprintf("`%s` (`#%s`): Channels `%d`, Members: `%d`, Region: `%s`\n",
					guild.Name, guild.ID, len(guild.Channels), len(users), guild.Region)
				totalChannels += len(guild.Channels)
				totalMembers += len(users)
			}
			resultText += fmt.Sprintf("Total Stats: Servers `%d`, Channels: `%d`, Members: `%d`", len(session.State.Guilds), totalChannels, totalMembers)

			for _, resultPage := range helpers.Pagify(resultText, "\n") {
				_, err := helpers.SendMessage(msg.ChannelID, resultPage)
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			}
		})
		return
	case "echo", "say": // [p]echo <channel> <message>
		helpers.RequireMod(msg, func() {
			args := strings.Fields(content)
			if len(args) >= 2 {
				sourceChannel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)
				targetChannel, err := helpers.GetChannelFromMention(msg, args[0])
				if err != nil || targetChannel.ID == "" {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					return
				}
				if sourceChannel.GuildID != targetChannel.GuildID {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.echo-error-wrong-server"))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}

				newText := strings.TrimSpace(strings.Replace(content, strings.Join(args[:1], " "), "", 1))
				newMessages, err := helpers.SendMessage(targetChannel.ID, newText)
				if err != nil {
					if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingAccess {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.echo-error-no-access"))
						return
					}
				}

				if newMessages != nil && len(newMessages) > 0 {
					newMessageIDs := make([]string, 0)
					for _, newMessage := range newMessages {
						newMessageIDs = append(newMessageIDs, newMessage.ID)
					}
					_, err = helpers.EventlogLog(time.Now(), targetChannel.GuildID, strings.Join(newMessageIDs, ","),
						models.EventlogTargetTypeMessage, msg.Author.ID,
						models.EventlogTypeRobyulPostCreate, "",
						nil,
						[]models.ElasticEventlogOption{
							{
								Key:   "post_message",
								Value: newText,
							},
							{
								Key:   "post_channelid",
								Value: targetChannel.ID,
								Type:  models.EventlogTargetTypeChannel,
							},
						}, false)
					helpers.RelaxLog(err)
				}

				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			} else {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
				return
			}
		})
		return
	case "edit": // [p]edit <channel> <message id> <message>
		helpers.RequireAdmin(msg, func() {
			args := strings.Fields(content)
			if len(args) >= 3 {
				sourceChannel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)
				targetChannel, err := helpers.GetChannelFromMention(msg, args[0])
				if err != nil || targetChannel.ID == "" {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					return
				}
				if sourceChannel.GuildID != targetChannel.GuildID {
					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.echo-error-wrong-server"))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}
				targetMessage, err := session.ChannelMessage(targetChannel.ID, args[1])
				if err != nil {
					if errD, ok := err.(*discordgo.RESTError); ok {
						if errD.Message.Code == 10008 || strings.Contains(err.Error(), "is not snowflake") {
							_, err = helpers.SendMessage(sourceChannel.ID, helpers.GetText("plugins.mod.edit-error-not-found"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						} else {
							helpers.Relax(err)
						}
					} else {
						helpers.Relax(err)
					}
				}
				newText := strings.TrimSpace(strings.Replace(content, strings.Join(args[:2], " "), "", 1))
				editMessage, _ := helpers.EditMessage(targetChannel.ID, targetMessage.ID, newText)

				if editMessage != nil && targetMessage != nil {
					_, err = helpers.EventlogLog(time.Now(), targetChannel.GuildID, editMessage.ID,
						models.EventlogTargetTypeMessage, msg.Author.ID,
						models.EventlogTypeRobyulPostUpdate, "",
						[]models.ElasticEventlogChange{
							{
								Key:      "post_message",
								OldValue: targetMessage.Content,
								NewValue: newText,
							},
						},
						[]models.ElasticEventlogOption{
							{
								Key:   "post_channelid",
								Value: targetChannel.ID,
								Type:  models.EventlogTargetTypeChannel,
							},
						}, false)
					helpers.RelaxLog(err)
				}

			} else {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
				return
			}
		})
		return
	case "upload": // [p]upload <channel> + UPLOAD
		helpers.RequireMod(msg, func() {
			args := strings.Fields(content)
			if len(args) >= 1 && len(msg.Attachments) > 0 {
				fileToUpload := helpers.NetGet(msg.Attachments[0].URL)
				sourceChannel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)
				targetChannel, err := helpers.GetChannelFromMention(msg, args[0])
				if err != nil || targetChannel.ID == "" {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					return
				}
				if sourceChannel.GuildID != targetChannel.GuildID {
					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.echo-error-wrong-server"))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}
				newMessage, _ := session.ChannelFileSend(targetChannel.ID, msg.Attachments[0].Filename, bytes.NewReader(fileToUpload))
				if newMessage != nil && newMessage.Attachments != nil && len(newMessage.Attachments) > 0 {
					_, err = helpers.EventlogLog(time.Now(), targetChannel.GuildID, newMessage.ID,
						models.EventlogTargetTypeMessage, msg.Author.ID,
						models.EventlogTypeRobyulPostCreate, "",
						nil,
						[]models.ElasticEventlogOption{
							{
								Key:   "post_attachment_filename",
								Value: msg.Attachments[0].Filename,
							},
							{
								Key:   "post_attachment_link",
								Value: newMessage.Attachments[0].URL,
							},
							{
								Key:   "post_channelid",
								Value: targetChannel.ID,
								Type:  models.EventlogTargetTypeChannel,
							},
						}, false)
					helpers.RelaxLog(err)
				}

			} else {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
				return
			}
		})
		return
	case "get": // [p]get <channel> <message id>
		helpers.RequireMod(msg, func() {
			args := strings.Fields(content)
			if len(args) >= 2 {
				sourceChannel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)
				targetChannel, err := helpers.GetChannelFromMention(msg, args[0])
				if err != nil || targetChannel.ID == "" {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					return
				}
				if sourceChannel.GuildID != targetChannel.GuildID {
					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.echo-error-wrong-server"))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}
				targetMessage, err := session.ChannelMessage(targetChannel.ID, args[1])
				if err != nil {
					if err, ok := err.(*discordgo.RESTError); ok {
						if err.Message.Code == 10008 || err.Message.Code == 50001 {
							_, err := helpers.SendMessage(sourceChannel.ID, helpers.GetText("plugins.mod.edit-error-not-found"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						} else {
							helpers.Relax(err)
						}
					} else {
						helpers.Relax(err)
					}
				}
				newMessage := fmt.Sprintf("```%s```", helpers.ReplaceEmojis(targetMessage.Content))
				_, err = helpers.SendMessage(msg.ChannelID, newMessage)
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			} else {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
				return
			}
		})
		return
	case "react": // [p]react <channel> <message id> <emoji>
		helpers.RequireMod(msg, func() {
			args := strings.Fields(content)
			if len(args) >= 3 {
				sourceChannel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)
				targetChannel, err := helpers.GetChannelFromMention(msg, args[0])
				if err != nil || targetChannel.ID == "" {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					return
				}
				if sourceChannel.GuildID != targetChannel.GuildID {
					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.echo-error-wrong-server"))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}
				targetMessage, err := session.ChannelMessage(targetChannel.ID, args[1])
				if err != nil {
					if errD, ok := err.(*discordgo.RESTError); ok {
						if errD.Message.Code == 10008 || strings.Contains(err.Error(), "is not snowflake") {
							_, err = helpers.SendMessage(sourceChannel.ID, helpers.GetText("plugins.mod.edit-error-not-found"))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							return
						}
						helpers.Relax(err)
					} else {
						helpers.Relax(err)
					}
				}
				session.MessageReactionAdd(targetChannel.ID, targetMessage.ID, strings.Replace(strings.Replace(args[2], ">", "", -1), "<", "", -1))
			} else {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
				return
			}
		})
		return
	case "inspect", "inspect-extended": // [p]inspect[-extended] <user>
		isMod := helpers.IsMod(msg)
		isAllowedToInspectExtended := helpers.CanInspectExtended(msg)
		isAllowedToInspectBasic := helpers.CanInspectBasic(msg)

		if isMod == false && isAllowedToInspectExtended == false && isAllowedToInspectBasic == false {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("mod.no_permission"))
			return
		}

		isExtendedInspect := false
		if command == "inspect-extended" {
			if isAllowedToInspectExtended == false {
				_, err := helpers.SendMessage(msg.ChannelID, "You aren't allowed to do this!")
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			}
			isExtendedInspect = true
		}
		session.ChannelTyping(msg.ChannelID)
		args := strings.Fields(content)
		var targetUser *discordgo.User
		var err error
		if len(args) >= 1 && args[0] != "" {
			targetUser, err = helpers.GetUserFromMention(args[0])
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); (ok && errD.Message.Code == discordgo.ErrCodeUnknownUser) || strings.Contains(err.Error(), "user not found") {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.user-not-found"))
					return
				} else {
					helpers.Relax(err)
				}
			}
			helpers.Relax(err)
			if targetUser.ID == "" {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				return
			}
		} else {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
			return
		}
		if targetUser.ID == session.State.User.ID {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
			return
		}
		textVersion := false
		if len(args) >= 2 && args[1] == "text" {
			textVersion = true
		}
		channel, err := helpers.GetChannel(msg.ChannelID)
		helpers.Relax(err)

		resultEmbed := &discordgo.MessageEmbed{
			Title:       helpers.GetTextF("plugins.mod.inspect-embed-title", targetUser.Username, targetUser.Discriminator),
			Description: helpers.GetText("plugins.mod.inspect-in-progress"),
			URL:         helpers.GetAvatarUrl(targetUser),
			Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: helpers.GetAvatarUrl(targetUser)},
			Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.mod.inspect-embed-footer", targetUser.ID, len(session.State.Guilds))},
			Color:       0x0FADED,
		}
		var resultMessages []*discordgo.Message
		if textVersion == false {
			resultMessages, err = helpers.SendEmbed(msg.ChannelID, resultEmbed)
			helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
		} else {
			resultMessages, err = helpers.SendMessage(msg.ChannelID,
				helpers.GetText("plugins.mod.inspect-in-progress"))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
		}
		if len(resultMessages) <= 0 {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.errors.generic-nomessage"))
			return
		}
		resultMessage := resultMessages[0]

		bannedOnServerList, checkFailedServerList := m.inspectUserBans(targetUser, channel.GuildID)

		resultEmbed.Description = helpers.GetTextF("plugins.mod.inspect-description-done", targetUser.ID)
		resultText := helpers.GetTextF("plugins.mod.inspect-description-done", targetUser.ID)
		resultText += fmt.Sprintf("Username: `%s#%s,` ID: `#%s`",
			targetUser.Username, targetUser.Discriminator, targetUser.ID)
		if helpers.GetAvatarUrl(targetUser) != "" {
			resultText += fmt.Sprintf(", DP: <%s>", helpers.GetAvatarUrl(targetUser))
		}
		resultText += "\n"

		resultBansText := ""
		if len(bannedOnServerList) <= 0 {
			resultBansText += fmt.Sprintf(":white_check_mark: User is banned on none servers.\n:black_medium_small_square:Checked %d servers.\n", len(session.State.Guilds)-len(checkFailedServerList))
		} else {
			if isExtendedInspect == false {
				resultBansText += fmt.Sprintf(":warning: User is banned on **%d** servers.\n:black_medium_small_square:Checked %d servers.\n", len(bannedOnServerList), len(session.State.Guilds)-len(checkFailedServerList))
			} else {
				resultBansText += fmt.Sprintf(":warning: User is banned on **%d** servers:\n", len(bannedOnServerList))
				i := 0
			BannedOnLoop:
				for _, bannedOnServer := range bannedOnServerList {
					resultBansText += fmt.Sprintf(":black_small_square:`%s` (#%s)\n", bannedOnServer.Name, bannedOnServer.ID)
					i++
					if i >= 4 && textVersion == false {
						resultBansText += fmt.Sprintf(":black_small_square: and %d other server(s)\n", len(bannedOnServerList)-(i+1))
						break BannedOnLoop
					}
				}
				resultBansText += fmt.Sprintf(":black_medium_small_square:Checked %d servers.\n", len(session.State.Guilds)-len(checkFailedServerList))
			}
		}

		isOnServerList := m.inspectCommonServers(targetUser)
		commonGuildsText := ""
		if len(isOnServerList) > 0 {
			if isExtendedInspect == false {
				commonGuildsText += fmt.Sprintf(":white_check_mark: User is on **%d** server(s) with Robyul.\n", len(isOnServerList))
			} else {
				commonGuildsText += fmt.Sprintf(":white_check_mark: User is on **%d** server(s) with Robyul:\n", len(isOnServerList))
				i := 0
			ServerListLoop:
				for _, isOnServer := range isOnServerList {
					commonGuildsText += fmt.Sprintf(":black_small_square:`%s` (#%s)\n", isOnServer.Name, isOnServer.ID)
					i++
					if i >= 4 && textVersion == false {
						commonGuildsText += fmt.Sprintf(":black_small_square: and %d other server(s)\n", len(isOnServerList)-(i))
						break ServerListLoop
					}
				}
			}
		} else {
			commonGuildsText += ":question: User is on **none** servers with Robyul.\n"
		}

		joinedTime := helpers.GetTimeFromSnowflake(targetUser.ID)
		oneDayAgo := time.Now().AddDate(0, 0, -1)
		oneWeekAgo := time.Now().AddDate(0, 0, -7)
		joinedTimeText := ""
		if !joinedTime.After(oneWeekAgo) {
			joinedTimeText += fmt.Sprintf(":white_check_mark: User Account got created %s.\n:black_medium_small_square:Joined at %s.\n", helpers.SinceInDaysText(joinedTime), joinedTime.Format(time.ANSIC))
		} else if !joinedTime.After(oneDayAgo) {
			joinedTimeText += fmt.Sprintf(":question: User Account is less than one Week old.\n:black_medium_small_square:Joined at %s.\n", joinedTime.Format(time.ANSIC))
		} else {
			joinedTimeText += fmt.Sprintf(":warning: User Account is less than one Day old.\n:black_medium_small_square:Joined at %s.\n", joinedTime.Format(time.ANSIC))
		}

		troublemakerReports := m.getTroublemakerReports(targetUser)
		var troublemakerReportsText string
		if len(troublemakerReports) <= 0 {
			troublemakerReportsText = ":white_check_mark: User never got reported\n"
		} else {
			troublemakerReportsText = fmt.Sprintf(":warning: User got reported %d time(s)\nUse `_troublemaker list %s` to view the details.\n", len(troublemakerReports), targetUser.ID)
		}

		joins, _ := m.GetJoins(targetUser.ID, channel.GuildID)
		joinsText := ""
		if len(joins) == 0 {
			joinsText = ":white_check_mark: User never joined this server\n"
		} else if len(joins) == 1 {
			if joins[0].InviteCodeUsed != "" {
				createdByUser, _ := helpers.GetUser(joins[0].InviteCodeCreatedByUserID)
				if createdByUser == nil {
					createdByUser = new(discordgo.User)
					createdByUser.ID = joins[0].InviteCodeCreatedByUserID
					createdByUser.Username = "N/A"
				}

				var labelText string
				if joins[0].VanityInviteUsedName != "" {
					labelText = " (`" + helpers.GetConfig().Path("website.vanityurl_domain").Data().(string) + "/" + joins[0].VanityInviteUsedName + "`)"
				}

				joinsText = fmt.Sprintf(":white_check_mark: User joined this server once (%s) with the invite `%s`%s created by `%s (#%s)` %s\n",
					humanize.Time(joins[0].JoinedAt), joins[0].InviteCodeUsed, labelText, createdByUser.Username,
					createdByUser.ID, humanize.Time(joins[0].InviteCodeCreatedAt))
			} else {
				joinsText = fmt.Sprintf(":white_check_mark: User joined this server once (%s)\nGive Robyul the `Manage Server` permission to see using which invite.\n",
					humanize.Time(joins[0].JoinedAt))
			}
		} else if len(joins) > 1 {
			sort.Slice(joins, func(i, j int) bool { return joins[i].JoinedAt.After(joins[j].JoinedAt) })
			lastJoin := joins[0]

			if lastJoin.InviteCodeUsed != "" {
				createdByUser, _ := helpers.GetUser(lastJoin.InviteCodeCreatedByUserID)
				if createdByUser == nil {
					createdByUser = new(discordgo.User)
					createdByUser.ID = lastJoin.InviteCodeCreatedByUserID
					createdByUser.Username = "N/A"
				}

				var labelText string
				if lastJoin.VanityInviteUsedName != "" {
					labelText = " (`" + helpers.GetConfig().Path("website.vanityurl_domain").Data().(string) + "/" + joins[0].VanityInviteUsedName + "`)"
				}

				joinsText = fmt.Sprintf(":warning: User joined this server %d times (last time %s)\n"+
					"Last time with the invite `%s`%s created by `%s (#%s)` %s\n",
					len(joins), humanize.Time(lastJoin.JoinedAt), lastJoin.InviteCodeUsed,
					labelText, createdByUser.Username, createdByUser.ID, humanize.Time(lastJoin.InviteCodeCreatedAt))
			} else {
				joinsText = fmt.Sprintf(":warning: User joined this server %d times (last time %s)\n"+
					"Give Robyul the `Manage Server` permission to see using which invites.\n",
					len(joins), humanize.Time(lastJoin.JoinedAt))
			}
		}

		isBannedOnBansdiscordlistNet, err := helpers.IsBannedOnBansdiscordlistNet(targetUser.ID)
		helpers.RelaxLog(err)
		isBannedOnBansdiscordlistNetText := ":white_check_mark: User is not banned.\n"
		isBannedOnBansdiscordlistNetTextText := ":white_check_mark: User is not banned on <https://bans.discordlist.net/>.\n"
		if isBannedOnBansdiscordlistNet {
			isBannedOnBansdiscordlistNetText = ":warning: User is banned on [bans.discordlist.net](https://bans.discordlist.net/).\n"
			isBannedOnBansdiscordlistNetTextText = ":warning: User is banned on <https://bans.discordlist.net/>.\n"
		}

		resultEmbed.Fields = []*discordgo.MessageEmbedField{
			{Name: "Bans", Value: resultBansText, Inline: false},
			{Name: "Troublemaker Reports", Value: troublemakerReportsText, Inline: false},
			{Name: "bans.discordlist.net", Value: isBannedOnBansdiscordlistNetText, Inline: false},
			{Name: "Join History", Value: joinsText, Inline: false},
			{Name: "Common Servers", Value: commonGuildsText, Inline: false},
			{Name: "Account Age", Value: joinedTimeText, Inline: false},
		}
		resultText += resultBansText
		resultText += troublemakerReportsText
		resultText += isBannedOnBansdiscordlistNetTextText
		resultText += joinsText
		resultText += commonGuildsText
		resultText += joinedTimeText

		for _, failedServer := range checkFailedServerList {
			if failedServer.ID == channel.GuildID {
				noAccessToBansText := "\n:warning: I wasn't able to gather the ban list for this server!\nPlease give Robyul the permission `Ban Members` to help other servers.\n"
				resultEmbed.Description += noAccessToBansText
				resultText += noAccessToBansText
				break
			}
		}

		if textVersion == false {
			helpers.EditEmbed(msg.ChannelID, resultMessage.ID, resultEmbed)
		} else {
			pages := helpers.Pagify(resultText, "\n")
			if len(pages) <= 1 {
				_, err = helpers.EditMessage(msg.ChannelID, resultMessage.ID, resultText)
				helpers.Relax(err)
			} else {
				session.ChannelMessageDelete(msg.ChannelID, resultMessage.ID)
				for _, page := range pages {
					_, err = helpers.SendMessage(msg.ChannelID, page)
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				}
			}
		}
		return
	case "auto-inspects-channel": // [p]auto-inspects-channel [<channel id>]
		helpers.RequireAdmin(msg, func() {
			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)
			settings := helpers.GuildSettingsGetCached(channel.GuildID)
			args := strings.Fields(content)
			var successMessage string
			// Add Text
			if len(args) >= 1 {
				targetChannel, err := helpers.GetChannelFromMention(msg, args[0])
				if err != nil || targetChannel.ID == "" {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					return
				}

				chooseEmbed := &discordgo.MessageEmbed{
					Title:       fmt.Sprintf("@%s Enable Auto Inspect Triggers", msg.Author.Username),
					Description: "**Please wait a second...** :construction_site:",
					Footer:      &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Robyul is currently on %d servers.", len(session.State.Guilds))},
					Color:       0x0FADED,
				}
				chooseMessages, err := helpers.SendEmbed(msg.ChannelID, chooseEmbed)
				helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
				if len(chooseMessages) <= 0 {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.errors.generic-nomessage"))
					return
				}
				chooseMessage := chooseMessages[0]

				allowedEmotes := []string{
					emojis.From("1"),
					emojis.From("2"),
					emojis.From("3"),
					emojis.From("4"),
					emojis.From("5"),
					emojis.From("6"),
					emojis.From("9"),
					"💾"}
				for _, allowedEmote := range allowedEmotes {
					err = session.MessageReactionAdd(msg.ChannelID, chooseMessage.ID, allowedEmote)
					helpers.Relax(err)
				}

				channelIDBefore := settings.InspectsChannel
				settingsBefore := settings.InspectTriggersEnabled

				needEmbedUpdate := true
				emotesLocked := false

				// @TODO: use reaction event, see stats.go
			HandleChooseReactions:
				for {
					saveAndExits, _ := cache.GetSession().MessageReactions(msg.ChannelID, chooseMessage.ID, "💾", 100)
					for _, saveAndExit := range saveAndExits {
						if saveAndExit.ID == msg.Author.ID {
							// user wants to exit
							session.MessageReactionRemove(msg.ChannelID, chooseMessage.ID, "💾", msg.Author.ID)
							break HandleChooseReactions
						}
					}
					numberOnes, _ := cache.GetSession().MessageReactions(msg.ChannelID, chooseMessage.ID, emojis.From("1"), 100)
					for _, numberOne := range numberOnes {
						if numberOne.ID == msg.Author.ID {
							if settings.InspectTriggersEnabled.UserBannedOnOtherServers && emotesLocked == false {
								settings.InspectTriggersEnabled.UserBannedOnOtherServers = false
							} else {
								settings.InspectTriggersEnabled.UserBannedOnOtherServers = true
							}
							needEmbedUpdate = true
							err := session.MessageReactionRemove(msg.ChannelID, chooseMessage.ID, emojis.From("1"), msg.Author.ID)
							if err != nil {
								emotesLocked = true
							}
						}
					}
					numberTwos, _ := cache.GetSession().MessageReactions(msg.ChannelID, chooseMessage.ID, emojis.From("2"), 100)
					for _, numberTwo := range numberTwos {
						if numberTwo.ID == msg.Author.ID {
							if settings.InspectTriggersEnabled.UserNoCommonServers && emotesLocked == false {
								settings.InspectTriggersEnabled.UserNoCommonServers = false
							} else {
								settings.InspectTriggersEnabled.UserNoCommonServers = true
							}
							needEmbedUpdate = true
							err := session.MessageReactionRemove(msg.ChannelID, chooseMessage.ID, emojis.From("2"), msg.Author.ID)
							if err != nil {
								emotesLocked = true
							}
						}
					}
					NumberThrees, _ := cache.GetSession().MessageReactions(msg.ChannelID, chooseMessage.ID, emojis.From("3"), 100)
					for _, NumberThree := range NumberThrees {
						if NumberThree.ID == msg.Author.ID {
							if settings.InspectTriggersEnabled.UserNewlyCreatedAccount && emotesLocked == false {
								settings.InspectTriggersEnabled.UserNewlyCreatedAccount = false
							} else {
								settings.InspectTriggersEnabled.UserNewlyCreatedAccount = true
							}
							needEmbedUpdate = true
							err := session.MessageReactionRemove(msg.ChannelID, chooseMessage.ID, emojis.From("3"), msg.Author.ID)
							if err != nil {
								emotesLocked = true
							}
						}
					}
					NumberFours, _ := cache.GetSession().MessageReactions(msg.ChannelID, chooseMessage.ID, emojis.From("4"), 100)
					for _, NumberFour := range NumberFours {
						if NumberFour.ID == msg.Author.ID {
							if settings.InspectTriggersEnabled.UserReported && emotesLocked == false {
								settings.InspectTriggersEnabled.UserReported = false
							} else {
								settings.InspectTriggersEnabled.UserReported = true
							}
							needEmbedUpdate = true
							err := session.MessageReactionRemove(msg.ChannelID, chooseMessage.ID, emojis.From("4"), msg.Author.ID)
							if err != nil {
								emotesLocked = true
							}
						}
					}
					NumberFives, _ := cache.GetSession().MessageReactions(msg.ChannelID, chooseMessage.ID, emojis.From("5"), 100)
					for _, NumberFive := range NumberFives {
						if NumberFive.ID == msg.Author.ID {
							if settings.InspectTriggersEnabled.UserMultipleJoins && emotesLocked == false {
								settings.InspectTriggersEnabled.UserMultipleJoins = false
							} else {
								settings.InspectTriggersEnabled.UserMultipleJoins = true
							}
							needEmbedUpdate = true
							err := session.MessageReactionRemove(msg.ChannelID, chooseMessage.ID, emojis.From("5"), msg.Author.ID)
							if err != nil {
								emotesLocked = true
							}
						}
					}
					NumberSixes, _ := cache.GetSession().MessageReactions(msg.ChannelID, chooseMessage.ID, emojis.From("6"), 100)
					for _, NumberSix := range NumberSixes {
						if NumberSix.ID == msg.Author.ID {
							if settings.InspectTriggersEnabled.UserBannedDiscordlistNet && emotesLocked == false {
								settings.InspectTriggersEnabled.UserBannedDiscordlistNet = false
							} else {
								settings.InspectTriggersEnabled.UserBannedDiscordlistNet = true
							}
							needEmbedUpdate = true
							err := session.MessageReactionRemove(msg.ChannelID, chooseMessage.ID, emojis.From("6"), msg.Author.ID)
							if err != nil {
								emotesLocked = true
							}
						}
					}
					NumberNines, _ := cache.GetSession().MessageReactions(msg.ChannelID, chooseMessage.ID, emojis.From("9"), 100)
					for _, NumberNine := range NumberNines {
						if NumberNine.ID == msg.Author.ID {
							if settings.InspectTriggersEnabled.UserJoins && emotesLocked == false {
								settings.InspectTriggersEnabled.UserJoins = false
							} else {
								settings.InspectTriggersEnabled.UserJoins = true
							}
							needEmbedUpdate = true
							err := session.MessageReactionRemove(msg.ChannelID, chooseMessage.ID, emojis.From("9"), msg.Author.ID)
							if err != nil {
								emotesLocked = true
							}
						}
					}

					if needEmbedUpdate == true {
						chooseEmbed.Description = fmt.Sprintf(
							"Choose which warnings should trigger an automatic inspect post in <#%s>.\n"+
								"**Available Triggers**\n",
							targetChannel.ID)
						enabledEmote := ":black_square_button:"
						if settings.InspectTriggersEnabled.UserBannedOnOtherServers {
							enabledEmote = ":heavy_check_mark:"
						}
						chooseEmbed.Description += fmt.Sprintf("%s %s User is banned on a different server with Robyul on. Gets checked everytime an user joins or gets banned on a different server with Robyul on.\n",
							emojis.FromToText("1"), enabledEmote)

						enabledEmote = ":black_square_button:"
						if settings.InspectTriggersEnabled.UserNoCommonServers {
							enabledEmote = ":heavy_check_mark:"
						}
						chooseEmbed.Description += fmt.Sprintf("%s %s User has none other common servers with Robyul. Gets checked everytime an user joins.\n",
							emojis.FromToText("2"), enabledEmote)

						enabledEmote = ":black_square_button:"
						if settings.InspectTriggersEnabled.UserNewlyCreatedAccount {
							enabledEmote = ":heavy_check_mark:"
						}
						chooseEmbed.Description += fmt.Sprintf("%s %s Account is less than one week old. Gets checked everytime an user joins.\n",
							emojis.FromToText("3"), enabledEmote)

						enabledEmote = ":black_square_button:"
						if settings.InspectTriggersEnabled.UserReported {
							enabledEmote = ":heavy_check_mark:"
						}
						chooseEmbed.Description += fmt.Sprintf("%s %s Account got reported as a troublemaker. Gets checked everytime an user joins.\n",
							emojis.FromToText("4"), enabledEmote)

						enabledEmote = ":black_square_button:"
						if settings.InspectTriggersEnabled.UserMultipleJoins {
							enabledEmote = ":heavy_check_mark:"
						}
						chooseEmbed.Description += fmt.Sprintf("%s %s Account joined this server more than once. Gets checked everytime an user joins.\n",
							emojis.FromToText("5"), enabledEmote)

						enabledEmote = ":black_square_button:"
						if settings.InspectTriggersEnabled.UserBannedDiscordlistNet {
							enabledEmote = ":heavy_check_mark:"
						}
						chooseEmbed.Description += fmt.Sprintf("%s %s Account is banned on bans.discordlist.net, a global (unofficial) discord ban list. Gets checked everytime an user joins.\n",
							emojis.FromToText("6"), enabledEmote)

						enabledEmote = ":black_square_button:"
						if settings.InspectTriggersEnabled.UserJoins {
							enabledEmote = ":heavy_check_mark:"
						}
						chooseEmbed.Description += fmt.Sprintf("%s %s Account joins this server. Triggers on every join.\n",
							emojis.FromToText("9"), enabledEmote)

						if emotesLocked == true {
							chooseEmbed.Description += fmt.Sprintf(":warning: Please give Robyul the `Manage Messages` permission to be able to disable triggers or disable all triggers using `%sauto-inspects-channel`.\n",
								helpers.GetPrefixForServer(channel.GuildID),
							)
						}
						chooseEmbed.Description += "Use :floppy_disk: to save and exit."
						chooseMessage, err = helpers.EditEmbed(msg.ChannelID, chooseMessage.ID, chooseEmbed)
						helpers.Relax(err)
						needEmbedUpdate = false
					}

					time.Sleep(1 * time.Second)
				}

				for _, allowedEmote := range allowedEmotes {
					session.MessageReactionRemove(msg.ChannelID, chooseMessage.ID, allowedEmote, session.State.User.ID)
				}
				settings.InspectsChannel = targetChannel.ID

				_, err = helpers.EventlogLog(time.Now(), channel.GuildID, "",
					models.EventlogTargetTypeChannel, msg.Author.ID,
					models.EventlogTypeRobyulAutoInspectsChannel, "",
					[]models.ElasticEventlogChange{
						{
							Key:      "autoinspectschannel_channelid",
							OldValue: channelIDBefore,
							NewValue: targetChannel.ID,
							Type:     models.EventlogTargetTypeChannel,
						},
						{
							Key:      "autoinspectschannel_userbannedonotherservers",
							OldValue: helpers.StoreBoolAsString(settingsBefore.UserBannedOnOtherServers),
							NewValue: helpers.StoreBoolAsString(settings.InspectTriggersEnabled.UserBannedOnOtherServers),
						},
						{
							Key:      "autoinspectschannel_usernocommonservers",
							OldValue: helpers.StoreBoolAsString(settingsBefore.UserNoCommonServers),
							NewValue: helpers.StoreBoolAsString(settings.InspectTriggersEnabled.UserNoCommonServers),
						},
						{
							Key:      "autoinspectschannel_usernewlycreatedaccount",
							OldValue: helpers.StoreBoolAsString(settingsBefore.UserNewlyCreatedAccount),
							NewValue: helpers.StoreBoolAsString(settings.InspectTriggersEnabled.UserNewlyCreatedAccount),
						},
						{
							Key:      "autoinspectschannel_userreported",
							OldValue: helpers.StoreBoolAsString(settingsBefore.UserReported),
							NewValue: helpers.StoreBoolAsString(settings.InspectTriggersEnabled.UserReported),
						},
						{
							Key:      "autoinspectschannel_usermultiplejoins",
							OldValue: helpers.StoreBoolAsString(settingsBefore.UserMultipleJoins),
							NewValue: helpers.StoreBoolAsString(settings.InspectTriggersEnabled.UserMultipleJoins),
						},
						{
							Key:      "autoinspectschannel_userbanneddiscordlistnet",
							OldValue: helpers.StoreBoolAsString(settingsBefore.UserBannedDiscordlistNet),
							NewValue: helpers.StoreBoolAsString(settings.InspectTriggersEnabled.UserBannedDiscordlistNet),
						},
						{
							Key:      "autoinspectschannel_userjoins",
							OldValue: helpers.StoreBoolAsString(settingsBefore.UserJoins),
							NewValue: helpers.StoreBoolAsString(settings.InspectTriggersEnabled.UserJoins),
						},
					},
					nil, false)
				helpers.RelaxLog(err)

				chooseEmbed.Description = strings.Replace(chooseEmbed.Description, "Use :floppy_disk: to save and exit.", "Saved.", -1)
				helpers.EditEmbed(msg.ChannelID, chooseMessage.ID, chooseEmbed)

				successMessage = helpers.GetText("plugins.mod.inspects-channel-set")
			} else {
				channelIDBefore := settings.InspectsChannel
				settings.InspectsChannel = ""
				settings.InspectTriggersEnabled.UserBannedOnOtherServers = false
				settings.InspectTriggersEnabled.UserNoCommonServers = false
				settings.InspectTriggersEnabled.UserNewlyCreatedAccount = false
				settings.InspectTriggersEnabled.UserReported = false
				settings.InspectTriggersEnabled.UserMultipleJoins = false
				settings.InspectTriggersEnabled.UserBannedDiscordlistNet = false
				settings.InspectTriggersEnabled.UserJoins = false
				successMessage = helpers.GetText("plugins.mod.inspects-channel-disabled")

				_, err = helpers.EventlogLog(time.Now(), channel.GuildID, "",
					models.EventlogTargetTypeChannel, msg.Author.ID,
					models.EventlogTypeRobyulAutoInspectsChannel, "",
					[]models.ElasticEventlogChange{
						{
							Key:      "autoinspectschannel_channelid",
							OldValue: channelIDBefore,
							NewValue: "",
							Type:     models.EventlogTargetTypeChannel,
						},
					},
					nil, false)
				helpers.RelaxLog(err)
			}
			err = helpers.GuildSettingsSet(channel.GuildID, settings)
			helpers.Relax(err)
			_, err = helpers.SendMessage(msg.ChannelID, successMessage)
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
		})
		return
	case "search-user": // [p]search-user <name>
		helpers.RequireMod(msg, func() {
			searchText := strings.TrimSpace(content)
			if len(searchText) > 3 {
				globalCheck := helpers.IsBotAdmin(msg.Author.ID)
				if globalCheck == true {
					helpers.SendMessage(msg.ChannelID, "Searching for users on all servers with Robyul. :speech_balloon:")
				} else {
					helpers.SendMessage(msg.ChannelID, "Searching for users on this server. :speech_balloon:")
				}

				currentChannel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)

				usersMatched := make([]*discordgo.User, 0)
				for _, serverGuild := range session.State.Guilds {
					if globalCheck == true || serverGuild.ID == currentChannel.GuildID {
						members := make([]*discordgo.Member, 0)
						for _, botGuild := range session.State.Guilds {
							if botGuild.ID == serverGuild.ID {
								for _, member := range botGuild.Members {
									members = append(members, member)
								}
							}
						}

						for _, serverMember := range members {
							fullUserNameToSearch := serverMember.User.Username + "#" + serverMember.User.Discriminator + " ~ " + serverMember.Nick + " ~ " + serverMember.User.ID
							if fuzzy.MatchFold(searchText, fullUserNameToSearch) {
								userIsAlreadyInList := false
								for _, userAlreadyInList := range usersMatched {
									if userAlreadyInList.ID == serverMember.User.ID {
										userIsAlreadyInList = true
									}
								}
								if userIsAlreadyInList == false {
									usersMatched = append(usersMatched, serverMember.User)
								}
							}
						}
					}
				}

				if len(usersMatched) <= 0 {
					_, err := helpers.SendMessage(msg.ChannelID, "Found no user who matches your search text. :spy:")
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				} else {
					resultText := fmt.Sprintf("Found %d users which matches your search text:\n", len(usersMatched))
					for _, userMatched := range usersMatched {
						resultText += fmt.Sprintf("`%s#%s` (User ID: `%s`)\n", userMatched.Username, userMatched.Discriminator, userMatched.ID)
					}
					for _, page := range helpers.Pagify(resultText, "\n") {
						_, err := helpers.SendMessage(msg.ChannelID, page)
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					}
					return
				}
			} else {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
				return
			}
		})
		return
	case "audit-log":
		helpers.RequireBotAdmin(msg, func() {
			session.ChannelTyping(msg.ChannelID)
			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)
			auditLogUrl := fmt.Sprintf(discordgo.EndpointAPI+"guilds/%s/audit-logs?limit=10", channel.GuildID)
			resp, err := session.Request("GET", auditLogUrl, nil)
			helpers.Relax(err)
			parsedResult, err := gabs.ParseJSON(resp)
			helpers.Relax(err)

			logMessage := ""

			var user *discordgo.Member
			var target *discordgo.Member

			logEntries, err := parsedResult.Path("audit_log_entries").Children()
			helpers.Relax(err)
			for _, logEntry := range logEntries {
				user, err = helpers.GetGuildMember(channel.GuildID, strings.Replace(logEntry.Path("user_id").String(), "\"", "", -1))
				if err != nil {
					user = new(discordgo.Member)
					user.User = new(discordgo.User)
					user.User.Username = "N/A"
				}
				target, err = helpers.GetGuildMember(channel.GuildID, strings.Replace(logEntry.Path("target_id").String(), "\"", "", -1))
				if err != nil {
					target = new(discordgo.Member)
					target.User = new(discordgo.User)
					target.User.Username = "N/A"
				}
				logMessage += "**Action** `" + logEntry.Path("action_type").String() + "` from `" + user.User.Username + "` for `" + target.User.Username + "`:\n"
				logEntryChanges, err := logEntry.Path("changes").Children()
				helpers.Relax(err)
				for _, logEntryChange := range logEntryChanges {
					logMessage += logEntryChange.Path("key").String() + ": " + logEntryChange.Path("new_value").String() + "\n"
				}
			}

			for _, page := range helpers.Pagify(logMessage, "\n") {
				_, err := helpers.SendMessage(msg.ChannelID, page)
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			}
		})
		return
	case "invites":
		helpers.RequireBotAdmin(msg, func() {
			session.ChannelTyping(msg.ChannelID)

			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)
			guildID := channel.GuildID

			args := strings.Fields(content)
			if len(args) >= 1 {
				guild, err := helpers.GetGuild(args[0])
				helpers.Relax(err)
				guildID = guild.ID
			}
			invitesUrl := fmt.Sprintf(discordgo.EndpointAPI+"guilds/%s/invites", guildID)
			resp, err := session.Request("GET", invitesUrl, nil)
			helpers.Relax(err)
			parsedResult, err := gabs.ParseJSON(resp)
			helpers.Relax(err)

			invites, err := parsedResult.Children()
			helpers.Relax(err)

			if len(invites) <= 0 {
				_, err := helpers.SendMessage(msg.ChannelID, "No invites found on this server. <:blobscream:317043778823389184>")
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			}

			inviteMessage := ""
			for _, invite := range invites {
				inviteCode := strings.Trim(invite.Path("code").String(), "\"")
				inviteInviter, err := helpers.GetGuildMember(channel.GuildID, strings.Trim(invite.Path("inviter.id").String(), "\""))
				if err != nil {
					inviteInviter = new(discordgo.Member)
					inviteInviter.User = new(discordgo.User)
					inviteInviter.User.Username = "N/A"
				}
				inviteUses := invite.Path("uses").String()
				inviteChannelID := strings.Trim(invite.Path("channel.id").String(), "\"")
				inviteMessage += fmt.Sprintf("`%s` by `%s` to <#%s>: **%s** uses\n",
					inviteCode, inviteInviter.User.Username, inviteChannelID, inviteUses)
			}

			for _, page := range helpers.Pagify(inviteMessage, "\n") {
				_, err := helpers.SendMessage(msg.ChannelID, page)
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			}
		})
		return
	case "leave-server":
		helpers.RequireRobyulMod(msg, func() {
			session.ChannelTyping(msg.ChannelID)
			args := strings.Fields(content)
			if len(args) < 1 {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				return
			}

			targetGuild, err := helpers.GetGuild(args[0])
			helpers.Relax(err)

			if helpers.ConfirmEmbed(msg.ChannelID, msg.Author,
				fmt.Sprintf("Are you sure you want me to leave the server `%s` (`#%s`)?",
					targetGuild.Name, targetGuild.ID), "✅", "🚫") {
				helpers.SendMessage(msg.ChannelID, "Goodbye <a:ablobwave:393869340975300638>")
				err = session.GuildLeave(targetGuild.ID)
				helpers.Relax(err)
			}
		})
		return
	case "create-invite":
		helpers.RequireRobyulMod(msg, func() {
			session.ChannelTyping(msg.ChannelID)

			args := strings.Fields(content)
			if len(args) < 1 {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				return
			}
			guild, err := helpers.GetGuild(args[0])
			helpers.Relax(err)

			for _, channel := range guild.Channels {
				invite, err := session.ChannelInviteCreate(channel.ID, discordgo.Invite{
					MaxAge:    60 * 60,
					MaxUses:   0,
					Temporary: false,
				})
				if err != nil {
					_, err = helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Unable to create an invite in: `#%s (#%s)`",
						channel.Name, channel.ID))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				} else {
					_, err = helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Created invite: `https://discord.gg/%s` for: `#%s (#%s)`",
						invite.Code, channel.Name, channel.ID))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}
			}

			_, err = helpers.SendMessage(msg.ChannelID, "No channels left to try. <:blobugh:317047327443517442>")
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			return
		})
		return
	case "prefix":
		session.ChannelTyping(msg.ChannelID)
		args := strings.Fields(content)

		channel, err := helpers.GetChannel(msg.ChannelID)
		helpers.Relax(err)

		if len(args) > 0 {
			helpers.RequireAdmin(msg, func() {
				newPrefix := args[0]

				settings := helpers.GuildSettingsGetCached(channel.GuildID)

				oldPrefix := settings.Prefix

				settings.Prefix = newPrefix
				err = helpers.GuildSettingsSet(channel.GuildID, settings)
				helpers.Relax(err)

				_, err = helpers.EventlogLog(time.Now(), channel.GuildID, channel.GuildID,
					models.EventlogTargetTypeGuild, msg.Author.ID,
					models.EventlogTypeRobyulPrefixUpdate, "",
					[]models.ElasticEventlogChange{
						{
							Key:      "prefix",
							OldValue: oldPrefix,
							NewValue: settings.Prefix,
						},
					},
					nil, false)
				helpers.RelaxLog(err)

				_, err = helpers.SendMessage(msg.ChannelID,
					helpers.GetTextF(
						"plugins.mod.prefix-set-success",
						helpers.GetPrefixForServer(channel.GuildID),
					))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			})
			return
		}

		_, err = helpers.SendMessage(msg.ChannelID,
			helpers.GetTextF(
				"plugins.mod.prefix-info",
				helpers.GetPrefixForServer(channel.GuildID),
				helpers.GetPrefixForServer(channel.GuildID),
			))
		helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
		return
	case "toggle-chatlog":
		session.ChannelTyping(msg.ChannelID)
		helpers.RequireAdmin(msg, func() {
			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)

			settings := helpers.GuildSettingsGetCached(channel.GuildID)
			oldSetting := true
			if settings.ChatlogDisabled {
				oldSetting = false
			}
			var setMessage string
			if settings.ChatlogDisabled {
				settings.ChatlogDisabled = false
				setMessage = "Chatlog has been enabled."
			} else {
				settings.ChatlogDisabled = true
				setMessage = "Chatlog has been disabled and Server Statistics will be stored anonymized."
			}
			err = helpers.GuildSettingsSet(channel.GuildID, settings)
			helpers.Relax(err)

			newSetting := true
			if settings.ChatlogDisabled {
				newSetting = false
			}

			_, err = helpers.EventlogLog(time.Now(), channel.GuildID, channel.GuildID,
				models.EventlogTargetTypeGuild, msg.Author.ID,
				models.EventlogTypeRobyulChatlogUpdate, "",
				[]models.ElasticEventlogChange{
					{
						Key:      "chatlog_enabled",
						OldValue: helpers.StoreBoolAsString(oldSetting),
						NewValue: helpers.StoreBoolAsString(newSetting),
					},
				},
				nil, false)
			helpers.RelaxLog(err)

			_, err = helpers.SendMessage(msg.ChannelID, setMessage)
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			return
		})
		return
	case "batch-roles": // [p]batch-roles role a | role b | role c [| [after=role name] [color=hex code]]
		// todo: permission settings
		session.ChannelTyping(msg.ChannelID)
		helpers.RequireMod(msg, func() {
			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)

			serverRolesReceived, err := session.GuildRoles(channel.GuildID)
			if err != nil {
				if errD := err.(*discordgo.RESTError); errD != nil {
					if errD.Message.Code == discordgo.ErrCodeMissingPermissions {
						_, err = helpers.SendMessage(msg.ChannelID, "Please give me the `Manage Roles` permission to use this feature.")
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					} else {
						helpers.Relax(err)
					}
				} else {
					helpers.Relax(err)
				}
			}
			serverRoles := make([]*discordgo.Role, 0)
			for _, roleReceived := range serverRolesReceived {
				serverRoles = append(serverRoles, &discordgo.Role{ID: roleReceived.ID, Position: roleReceived.Position, Name: roleReceived.Name})
			}

			var colour int
			var afterRole *discordgo.Role

			var data map[string]string
			if strings.Contains(content, "=") {
				args := strings.Split(content, "|")
				if len(args) > 1 {
					data = helpers.ParseKeyValueString(strings.TrimSpace(args[len(args)-1]))
				}
			}
			if colourText, ok := data["color"]; ok {
				colour = helpers.GetDiscordColorFromHex(colourText)
			}
			if colourText, ok := data["colour"]; ok {
				colour = helpers.GetDiscordColorFromHex(colourText)
			}
			if afterText, ok := data["after"]; ok {
				for _, role := range serverRoles {
					if strings.ToLower(role.Name) == strings.ToLower(afterText) || role.ID == afterText {
						afterRole = role
					}
				}
				if afterRole == nil || afterRole.ID == "" {
					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					helpers.Relax(err)
					return
				}
			}

			rolesToCreate := strings.Split(content, "|")
			var rolesCreated int
			roleErrors := make([]error, 0)
			for _, roleToCreate := range rolesToCreate {
				if strings.Contains(roleToCreate, "=") {
					continue
				}

				roleToCreate = strings.TrimSpace(roleToCreate)

				if roleToCreate == "" {
					continue
				}

				newRole, err := session.GuildRoleCreate(channel.GuildID)
				if err != nil {
					roleErrors = append(roleErrors, err)
					continue
				}
				newRole, err = session.GuildRoleEdit(channel.GuildID, newRole.ID, roleToCreate, colour, false, 0, false)
				if err != nil {
					roleErrors = append(roleErrors, err)
					continue
				}
				if afterRole != nil && afterRole.ID != "" {
					newServerRoles := make([]*discordgo.Role, 0)
					sort.Slice(serverRoles, func(i, j int) bool { return serverRoles[i].Position < serverRoles[j].Position })
					var position int

					var roleAdded bool
					for _, serverRole := range serverRoles {
						if serverRole.ID == afterRole.ID {
							newServerRoles = append(newServerRoles, &discordgo.Role{ID: newRole.ID, Position: position, Name: newRole.Name})
							afterRole = newRole
							roleAdded = true
							position++
						}
						newServerRoles = append(newServerRoles, &discordgo.Role{ID: serverRole.ID, Position: position, Name: serverRole.Name})
						position++
					}
					if !roleAdded {
						newServerRoles = append(newServerRoles, &discordgo.Role{ID: newRole.ID, Position: position, Name: newRole.Name})
						position++
					}
					serverRoles = newServerRoles
				}
				rolesCreated++
			}

			resultText := fmt.Sprintf("Successfully created %d roles, failed to create %d roles", rolesCreated, len(roleErrors))

			if afterRole != nil && afterRole.ID != "" {
				_, err = session.GuildRoleReorder(channel.GuildID, serverRoles)
				if err != nil {
					resultText += ", failed to reorder roles"
				} else {
					resultText += ", reordered the roles successfully"
				}
			}

			options := []models.ElasticEventlogOption{
				{
					Key:   "batchroles_created",
					Value: strconv.Itoa(rolesCreated),
				},
				{
					Key:   "batchroles_errors",
					Value: strconv.Itoa(len(roleErrors)),
				},
			}
			if colour > 0 {
				options = append(options, models.ElasticEventlogOption{
					Key:   "batchroles_color",
					Value: helpers.GetHexFromDiscordColor(colour),
				})
			}
			if afterRole != nil {
				options = append(options, models.ElasticEventlogOption{
					Key:   "batchroles_afteroleid",
					Value: afterRole.ID,
					Type:  models.EventlogTargetTypeRole,
				})
			}

			_, err = helpers.EventlogLog(time.Now(), channel.GuildID, channel.GuildID,
				models.EventlogTargetTypeGuild, msg.Author.ID,
				models.EventlogTypeRobyulBatchRolesCreate, "",
				nil,
				options, false)
			helpers.RelaxLog(err)

			_, err = helpers.SendMessage(msg.ChannelID, resultText)
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			return
		})
		return
	case "set-bot-dp":
		helpers.RequireRobyulMod(msg, func() {
			session.ChannelTyping(msg.ChannelID)

			if len(msg.Attachments) <= 0 {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
				return
			}

			imageData := helpers.NetGet(msg.Attachments[0].URL)

			image, err := png.Decode(bytes.NewReader(imageData))
			if err != nil {
				if strings.Contains(err.Error(), "not a PNG file") {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.set-bot-dp-error-not-png"))
					return
				}
			}
			helpers.Relax(err)

			var buff bytes.Buffer

			png.Encode(&buff, image)

			encodedString := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buff.Bytes())

			_, err = session.UserUpdate(
				"",
				"",
				"",
				encodedString,
				"",
			)
			helpers.Relax(err)

			_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.set-bot-dp-success"))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			return
		})
		return
	case "pin": // [p]pin <channel> <message id>
		helpers.RequireMod(msg, func() {
			args := strings.Fields(content)

			sourceChannel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)

			var targetMessageID string
			var targetChannelID string
			if len(sourceChannel.Messages) >= 1 {
				var targetMessage *discordgo.Message
				for _, message := range sourceChannel.Messages {
					if message.ID == msg.ID {
						continue
					}
					if targetMessage == nil {
						targetMessage = message
						continue
					}
					targetTime, _ := targetMessage.Timestamp.Parse()
					nextTime, _ := message.Timestamp.Parse()
					if nextTime.After(targetTime) {
						targetMessage = message
					}
				}
				if targetMessage != nil {
					targetMessageID = targetMessage.ID
				}
				targetChannelID = sourceChannel.ID
			}
			if len(args) >= 2 {
				targetChannelID = args[0]
				targetMessageID = args[1]
			}

			if targetMessageID == "" || targetChannelID == "" {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
				return
			}

			targetChannel, err := helpers.GetChannelFromMention(msg, targetChannelID)
			if err != nil || targetChannel.ID == "" {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				return
			}
			if sourceChannel.GuildID != targetChannel.GuildID {
				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.echo-error-wrong-server"))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			}
			targetMessage, err := session.ChannelMessage(targetChannel.ID, targetMessageID)
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok {
					if errD.Message.Code == discordgo.ErrCodeUnknownMessage || errD.Message.Code == discordgo.ErrCodeMissingAccess {
						_, err = helpers.SendMessage(sourceChannel.ID, helpers.GetText("plugins.mod.edit-error-not-found"))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					}
				}
				helpers.Relax(err)
			}

			var alreadyPinned bool
			pinnedMessages, err := session.ChannelMessagesPinned(targetMessage.ChannelID)
			if err == nil {
				for _, pinnedMessage := range pinnedMessages {
					if pinnedMessage.ID == targetMessage.ID {
						alreadyPinned = true
					}
				}
			}

			var message string
			if !alreadyPinned {
				err = session.ChannelMessagePin(targetMessage.ChannelID, targetMessage.ID)
				if targetChannel.ID != sourceChannel.ID {
					message = helpers.GetText("plugins.mod.pin-success")
				}
			} else {
				err = session.ChannelMessageUnpin(targetMessage.ChannelID, targetMessage.ID)
				message = helpers.GetText("plugins.mod.unpin-success")
			}
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok {
					if errD.Message.Code == discordgo.ErrCodeMissingPermissions {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.pin-error-permissions"))
						return
					}
					if errD.Message.Code == discordgo.ErrCodeMaximumPinsReached {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.pin-error-limit"))
						return
					}
					if errD.Message.Code == discordgo.ErrCodeCannotExecuteActionOnSystemMessage {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.pin-error-system-message"))
						return
					}
				}
			}
			helpers.Relax(err)

			if message != "" {
				_, err = helpers.SendMessage(msg.ChannelID, message)
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			}
			return
		})
		return
	}
}
func (m *Mod) removeBanFromCache(user *discordgo.GuildBanRemove) bool {
	for _, botGuild := range cache.GetSession().State.Guilds {
		if botGuild.ID == user.GuildID {
			cacheCodec := cache.GetRedisCacheCodec()
			var err error
			var guildBans []*discordgo.GuildBan
			key := fmt.Sprintf("robyul2-discord:api:bans:%s", botGuild.ID)
			if err = cacheCodec.Get(key, &guildBans); err == nil {
				newGuildBans := make([]*discordgo.GuildBan, 0)
				for _, guildBan := range guildBans {
					if guildBan.User.ID != user.User.ID {
						newGuildBans = append(newGuildBans, guildBan)
					}
				}
				err = cacheCodec.Set(&redisCache.Item{
					Key:        key,
					Object:     &newGuildBans,
					Expiration: time.Hour * 24 * 30 * 365,
				})
				if err != nil {
					raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
				}
				return true
			}
		}
	}
	return false
}

func (m *Mod) addBanToCache(user *discordgo.GuildBanAdd) bool {
	for _, botGuild := range cache.GetSession().State.Guilds {
		if botGuild.ID == user.GuildID {
			cacheCodec := cache.GetRedisCacheCodec()
			var err error
			var guildBans []*discordgo.GuildBan
			key := fmt.Sprintf("robyul2-discord:api:bans:%s", botGuild.ID)
			if err = cacheCodec.Get(key, &guildBans); err == nil {
				guildBans = append(guildBans, &discordgo.GuildBan{Reason: "", User: user.User})
				err = cacheCodec.Set(&redisCache.Item{
					Key:        key,
					Object:     &guildBans,
					Expiration: time.Hour * 24 * 30 * 365,
				})
				if err != nil {
					raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
				}
				return true
			} else {
				guildBans = []*discordgo.GuildBan{{Reason: "", User: user.User}}
				err = cacheCodec.Set(&redisCache.Item{
					Key:        key,
					Object:     &guildBans,
					Expiration: time.Hour * 24 * 30 * 365,
				})
				if err != nil {
					raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
				}
				return true
			}
		}
	}
	return false
}

func (m *Mod) inspectUserBans(user *discordgo.User, sourceGuildID string) ([]*discordgo.Guild, []*discordgo.Guild) {
	bannedOnServerList := make([]*discordgo.Guild, 0)
	checkFailedServerList := make([]*discordgo.Guild, 0)

	cacheCodec := cache.GetRedisCacheCodec()
	var key string
	var guildBans []*discordgo.GuildBan
	var err error
	for _, botGuild := range cache.GetSession().State.Guilds {
		key = fmt.Sprintf("robyul2-discord:api:bans:%s", botGuild.ID)
		if err = cacheCodec.Get(key, &guildBans); err == nil {
			for _, guildBan := range guildBans {
				if guildBan.User.ID == user.ID {
					bannedOnServerList = append(bannedOnServerList, botGuild)
					cache.GetLogger().WithField("module", "mod").Info(fmt.Sprintf("user %s (%s) is banned on Guild %s (#%s)",
						user.Username, user.ID, botGuild.Name, botGuild.ID))
				}
			}
		} else {
			checkFailedServerList = append(checkFailedServerList, botGuild)
		}
	}
	return bannedOnServerList, checkFailedServerList
}

func (m *Mod) inspectCommonServers(user *discordgo.User) []*discordgo.Guild {
	isOnServerList := make([]*discordgo.Guild, 0)
	for _, botGuild := range cache.GetSession().State.Guilds {
		if helpers.GetIsInGuild(botGuild.ID, user.ID) {
			isOnServerList = append(isOnServerList, botGuild)
		}
	}
	return isOnServerList
}

func (m *Mod) getTroublemakerReports(user *discordgo.User) (entryBucket []models.TroublemakerlogEntry) {
	helpers.MDbIter(helpers.MdbCollection(models.TroublemakerlogTable).Find(bson.M{"userid": user.ID})).All(&entryBucket)
	return entryBucket
}

func (m *Mod) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {
	go func() {
		defer helpers.Recover()

		// get joined at date
		joinedAt, err := discordgo.Timestamp(member.JoinedAt).Parse()
		if err != nil {
			joinedAt = time.Now()
		}
		// Get invite link
		var usedInvite CacheInviteInformation
		var usedVanityInvite string
		if helpers.GetMemberPermissions(member.GuildID, cache.GetSession().State.User.ID)&discordgo.PermissionManageServer == discordgo.PermissionManageServer ||
			helpers.GetMemberPermissions(member.GuildID, cache.GetSession().State.User.ID)&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator {

			invites, err := session.GuildInvites(member.GuildID)
			if err != nil {
				helpers.RelaxLog(err)
			} else {
				newCacheInvites := make([]CacheInviteInformation, 0)
				for _, invite := range invites {
					createdAt, err := invite.CreatedAt.Parse()
					if err != nil {
						continue
					}
					invitedByID := ""
					if invite.Inviter != nil {
						invitedByID = invite.Inviter.ID
					}
					newCacheInvites = append(newCacheInvites, CacheInviteInformation{
						GuildID:         invite.Guild.ID,
						CreatedByUserID: invitedByID,
						Code:            invite.Code,
						CreatedAt:       createdAt,
						Uses:            invite.Uses,
					})
				}
				foundDiffsInInvites := make([]CacheInviteInformation, 0)
				if _, ok := invitesCache[member.GuildID]; ok {
					for _, newInvite := range newCacheInvites {
						seenInOldCache := false
						for _, oldInvite := range invitesCache[member.GuildID] {
							if oldInvite.Code == newInvite.Code {
								seenInOldCache = true
								if oldInvite.Uses != newInvite.Uses {
									foundDiffsInInvites = append(foundDiffsInInvites, newInvite)
								}
							}
						}
						if seenInOldCache == false && newInvite.Uses == 1 {
							foundDiffsInInvites = append(foundDiffsInInvites, newInvite)
						}
					}
				}
				invitesCache[member.GuildID] = newCacheInvites
				if len(foundDiffsInInvites) == 1 {
					usedInvite = foundDiffsInInvites[0]
				}
			}

			vanityInvite, _ := helpers.GetVanityUrlByGuildID(member.GuildID)

			if vanityInvite.VanityName != "" {
				discordInviteCode, _ := helpers.GetDiscordInviteByVanityInvite(vanityInvite)
				if discordInviteCode == usedInvite.Code {
					usedVanityInvite = vanityInvite.VanityName
				}
			}
		}

		go func() {
			defer helpers.Recover()

			joinedAt, err := discordgo.Timestamp(member.JoinedAt).Parse()
			if err != nil {
				joinedAt = time.Now()
			}
			// Save join in DB
			_, err = helpers.MDbInsertWithoutLogging(
				models.ModJoinlogTable,
				models.ModJoinlogEntry{
					GuildID:                   member.GuildID,
					UserID:                    member.User.ID,
					JoinedAt:                  joinedAt,
					InviteCodeUsed:            usedInvite.Code,
					InviteCodeCreatedByUserID: usedInvite.CreatedByUserID,
					InviteCodeCreatedAt:       usedInvite.CreatedAt,
					VanityInviteUsedName:      usedVanityInvite,
				},
			)
			helpers.RelaxLog(err)
			go func() {
				if member.User.ID == session.State.User.ID { // Don't inspect Robyul
					return
				}

				if helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserBannedOnOtherServers ||
					helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserNoCommonServers ||
					helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserNewlyCreatedAccount ||
					helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserReported ||
					helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserMultipleJoins ||
					helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserBannedDiscordlistNet ||
					helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserJoins {
					guild, err := helpers.GetGuild(member.GuildID)
					if err != nil {
						raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
						return
					}

					bannedOnServerList, checkFailedServerList := m.inspectUserBans(member.User, guild.ID)
					troublemakerReports := m.getTroublemakerReports(member.User)
					joins, _ := m.GetJoins(member.User.ID, member.GuildID)

					cache.GetLogger().WithField("module", "mod").Info(fmt.Sprintf("Inspected user %s (%s) because he joined Guild %s (#%s): Banned On: %d, Banned Checks Failed: %d, Reports: %d, Joins: %d",
						member.User.Username, member.User.ID, guild.Name, guild.ID, len(bannedOnServerList), len(checkFailedServerList), len(troublemakerReports), len(joins)))

					isOnServerList := m.inspectCommonServers(member.User)

					joinedTime := helpers.GetTimeFromSnowflake(member.User.ID)
					oneDayAgo := time.Now().AddDate(0, 0, -1)
					oneWeekAgo := time.Now().AddDate(0, 0, -7)

					isBannedOnBansdiscordlistNet, err := helpers.IsBannedOnBansdiscordlistNet(member.User.ID)
					helpers.RelaxLog(err)

					if !((helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserBannedOnOtherServers && len(bannedOnServerList) > 0) ||
						(helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserNoCommonServers && (len(isOnServerList)-1) <= 0) ||
						(helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserNewlyCreatedAccount && joinedTime.After(oneWeekAgo)) ||
						(helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserReported && len(troublemakerReports) > 0) ||
						(helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserMultipleJoins && len(joins) > 1) ||
						(helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserBannedDiscordlistNet && isBannedOnBansdiscordlistNet) ||
						(helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserJoins)) {
						return
					}

					resultEmbed := &discordgo.MessageEmbed{
						Title: helpers.GetTextF("plugins.mod.inspect-embed-title", member.User.Username, member.User.Discriminator),
						Description: helpers.GetTextF("plugins.mod.inspect-description-done", member.User.ID) +
							"\n_inspected because User joined this Server._",
						URL:       helpers.GetAvatarUrl(member.User),
						Thumbnail: &discordgo.MessageEmbedThumbnail{URL: helpers.GetAvatarUrl(member.User)},
						Footer:    &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.mod.inspect-embed-footer", member.User.ID, len(session.State.Guilds))},
						Color:     0x0FADED,
					}

					resultBansText := ""
					if len(bannedOnServerList) <= 0 {
						resultBansText += fmt.Sprintf(":white_check_mark: User is banned on none servers.\n:black_medium_small_square:Checked %d servers.", len(session.State.Guilds)-len(checkFailedServerList))
					} else {
						resultBansText += fmt.Sprintf(":warning: User is banned on **%d** server(s).\n:black_medium_small_square:Checked %d servers.", len(bannedOnServerList), len(session.State.Guilds)-len(checkFailedServerList))
					}

					commonGuildsText := ""
					if len(isOnServerList)-1 > 0 { // -1 to exclude the server the user is currently on
						commonGuildsText += fmt.Sprintf(":white_check_mark: User is on **%d** other server(s) with Robyul.", len(isOnServerList)-1)
					} else {
						commonGuildsText += ":question: User is on **none** other servers with Robyul."
					}
					joinedTimeText := ""
					if !joinedTime.After(oneWeekAgo) {
						joinedTimeText += fmt.Sprintf(":white_check_mark: User Account got created %s.\n:black_medium_small_square:Joined at %s.", helpers.SinceInDaysText(joinedTime), joinedTime.Format(time.ANSIC))
					} else if !joinedTime.After(oneDayAgo) {
						joinedTimeText += fmt.Sprintf(":question: User Account is less than one Week old.\n:black_medium_small_square:Joined at %s.", joinedTime.Format(time.ANSIC))
					} else {
						joinedTimeText += fmt.Sprintf(":warning: User Account is less than one Day old.\n:black_medium_small_square:Joined at %s.", joinedTime.Format(time.ANSIC))
					}

					var troublemakerReportsText string
					if len(troublemakerReports) <= 0 {
						troublemakerReportsText = ":white_check_mark: User never got reported"
					} else {
						troublemakerReportsText = fmt.Sprintf(":warning: User got reported %d time(s)\nUse `_troublemaker list %s` to view the details.", len(troublemakerReports), member.User.ID)
					}

					joinsText := ""
					if len(joins) == 0 {
						joinsText = ":white_check_mark: User never joined this server\n"
					} else if len(joins) == 1 {
						if joins[0].InviteCodeUsed != "" {
							createdByUser, _ := helpers.GetUser(joins[0].InviteCodeCreatedByUserID)
							if createdByUser == nil {
								createdByUser = new(discordgo.User)
								createdByUser.ID = joins[0].InviteCodeCreatedByUserID
								createdByUser.Username = "N/A"
							}

							var labelText string
							if joins[0].VanityInviteUsedName != "" {
								labelText = " (`" + helpers.GetConfig().Path("website.vanityurl_domain").Data().(string) + "/" + joins[0].VanityInviteUsedName + "`)"
							}

							joinsText = fmt.Sprintf(":white_check_mark: User joined this server once with the invite `%s`%s created by `%s (#%s)` %s\n",
								joins[0].InviteCodeUsed, labelText, createdByUser.Username,
								createdByUser.ID, humanize.Time(joins[0].InviteCodeCreatedAt))
						} else {
							joinsText = fmt.Sprintf(":white_check_mark: User joined this server once\nGive Robyul the `Manage Server` permission to see using which invite.\n")
						}
					} else if len(joins) > 1 {
						sort.Slice(joins, func(i, j int) bool { return joins[i].JoinedAt.After(joins[j].JoinedAt) })
						lastJoin := joins[0]

						if lastJoin.InviteCodeUsed != "" {
							createdByUser, _ := helpers.GetUser(lastJoin.InviteCodeCreatedByUserID)
							if createdByUser == nil {
								createdByUser = new(discordgo.User)
								createdByUser.ID = lastJoin.InviteCodeCreatedByUserID
								createdByUser.Username = "N/A"
							}

							var labelText string
							if lastJoin.VanityInviteUsedName != "" {
								labelText = " (`" + helpers.GetConfig().Path("website.vanityurl_domain").Data().(string) + "/" + joins[0].VanityInviteUsedName + "`)"
							}

							joinsText = fmt.Sprintf(":warning: User joined this server %d times\nLast time with the invite `%s`%s created by `%s (#%s)` %s\n",
								len(joins), lastJoin.InviteCodeUsed,
								labelText, createdByUser.Username, createdByUser.ID, humanize.Time(lastJoin.InviteCodeCreatedAt))
						} else {
							joinsText = fmt.Sprintf(":warning: User joined this server %d times (last time %s)\n"+
								"Give Robyul the `Manage Server` permission to see using which invites.\n",
								len(joins), humanize.Time(lastJoin.JoinedAt))
						}
					}

					isBannedOnBansdiscordlistNetText := ":white_check_mark: User is not banned.\n"
					if isBannedOnBansdiscordlistNet {
						isBannedOnBansdiscordlistNetText = ":warning: User is banned on [bans.discordlist.net](https://bans.discordlist.net/).\n"
					}

					resultEmbed.Fields = []*discordgo.MessageEmbedField{
						{Name: "Bans", Value: resultBansText, Inline: false},
						{Name: "Troublemaker Reports", Value: troublemakerReportsText, Inline: false},
						{Name: "bans.discordlist.net", Value: isBannedOnBansdiscordlistNetText, Inline: false},
						{Name: "Join History", Value: joinsText, Inline: false},
						{Name: "Common Servers", Value: commonGuildsText, Inline: false},
						{Name: "Account Age", Value: joinedTimeText, Inline: false},
					}

					for _, failedServer := range checkFailedServerList {
						if failedServer.ID == member.GuildID {
							resultEmbed.Description += "\n:warning: I wasn't able to gather the ban list for this server!\nPlease give Robyul the permission `Ban Members` to help other servers."
							break
						}
					}

					_, err = helpers.SendEmbed(helpers.GuildSettingsGetCached(member.GuildID).InspectsChannel, resultEmbed)
					if err != nil {
						cache.GetLogger().WithField("module", "mod").Warnf("Failed to send guild join inspect to channel #%s on guild #%s: %s",
							helpers.GuildSettingsGetCached(member.GuildID).InspectsChannel, member.GuildID, err.Error())
						if errD, ok := err.(*discordgo.RESTError); ok {
							if errD.Message.Code != discordgo.ErrCodeMissingAccess && errD.Message.Code != discordgo.ErrCodeMissingPermissions {
								raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
							}
						} else {
							raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
						}
						return
					}
				}
			}()
			go func() {
				defer helpers.Recover()

				if !cache.HasElastic() {
					return
				}

				err := helpers.ElasticAddJoin(member, usedInvite.Code, usedVanityInvite)
				helpers.Relax(err)
			}()
		}()
		go func() {
			defer helpers.Recover()

			settings := helpers.GuildSettingsGetCached(member.GuildID)

			for _, mutedMember := range settings.MutedMembers {
				if mutedMember == member.User.ID {
					muteRole, err := helpers.GetMuteRole(member.GuildID)
					if err != nil {
						raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
						return
					}
					err = session.GuildMemberRoleAdd(member.GuildID, member.User.ID, muteRole.ID)
					if err != nil {
						raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
						return
					}
				}
			}
		}()
		go func() {
			defer helpers.Recover()

			options := make([]models.ElasticEventlogOption, 0)

			if usedInvite.Code != "" {
				options = append(options, models.ElasticEventlogOption{
					Key:   "used_invite_code",
					Value: usedInvite.Code,
				})
			}
			if usedVanityInvite != "" {
				options = append(options, models.ElasticEventlogOption{
					Key:   "used_vanity_invite_name",
					Value: usedVanityInvite,
				})
			}

			_, err := helpers.EventlogLog(joinedAt, member.GuildID, member.User.ID, models.EventlogTargetTypeUser, "", models.EventlogTypeMemberJoin, "", nil, options, false)
			helpers.RelaxLog(err)
		}()
	}()
}

func (m *Mod) GetJoins(userID string, guildID string) (joins []models.ModJoinlogEntry, err error) {
	err = helpers.MDbIter(helpers.MdbCollection(models.ModJoinlogTable).Find(
		bson.M{"userid": userID, "guildid": guildID}).Sort("-joinedat")).All(&joins)
	return joins, err
}

func (m *Mod) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {
}

func (m *Mod) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {
}

func (m *Mod) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {
}

func (m *Mod) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {

}
func (m *Mod) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {
	if helpers.GetMemberPermissions(user.GuildID, cache.GetSession().State.User.ID)&discordgo.PermissionBanMembers != discordgo.PermissionBanMembers &&
		helpers.GetMemberPermissions(user.GuildID, cache.GetSession().State.User.ID)&discordgo.PermissionAdministrator != discordgo.PermissionAdministrator {
		return
	}

	go func() {
		bannedOnGuild, err := helpers.GetGuild(user.GuildID)
		if err != nil {
			raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
			return
		}

		updated := m.addBanToCache(user)
		if !updated {
			return
		}
		for _, targetGuild := range cache.GetSession().State.Guilds {
			if targetGuild.ID != user.GuildID && helpers.GuildSettingsGetCached(targetGuild.ID).InspectTriggersEnabled.UserBannedOnOtherServers {
				if user.User.ID == session.State.User.ID { // Don't inspect Robyul
					return
				}

				// check if user is on this guild
				if helpers.GetIsInGuild(targetGuild.ID, user.User.ID) {
					guild, err := helpers.GetGuild(targetGuild.ID)
					if err != nil {
						raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
						continue
					}

					bannedOnServerList, checkFailedServerList := m.inspectUserBans(user.User, guild.ID)

					cache.GetLogger().WithField("module", "mod").Info(fmt.Sprintf("Inspected user %s (%s) because he got banned on Guild %s (#%s) for Guild %s (#%s): Banned On: %d, Banned Checks Failed: %d",
						user.User.Username, user.User.ID, bannedOnGuild.Name, bannedOnGuild.ID, guild.Name, guild.ID, len(bannedOnServerList), len(checkFailedServerList)))

					resultEmbed := &discordgo.MessageEmbed{
						Title: helpers.GetTextF("plugins.mod.inspect-embed-title", user.User.Username, user.User.Discriminator),
						Description: helpers.GetTextF("plugins.mod.inspect-description-done", user.User.ID) +
							"\n_inspected because User got banned on a different Server._",
						URL:       helpers.GetAvatarUrl(user.User),
						Thumbnail: &discordgo.MessageEmbedThumbnail{URL: helpers.GetAvatarUrl(user.User)},
						Footer:    &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.mod.inspect-embed-footer", user.User.ID, len(session.State.Guilds))},
						Color:     0x0FADED,
					}

					resultBansText := ""
					if len(bannedOnServerList) <= 0 {
						resultBansText += fmt.Sprintf(":white_check_mark: User is banned on none servers.\n:black_medium_small_square:Checked %d servers.", len(session.State.Guilds)-len(checkFailedServerList))
					} else {
						resultBansText += fmt.Sprintf(":warning: User is banned on **%d** server(s).\n:black_medium_small_square:Checked %d servers.", len(bannedOnServerList), len(session.State.Guilds)-len(checkFailedServerList))
					}

					isOnServerList := m.inspectCommonServers(user.User)
					commonGuildsText := ""
					if len(isOnServerList) > 0 { // -1 to exclude the server the user is currently on
						commonGuildsText += fmt.Sprintf(":white_check_mark: User is on **%d** server(s) with Robyul.", len(isOnServerList)-1)
					} else {
						commonGuildsText += ":question: User is on **none** servers with Robyul."
					}

					joinedTime := helpers.GetTimeFromSnowflake(user.User.ID)
					oneDayAgo := time.Now().AddDate(0, 0, -1)
					oneWeekAgo := time.Now().AddDate(0, 0, -7)
					joinedTimeText := ""
					if !joinedTime.After(oneWeekAgo) {
						joinedTimeText += fmt.Sprintf(":white_check_mark: User Account got created %s.\n:black_medium_small_square:Joined at %s.", helpers.SinceInDaysText(joinedTime), joinedTime.Format(time.ANSIC))
					} else if !joinedTime.After(oneDayAgo) {
						joinedTimeText += fmt.Sprintf(":question: User Account is less than one Week old.\n:black_medium_small_square:Joined at %s.", joinedTime.Format(time.ANSIC))
					} else {
						joinedTimeText += fmt.Sprintf(":warning: User Account is less than one Day old.\n:black_medium_small_square:Joined at %s.", joinedTime.Format(time.ANSIC))
					}

					troublemakerReports := m.getTroublemakerReports(user.User)
					var troublemakerReportsText string
					if len(troublemakerReports) <= 0 {
						troublemakerReportsText = ":white_check_mark: User never got reported"
					} else {
						troublemakerReportsText = fmt.Sprintf(":warning: User got reported %d time(s)\nUse `_troublemaker list %s` to view the details.", len(troublemakerReports), user.User.ID)
					}

					joins, _ := m.GetJoins(user.User.ID, targetGuild.ID)
					joinsText := ""
					if len(joins) == 0 {
						joinsText = ":white_check_mark: User never joined this server\n"
					} else if len(joins) == 1 {
						if joins[0].InviteCodeUsed != "" {
							createdByUser, _ := helpers.GetUser(joins[0].InviteCodeCreatedByUserID)
							if createdByUser == nil {
								createdByUser = new(discordgo.User)
								createdByUser.ID = joins[0].InviteCodeCreatedByUserID
								createdByUser.Username = "N/A"
							}

							var labelText string
							if joins[0].VanityInviteUsedName != "" {
								labelText = " (`" + helpers.GetConfig().Path("website.vanityurl_domain").Data().(string) + "/" + joins[0].VanityInviteUsedName + "`)"
							}

							joinsText = fmt.Sprintf(":white_check_mark: User joined this server once (%s) with the invite `%s`%s created by `%s (#%s)` %s\n",
								humanize.Time(joins[0].JoinedAt), joins[0].InviteCodeUsed, labelText, createdByUser.Username,
								createdByUser.ID, humanize.Time(joins[0].InviteCodeCreatedAt))
						} else {
							joinsText = fmt.Sprintf(":white_check_mark: User joined this server once (%s)\nGive Robyul the `Manage Server` permission to see using which invite.\n",
								humanize.Time(joins[0].JoinedAt))
						}
					} else if len(joins) > 1 {
						sort.Slice(joins, func(i, j int) bool { return joins[i].JoinedAt.After(joins[j].JoinedAt) })
						lastJoin := joins[0]

						if lastJoin.InviteCodeUsed != "" {
							createdByUser, _ := helpers.GetUser(lastJoin.InviteCodeCreatedByUserID)
							if createdByUser == nil {
								createdByUser = new(discordgo.User)
								createdByUser.ID = lastJoin.InviteCodeCreatedByUserID
								createdByUser.Username = "N/A"
							}

							var labelText string
							if lastJoin.VanityInviteUsedName != "" {
								labelText = " (`" + helpers.GetConfig().Path("website.vanityurl_domain").Data().(string) + "/" + joins[0].VanityInviteUsedName + "`)"
							}

							joinsText = fmt.Sprintf(":warning: User joined this server %d times (last time %s)\n"+
								"Last time with the invite `%s`%s created by `%s (#%s)` %s\n",
								len(joins), humanize.Time(lastJoin.JoinedAt), lastJoin.InviteCodeUsed,
								labelText, createdByUser.Username, createdByUser.ID, humanize.Time(lastJoin.InviteCodeCreatedAt))
						} else {
							joinsText = fmt.Sprintf(":warning: User joined this server %d times (last time %s)\n"+
								"Give Robyul the `Manage Server` permission to see using which invites.\n",
								len(joins), humanize.Time(lastJoin.JoinedAt))
						}
					}

					isBannedOnBansdiscordlistNet, err := helpers.IsBannedOnBansdiscordlistNet(user.User.ID)
					helpers.RelaxLog(err)
					isBannedOnBansdiscordlistNetText := ":white_check_mark: User is not banned.\n"
					if isBannedOnBansdiscordlistNet {
						isBannedOnBansdiscordlistNetText = ":warning: User is banned on [bans.discordlist.net](https://bans.discordlist.net/).\n"
					}

					resultEmbed.Fields = []*discordgo.MessageEmbedField{
						{Name: "Bans", Value: resultBansText, Inline: false},
						{Name: "Troublemaker Reports", Value: troublemakerReportsText, Inline: false},
						{Name: "bans.discordlist.net", Value: isBannedOnBansdiscordlistNetText, Inline: false},
						{Name: "Join History", Value: joinsText, Inline: false},
						{Name: "Common Servers", Value: commonGuildsText, Inline: false},
						{Name: "Account Age", Value: joinedTimeText, Inline: false},
					}

					for _, failedServer := range checkFailedServerList {
						if failedServer.ID == targetGuild.ID {
							resultEmbed.Description += "\n:warning: I wasn't able to gather the ban list for this server!\nPlease give Robyul the permission `Ban Members` to help other servers."
							break
						}
					}

					targetChannel, err := helpers.GetChannelWithoutApi(helpers.GuildSettingsGetCached(targetGuild.ID).InspectsChannel)
					if err == nil {
						_, err = helpers.SendEmbed(targetChannel.ID, resultEmbed)
						if err != nil {
							cache.GetLogger().WithField("module", "mod").Warnf("Failed to send guild ban inspect to channel #%s on guild #%s: %s",
								helpers.GuildSettingsGetCached(targetGuild.ID).InspectsChannel, targetGuild.ID, err.Error())
							if errD, ok := err.(*discordgo.RESTError); ok {
								if errD.Message.Code != discordgo.ErrCodeMissingAccess {
									helpers.RelaxLog(err)
								}
							} else {
								raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
							}
							continue
						}
					}
				}
			}
		}
	}()
}

func (m *Mod) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {
	m.removeBanFromCache(user)
}
func (m *Mod) OnMessageDelete(msg *discordgo.MessageDelete, session *discordgo.Session) {

}

func (m *Mod) Min(a, b int) int {
	if a <= b {
		return a
	}
	return b
}

// https://www.dotnetperls.com/duplicates-go
func (m *Mod) removeDuplicates(elements []string) []string {
	encountered := map[string]bool{}

	// Create a map of all unique elements.
	for v := range elements {
		encountered[elements[v]] = true
	}

	// Place all keys from the map into a slice.
	var result []string
	for key := range encountered {
		result = append(result, key)
	}
	return result
}
