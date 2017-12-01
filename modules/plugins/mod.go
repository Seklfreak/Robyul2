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
	"github.com/bradfitz/slice"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/getsentry/raven-go"
	redisCache "github.com/go-redis/cache"
	rethink "github.com/gorethink/gorethink"
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
	}
}

type DB_Mod_JoinLog struct {
	ID                        string    `gorethink:"id,omitempty"`
	GuildID                   string    `gorethink:"guildid"`
	UserID                    string    `gorethink:"userid"`
	JoinedAt                  time.Time `gorethink:"joinedat"`
	InviteCodeUsed            string    `gorethink:"invitecode"`
	InviteCodeCreatedByUserID string    `gorethink:"invitecode_createdbyuserid"`
	InviteCodeCreatedAt       time.Time `gorethink:"invitecode_createdat"`
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
		log := cache.GetLogger()

		for _, guild := range session.State.Guilds {
			invites, err := session.GuildInvites(guild.ID)
			if err != nil {
				//log.WithField("module", "mod").Error(fmt.Sprintf("error getting invites from guild %s (#%s): %s",
				//	guild.Name, guild.ID, err.Error()))
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
		log.WithField("module", "mod").Info(fmt.Sprintf("got invite link cache of %d servers", len(invitesCache)))
	}()
	go m.cacheBans()
	cache.GetLogger().WithField("module", "mod").Info("Started cacheBans")
}

func (m *Mod) Uninit(session *discordgo.Session) {

}

func (m *Mod) cacheBans() {
	defer helpers.Recover()

	var key string
	var guildBansCached int
	cacheCodec := cache.GetRedisCacheCodec()
	cache.GetLogger().WithField("module", "mod").Debug("started bans caching for redis")
	guildBansCached = 0
	for _, botGuild := range cache.GetSession().State.Guilds {
		key = fmt.Sprintf("robyul2-discord:api:bans:%s", botGuild.ID)
		guildBans, err := cache.GetSession().GuildBans(botGuild.ID)
		if err != nil {
			if errD, ok := err.(*discordgo.RESTError); ok {
				if errD.Message.Code != 50013 {
					raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
				}
			} else {
				raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
			}
		} else {
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
	}
	cache.GetLogger().WithField("module", "mod").Debug(fmt.Sprintf("cached bans for %d guilds in redis", guildBansCached))
}

func (m *Mod) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
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
							}
						} else {
							if helpers.ConfirmEmbed(msg.ChannelID, msg.Author, helpers.GetTextF("plugins.mod.deleting-message-bulkdelete-confirm", len(messagesToDeleteIds)), "✅", "🚫") == true {
								for i := 0; i < len(messagesToDeleteIds); i += 100 {
									batch := messagesToDeleteIds[i:m.Min(i+100, len(messagesToDeleteIds))]
									err := session.ChannelMessagesBulkDelete(msg.ChannelID, batch)
									cache.GetLogger().WithField("module", "mod").Info(fmt.Sprintf("Deleted %d messages (command issued by %s (#%s))", len(batch), msg.Author.Username, msg.Author.ID))
									if err != nil {
										if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == 50034 {
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
				muteRole, err := helpers.GetMuteRole(channel.GuildID)
				if err != nil {
					if err, ok := err.(*discordgo.RESTError); ok && err.Message.Code == 50013 {
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.mod.get-mute-role-no-permissions"))
						return
					} else {
						helpers.Relax(err)
					}
				}
				if helpers.GetIsInGuild(channel.GuildID, targetUser.ID) {
					err = session.GuildMemberRoleAdd(channel.GuildID, targetUser.ID, muteRole.ID)
					if err != nil {
						if errD, ok := err.(discordgo.RESTError); ok {
							if errD.Message.Code == 10007 {
								_, err = helpers.SendMessage(msg.ChannelID, "I wasn't able to assign the mute role to the given user.")
								helpers.Relax(err)
								return
							} else {
								helpers.Relax(err)
							}
						} else {
							helpers.Relax(err)
						}
					}
				}

				successText := helpers.GetTextF("plugins.mod.user-muted-success", targetUser.Username, targetUser.ID)

				if time.Now().Before(timeToUnmuteAt) {
					signature := helpers.UnmuteUserSignature(channel.GuildID, targetUser.ID)
					signature.ETA = &timeToUnmuteAt

					_, err = cache.GetMachineryServer().SendTask(signature)
					helpers.Relax(err)

					successText = helpers.GetTextF("plugins.mod.user-muted-success-timed", targetUser.Username, targetUser.ID, timeToUnmuteAt.Format(time.ANSIC)+" UTC")
				}

				_, err = helpers.SendMessage(msg.ChannelID, successText)
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			} else {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
				return
			}
		})
		return
	case "unmute": // [p]unmute server <User>
		helpers.RequireMod(msg, func() {
			session.ChannelTyping(msg.ChannelID)
			args := strings.Fields(content)
			if len(args) >= 1 {
				targetUser, _ := helpers.GetUserFromMention(args[0])
				if targetUser == nil {
					targetUser = new(discordgo.User)
					targetUser.ID = args[1]
					targetUser.Username = "N/A"
				}
				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)

				err = helpers.UnmuteUser(channel.GuildID, targetUser.ID)
				helpers.Relax(err)

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
				_, err = helpers.SendMessage(targetChannel.ID, newText)
				if err != nil {
					if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeMissingAccess {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.echo-error-no-access"))
						return
					}
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
				helpers.EditMessage(targetChannel.ID, targetMessage.ID, newText)
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
				session.ChannelFileSend(targetChannel.ID, msg.Attachments[0].Filename, bytes.NewReader(fileToUpload))
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
				newMessage := fmt.Sprintf("```%s```", targetMessage.Content)
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

		if isMod == false && isAllowedToInspectExtended == false {
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
				if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == 10013 {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.mod.user-not-found"))
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
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
			resultBansText += fmt.Sprintf("✅ User is banned on none servers.\n◾Checked %d servers.\n", len(session.State.Guilds)-len(checkFailedServerList))
		} else {
			if isExtendedInspect == false {
				resultBansText += fmt.Sprintf("⚠ User is banned on **%d** servers.\n◾Checked %d servers.\n", len(bannedOnServerList), len(session.State.Guilds)-len(checkFailedServerList))
			} else {
				resultBansText += fmt.Sprintf("⚠ User is banned on **%d** servers:\n", len(bannedOnServerList))
				i := 0
			BannedOnLoop:
				for _, bannedOnServer := range bannedOnServerList {
					resultBansText += fmt.Sprintf("▪`%s` (#%s)\n", bannedOnServer.Name, bannedOnServer.ID)
					i++
					if i >= 4 && textVersion == false {
						resultBansText += fmt.Sprintf("▪ and %d other server(s)\n", len(bannedOnServerList)-(i+1))
						break BannedOnLoop
					}
				}
				resultBansText += fmt.Sprintf("◾Checked %d servers.\n", len(session.State.Guilds)-len(checkFailedServerList))
			}
		}

		isOnServerList := m.inspectCommonServers(targetUser)
		commonGuildsText := ""
		if len(isOnServerList) > 0 { // -1 to exclude the server the user is currently on
			if isExtendedInspect == false {
				commonGuildsText += fmt.Sprintf("✅ User is on **%d** server(s) with Robyul.\n", len(isOnServerList))
			} else {
				commonGuildsText += fmt.Sprintf("✅ User is on **%d** server(s) with Robyul:\n", len(isOnServerList))
				i := 0
			ServerListLoop:
				for _, isOnServer := range isOnServerList {
					commonGuildsText += fmt.Sprintf("▪`%s` (#%s)\n", isOnServer.Name, isOnServer.ID)
					i++
					if i >= 4 && textVersion == false {
						commonGuildsText += fmt.Sprintf("▪ and %d other server(s)\n", len(isOnServerList)-(i))
						break ServerListLoop
					}
				}
			}
		} else {
			commonGuildsText += "❓ User is on **none** other servers with Robyul.\n"
		}

		joinedTime := helpers.GetTimeFromSnowflake(targetUser.ID)
		oneDayAgo := time.Now().AddDate(0, 0, -1)
		oneWeekAgo := time.Now().AddDate(0, 0, -7)
		joinedTimeText := ""
		if !joinedTime.After(oneWeekAgo) {
			joinedTimeText += fmt.Sprintf("✅ User Account got created %s.\n◾Joined at %s.\n", helpers.SinceInDaysText(joinedTime), joinedTime.Format(time.ANSIC))
		} else if !joinedTime.After(oneDayAgo) {
			joinedTimeText += fmt.Sprintf("❓ User Account is less than one Week old.\n◾Joined at %s.\n", joinedTime.Format(time.ANSIC))
		} else {
			joinedTimeText += fmt.Sprintf("⚠ User Account is less than one Day old.\n◾Joined at %s.\n", joinedTime.Format(time.ANSIC))
		}

		troublemakerReports := m.getTroublemakerReports(targetUser)
		var troublemakerReportsText string
		if len(troublemakerReports) <= 0 {
			troublemakerReportsText = "✅ User never got reported\n"
		} else {
			troublemakerReportsText = fmt.Sprintf("⚠ User got reported %d time(s)\nUse `_troublemaker list %s` to view the details.\n", len(troublemakerReports), targetUser.ID)
		}

		joins, _ := m.GetJoins(targetUser.ID, channel.GuildID)
		joinsText := ""
		if len(joins) == 0 {
			joinsText = "✅ User never joined this server\n"
		} else if len(joins) == 1 {
			if joins[0].InviteCodeUsed != "" {
				createdByUser, _ := helpers.GetUser(joins[0].InviteCodeCreatedByUserID)
				if createdByUser == nil {
					createdByUser = new(discordgo.User)
					createdByUser.ID = joins[0].InviteCodeCreatedByUserID
					createdByUser.Username = "N/A"
				}

				joinsText = fmt.Sprintf("✅ User joined this server once with the invite `%s` created by `%s (#%s)` %s\n",
					joins[0].InviteCodeUsed, createdByUser.Username, createdByUser.ID, humanize.Time(joins[0].InviteCodeCreatedAt))
			} else {
				joinsText = "✅ User joined this server once\nGive Robyul the `Manage Server` permission to see using which invite.\n"
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

				joinsText = fmt.Sprintf("⚠ User joined this server %d times\nLast time with the invite `%s` created by `%s (#%s)` %s\n",
					len(joins),
					lastJoin.InviteCodeUsed, createdByUser.Username, createdByUser.ID, humanize.Time(lastJoin.InviteCodeCreatedAt))
			} else {
				joinsText = fmt.Sprintf("⚠ User joined this server %d times\nGive Robyul the `Manage Server` permission to see using which invites.\n", len(joins))
			}
		}

		isBannedOnBansdiscordlistNet, err := helpers.IsBannedOnBansdiscordlistNet(targetUser.ID)
		helpers.RelaxLog(err)
		isBannedOnBansdiscordlistNetText := "✅ User is not banned.\n"
		isBannedOnBansdiscordlistNetTextText := "✅ User is not banned on <https://bans.discordlist.net/>.\n"
		if isBannedOnBansdiscordlistNet {
			isBannedOnBansdiscordlistNetText = "⚠ User is banned on [bans.discordlist.net](https://bans.discordlist.net/).\n"
			isBannedOnBansdiscordlistNetTextText = "⚠ User is banned on <https://bans.discordlist.net/>.\n"
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
				noAccessToBansText := "\n⚠ I wasn't able to gather the ban list for this server!\nPlease give Robyul the permission `Ban Members` to help other servers.\n"
				resultEmbed.Description += noAccessToBansText
				resultText += noAccessToBansText
				break
			}
		}

		if textVersion == false {
			_, err = helpers.EditEmbed(msg.ChannelID, resultMessage.ID, resultEmbed)
			helpers.Relax(err)
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
					"💾"}
				for _, allowedEmote := range allowedEmotes {
					err = session.MessageReactionAdd(msg.ChannelID, chooseMessage.ID, allowedEmote)
					helpers.Relax(err)
				}

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

					if needEmbedUpdate == true {
						chooseEmbed.Description = fmt.Sprintf(
							"Choose which warnings should trigger an automatic inspect post in <#%s>.\n"+
								"**Available Triggers**\n",
							targetChannel.ID)
						enabledEmote := "🔲"
						if settings.InspectTriggersEnabled.UserBannedOnOtherServers {
							enabledEmote = "✔"
						}
						chooseEmbed.Description += fmt.Sprintf("%s %s User is banned on a different server with Robyul on. Gets checked everytime an user joins or gets banned on a different server with Robyul on.\n",
							emojis.From("1"), enabledEmote)

						enabledEmote = "🔲"
						if settings.InspectTriggersEnabled.UserNoCommonServers {
							enabledEmote = "✔"
						}
						chooseEmbed.Description += fmt.Sprintf("%s %s User has none other common servers with Robyul. Gets checked everytime an user joins.\n",
							emojis.From("2"), enabledEmote)

						enabledEmote = "🔲"
						if settings.InspectTriggersEnabled.UserNewlyCreatedAccount {
							enabledEmote = "✔"
						}
						chooseEmbed.Description += fmt.Sprintf("%s %s Account is less than one week old. Gets checked everytime an user joins.\n",
							emojis.From("3"), enabledEmote)

						enabledEmote = "🔲"
						if settings.InspectTriggersEnabled.UserReported {
							enabledEmote = "✔"
						}
						chooseEmbed.Description += fmt.Sprintf("%s %s Account got reported as a troublemaker. Gets checked everytime an user joins.\n",
							emojis.From("4"), enabledEmote)

						enabledEmote = "🔲"
						if settings.InspectTriggersEnabled.UserMultipleJoins {
							enabledEmote = "✔"
						}
						chooseEmbed.Description += fmt.Sprintf("%s %s Account joined this server more than once. Gets checked everytime an user joins.\n",
							emojis.From("5"), enabledEmote)

						enabledEmote = "🔲"
						if settings.InspectTriggersEnabled.UserBannedDiscordlistNet {
							enabledEmote = "✔"
						}
						chooseEmbed.Description += fmt.Sprintf("%s %s Account is banned on bans.discordlist.net, a global (unofficial) discord ban list.\n",
							emojis.From("6"), enabledEmote)

						if emotesLocked == true {
							chooseEmbed.Description += fmt.Sprintf("⚠ Please give Robyul the `Manage Messages` permission to be able to disable triggers or disable all triggers using `%sauto-inspects-channel`.\n",
								helpers.GetPrefixForServer(channel.GuildID),
							)
						}
						chooseEmbed.Description += "Use 💾 to save and exit."
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

				chooseEmbed.Description = strings.Replace(chooseEmbed.Description, "Use 💾 to save and exit.", "Saved.", -1)
				helpers.EditEmbed(msg.ChannelID, chooseMessage.ID, chooseEmbed)

				successMessage = helpers.GetText("plugins.mod.inspects-channel-set")
			} else {
				settings.InspectTriggersEnabled.UserBannedOnOtherServers = false
				settings.InspectTriggersEnabled.UserNoCommonServers = false
				settings.InspectTriggersEnabled.UserNewlyCreatedAccount = false
				successMessage = helpers.GetText("plugins.mod.inspects-channel-disabled")
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
					helpers.SendMessage(msg.ChannelID, "Searching for users on all servers with Robyul. 💬")
				} else {
					helpers.SendMessage(msg.ChannelID, "Searching for users on this server. 💬")
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
					_, err := helpers.SendMessage(msg.ChannelID, "Found no user who matches your search text. 🕵")
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
				helpers.SendMessage(msg.ChannelID, "Goodbye <:blobwave:317048219098021888>")
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
				settings.Prefix = newPrefix
				err = helpers.GuildSettingsSet(channel.GuildID, settings)
				helpers.Relax(err)

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

func (m *Mod) getTroublemakerReports(user *discordgo.User) []DB_Troublemaker_Entry {
	var entryBucket []DB_Troublemaker_Entry
	listCursor, err := rethink.Table("troublemakerlog").Filter(
		rethink.Row.Field("userid").Eq(user.ID),
	).Run(helpers.GetDB())
	helpers.Relax(err)
	defer listCursor.Close()
	listCursor.All(&entryBucket)

	return entryBucket
}

func (m *Mod) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {
	go func() {
		// Get invite link
		var usedInvite CacheInviteInformation
		invites, err := session.GuildInvites(member.GuildID)
		if err != nil {
			cache.GetLogger().WithField("module", "mod").Error(fmt.Sprintf("error getting invites from guild #%s: %s",
				member.GuildID, err.Error()))
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

		joinedAt, err := discordgo.Timestamp(member.JoinedAt).Parse()
		if err != nil {
			joinedAt = time.Now()
		}
		// Save join in DB
		newJoinLog := DB_Mod_JoinLog{
			GuildID:                   member.GuildID,
			UserID:                    member.User.ID,
			JoinedAt:                  joinedAt,
			InviteCodeUsed:            usedInvite.Code,
			InviteCodeCreatedByUserID: usedInvite.CreatedByUserID,
			InviteCodeCreatedAt:       usedInvite.CreatedAt,
		}
		err = m.InsertJoinLog(newJoinLog)
		if err != nil {
			raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
		}
		go func() {
			if member.User.ID == session.State.User.ID { // Don't inspect Robyul
				return
			}

			if helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserBannedOnOtherServers ||
				helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserNoCommonServers ||
				helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserNewlyCreatedAccount ||
				helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserReported ||
				helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserMultipleJoins ||
				helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserBannedDiscordlistNet {
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

				if !((helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserBannedOnOtherServers && len(bannedOnServerList) > 0) ||
					(helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserNoCommonServers && (len(isOnServerList)-1) <= 0) ||
					(helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserNewlyCreatedAccount && joinedTime.After(oneWeekAgo)) ||
					(helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserReported && len(troublemakerReports) > 0) ||
					(helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserMultipleJoins && len(joins) > 1) ||
					(helpers.GuildSettingsGetCached(member.GuildID).InspectTriggersEnabled.UserBannedDiscordlistNet && isBannedOnBansdiscordlistNet)) {
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
					resultBansText += fmt.Sprintf("✅ User is banned on none servers.\n◾Checked %d servers.", len(session.State.Guilds)-len(checkFailedServerList))
				} else {
					resultBansText += fmt.Sprintf("⚠ User is banned on **%d** server(s).\n◾Checked %d servers.", len(bannedOnServerList), len(session.State.Guilds)-len(checkFailedServerList))
				}

				commonGuildsText := ""
				if len(isOnServerList)-1 > 0 { // -1 to exclude the server the user is currently on
					commonGuildsText += fmt.Sprintf("✅ User is on **%d** other server(s) with Robyul.", len(isOnServerList)-1)
				} else {
					commonGuildsText += "❓ User is on **none** other servers with Robyul."
				}
				joinedTimeText := ""
				if !joinedTime.After(oneWeekAgo) {
					joinedTimeText += fmt.Sprintf("✅ User Account got created %s.\n◾Joined at %s.", helpers.SinceInDaysText(joinedTime), joinedTime.Format(time.ANSIC))
				} else if !joinedTime.After(oneDayAgo) {
					joinedTimeText += fmt.Sprintf("❓ User Account is less than one Week old.\n◾Joined at %s.", joinedTime.Format(time.ANSIC))
				} else {
					joinedTimeText += fmt.Sprintf("⚠ User Account is less than one Day old.\n◾Joined at %s.", joinedTime.Format(time.ANSIC))
				}

				var troublemakerReportsText string
				if len(troublemakerReports) <= 0 {
					troublemakerReportsText = "✅ User never got reported"
				} else {
					troublemakerReportsText = fmt.Sprintf("⚠ User got reported %d time(s)\nUse `_troublemaker list %s` to view the details.", len(troublemakerReports), member.User.ID)
				}

				joinsText := ""
				if len(joins) == 0 {
					joinsText = "✅ User never joined this server\n"
				} else if len(joins) == 1 {
					if joins[0].InviteCodeUsed != "" {
						createdByUser, _ := helpers.GetUser(joins[0].InviteCodeCreatedByUserID)
						if createdByUser == nil {
							createdByUser = new(discordgo.User)
							createdByUser.ID = joins[0].InviteCodeCreatedByUserID
							createdByUser.Username = "N/A"
						}

						joinsText = fmt.Sprintf("✅ User joined this server once with the invite `%s` created by `%s (#%s)` %s\n",
							joins[0].InviteCodeUsed, createdByUser.Username, createdByUser.ID, humanize.Time(joins[0].InviteCodeCreatedAt))
					} else {
						joinsText = "✅ User joined this server once\nGive Robyul the `Manage Server` permission to see using which invite.\n"
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

						joinsText = fmt.Sprintf("⚠ User joined this server %d times\nLast time with the invite `%s` created by `%s (#%s)` %s\n",
							len(joins),
							lastJoin.InviteCodeUsed, createdByUser.Username, createdByUser.ID, humanize.Time(lastJoin.InviteCodeCreatedAt))
					} else {

						joinsText = fmt.Sprintf("⚠ User joined this server %d times\nGive Robyul the `Manage Server` permission to see using which invites.\n", len(joins))
					}
				}

				helpers.RelaxLog(err)
				isBannedOnBansdiscordlistNetText := "✅ User is not banned.\n"
				if isBannedOnBansdiscordlistNet {
					isBannedOnBansdiscordlistNetText = "⚠ User is banned on [bans.discordlist.net](https://bans.discordlist.net/).\n"
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
						resultEmbed.Description += "\n⚠ I wasn't able to gather the ban list for this server!\nPlease give Robyul the permission `Ban Members` to help other servers."
						break
					}
				}

				_, err = helpers.SendEmbed(helpers.GuildSettingsGetCached(member.GuildID).InspectsChannel, resultEmbed)
				if err != nil {
					cache.GetLogger().WithField("module", "mod").Error(fmt.Sprintf("Failed to send guild join inspect to channel #%s on guild #%s: %s",
						helpers.GuildSettingsGetCached(member.GuildID).InspectsChannel, member.GuildID, err.Error()))
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
	}()
	go func() {
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
}

func (m *Mod) InsertJoinLog(entry DB_Mod_JoinLog) error {
	if entry.UserID != "" {
		insert := rethink.Table("mod_joinlog").Insert(entry)
		_, err := insert.RunWrite(helpers.GetDB())
		return err
	}
	return nil
}

func (m *Mod) GetJoins(userID string, guildID string) ([]DB_Mod_JoinLog, error) {
	var entryBucket []DB_Mod_JoinLog
	listCursor, err := rethink.Table("mod_joinlog").Filter(
		rethink.Row.Field("userid").Eq(userID),
	).Run(helpers.GetDB())
	defer listCursor.Close()
	if err != nil {
		return entryBucket, err
	}
	err = listCursor.All(&entryBucket)
	result := make([]DB_Mod_JoinLog, 0)
	if err != nil {
		return result, err
	}

	for _, logEntry := range entryBucket {
		if logEntry.GuildID == guildID {
			result = append(result, logEntry)
		}
	}

	return result, nil
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
	go func() {
		bannedOnGuild, err := helpers.GetGuild(user.GuildID)
		if err != nil {
			raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
			return
		}
		// don't post if bot can't access the ban list of the server the user got banned on
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
						resultBansText += fmt.Sprintf("✅ User is banned on none servers.\n◾Checked %d servers.", len(session.State.Guilds)-len(checkFailedServerList))
					} else {
						resultBansText += fmt.Sprintf("⚠ User is banned on **%d** server(s).\n◾Checked %d servers.", len(bannedOnServerList), len(session.State.Guilds)-len(checkFailedServerList))
					}

					isOnServerList := m.inspectCommonServers(user.User)
					commonGuildsText := ""
					if len(isOnServerList)-1 > 0 { // -1 to exclude the server the user is currently on
						commonGuildsText += fmt.Sprintf("✅ User is on **%d** other server(s) with Robyul.", len(isOnServerList)-1)
					} else {
						commonGuildsText += "❓ User is on **none** other servers with Robyul."
					}

					joinedTime := helpers.GetTimeFromSnowflake(user.User.ID)
					oneDayAgo := time.Now().AddDate(0, 0, -1)
					oneWeekAgo := time.Now().AddDate(0, 0, -7)
					joinedTimeText := ""
					if !joinedTime.After(oneWeekAgo) {
						joinedTimeText += fmt.Sprintf("✅ User Account got created %s.\n◾Joined at %s.", helpers.SinceInDaysText(joinedTime), joinedTime.Format(time.ANSIC))
					} else if !joinedTime.After(oneDayAgo) {
						joinedTimeText += fmt.Sprintf("❓ User Account is less than one Week old.\n◾Joined at %s.", joinedTime.Format(time.ANSIC))
					} else {
						joinedTimeText += fmt.Sprintf("⚠ User Account is less than one Day old.\n◾Joined at %s.", joinedTime.Format(time.ANSIC))
					}

					troublemakerReports := m.getTroublemakerReports(user.User)
					var troublemakerReportsText string
					if len(troublemakerReports) <= 0 {
						troublemakerReportsText = "✅ User never got reported"
					} else {
						troublemakerReportsText = fmt.Sprintf("⚠ User got reported %d time(s)\nUse `_troublemaker list %s` to view the details.", len(troublemakerReports), user.User.ID)
					}

					joins, _ := m.GetJoins(user.User.ID, targetGuild.ID)
					joinsText := ""
					if len(joins) == 0 {
						joinsText = "✅ User never joined this server\n"
					} else if len(joins) == 1 {
						if joins[0].InviteCodeUsed != "" {
							createdByUser, _ := helpers.GetUser(joins[0].InviteCodeCreatedByUserID)
							if createdByUser == nil {
								createdByUser = new(discordgo.User)
								createdByUser.ID = joins[0].InviteCodeCreatedByUserID
								createdByUser.Username = "N/A"
							}

							joinsText = fmt.Sprintf("✅ User joined this server once with the invite `%s` created by `%s (#%s)` %s\n",
								joins[0].InviteCodeUsed, createdByUser.Username, createdByUser.ID, humanize.Time(joins[0].InviteCodeCreatedAt))
						} else {
							joinsText = "✅ User joined this server once\nGive Robyul the `Manage Server` permission to see using which invite.\n"
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

							joinsText = fmt.Sprintf("⚠ User joined this server %d times\nLast time with the invite `%s` created by `%s (#%s)` %s\n",
								len(joins),
								lastJoin.InviteCodeUsed, createdByUser.Username, createdByUser.ID, humanize.Time(lastJoin.InviteCodeCreatedAt))
						} else {

							joinsText = fmt.Sprintf("⚠ User joined this server %d times\nGive Robyul the `Manage Server` permission to see using which invites.\n", len(joins))
						}
					}

					resultEmbed.Fields = []*discordgo.MessageEmbedField{
						{Name: "Bans", Value: resultBansText, Inline: false},
						{Name: "Troublemaker Reports", Value: troublemakerReportsText, Inline: false},
						{Name: "Join History", Value: joinsText, Inline: false},
						{Name: "Common Servers", Value: commonGuildsText, Inline: false},
						{Name: "Account Age", Value: joinedTimeText, Inline: false},
					}

					for _, failedServer := range checkFailedServerList {
						if failedServer.ID == targetGuild.ID {
							resultEmbed.Description += "\n⚠ I wasn't able to gather the ban list for this server!\nPlease give Robyul the permission `Ban Members` to help other servers."
							break
						}
					}

					_, err = helpers.SendEmbed(helpers.GuildSettingsGetCached(targetGuild.ID).InspectsChannel, resultEmbed)
					if err != nil {
						cache.GetLogger().WithField("module", "mod").Error(fmt.Sprintf("Failed to send guild ban inspect to channel #%s on guild #%s: %s",
							helpers.GuildSettingsGetCached(targetGuild.ID).InspectsChannel, targetGuild.ID, err.Error()))
						if errD, ok := err.(*discordgo.RESTError); ok {
							if errD.Message.Code != 50001 {
								raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
							}
						} else {
							raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
						}
						continue
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
