package plugins

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/globalsign/mgo/bson"
	"github.com/kennygrant/sanitize"
)

type Bias struct{}

func (m *Bias) Commands() []string {
	return []string{
		"bias",
	}
}

var (
	biasChannels []models.BiasEntry
)

func (m *Bias) Init(session *discordgo.Session) {
	// refresh cache
	err := helpers.MDbIter(helpers.MdbCollection(models.BiasTable).Find(nil)).All(&biasChannels)
	helpers.Relax(err)
}

func (m *Bias) Uninit(session *discordgo.Session) {

}

func (m *Bias) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermBias) {
		return
	}

	args := strings.Fields(content)
	if len(args) >= 1 {
		switch args[0] {
		case "help":
			helpers.RequireAdminOrStaff(msg, func() {
				session.ChannelTyping(msg.ChannelID)
				for _, biasChannel := range biasChannels {
					if msg.ChannelID == biasChannel.ChannelID {
						exampleRoleName := ""
						biasListText := ""
						for _, biasCategory := range biasChannel.Categories {
							if biasCategory.Hidden == true {
								continue
							}
							if biasCategory.Message != "" {
								biasListText += "\n" + biasCategory.Message
							}
							biasListText += fmt.Sprintf("\n%s: ", biasCategory.Label)
							for i, biasRole := range biasCategory.Roles {
								if exampleRoleName == "" {
									exampleRoleName = biasRole.Print
								}
								if i != 0 {
									if i+1 < len(biasCategory.Roles) {
										biasListText += ", "
									} else {
										biasListText += " and "
									}
								}
								biasListText += fmt.Sprintf("**`%s`**", biasRole.Print)
							}
							calculatedLimit := biasCategory.Limit
							if biasCategory.Pool != "" {
								calculatedLimit = 0
								for _, poolCategorie := range biasChannel.Categories {
									if poolCategorie.Pool == biasCategory.Pool {
										calculatedLimit += poolCategorie.Limit
									}
								}
							}
							if calculatedLimit == 1 {
								biasListText += " (**`One Role`** Max)"
							} else if calculatedLimit > 1 {
								biasListText += fmt.Sprintf(" (**`%s Roles`** Max)", strings.Title(helpers.HumanizeNumber(calculatedLimit)))
							}
						}
						for _, page := range helpers.Pagify(helpers.GetTextF("plugins.bias.bias-help-message",
							biasListText, exampleRoleName, exampleRoleName), ",") {
							helpers.SendMessage(msg.ChannelID, page)
						}
						return
					}
				}

				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.bias.no-bias-config"))
				helpers.Relax(err)
			})
		case "refresh":
			helpers.RequireBotAdmin(msg, func() {
				session.ChannelTyping(msg.ChannelID)

				// refresh cache
				err := helpers.MDbIter(helpers.MdbCollection(models.BiasTable).Find(nil)).All(&biasChannels)
				helpers.Relax(err)

				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.bias.refreshed-config"))
				helpers.Relax(err)
			})
		case "set-config":
			helpers.RequireMod(msg, func() {
				session.ChannelTyping(msg.ChannelID)

				if len(args) < 2 {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					helpers.Relax(err)
					return
				}

				if len(msg.Attachments) <= 0 {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					helpers.Relax(err)
					return
				}

				targetChannel, err := helpers.GetChannelFromMention(msg, args[1])
				if err != nil || targetChannel.ID == "" {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					return
				}

				var channelConfig []models.BiasEntryCategory
				channelConfigJson := helpers.NetGet(msg.Attachments[0].URL)
				channelConfigJson = bytes.TrimPrefix(channelConfigJson, []byte("\xef\xbb\xbf")) // removes BOM
				err = json.Unmarshal(channelConfigJson, &channelConfig)
				if err != nil {
					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.bias.set-config-error-invalid"))
					helpers.Relax(err)
					return
				}

				var previousConfig models.BiasEntry
				err = helpers.MdbOne(
					helpers.MdbCollection(models.BiasTable).Find(bson.M{"channelid": targetChannel.ID}),
					&previousConfig,
				)
				var previousConfigBytes []byte
				if !helpers.IsMdbNotFound(err) {
					previousConfigBytes, err = json.Marshal(previousConfig.Categories)
					helpers.Relax(err)
				}

				newBiasConfigBytes, err := json.Marshal(channelConfig)
				helpers.Relax(err)

				err = helpers.MDbUpsert(
					models.BiasTable,
					bson.M{"channelid": targetChannel.ID},
					models.BiasEntry{
						GuildID:    targetChannel.GuildID,
						ChannelID:  targetChannel.ID,
						Categories: channelConfig,
					},
				)
				helpers.Relax(err)

				if previousConfig.ID == "" {
					_, err = helpers.EventlogLog(time.Now(), targetChannel.GuildID, targetChannel.ID,
						models.EventlogTargetTypeChannel, msg.Author.ID,
						models.EventlogTypeRobyulBiasConfigCreate, "",
						nil,
						[]models.ElasticEventlogOption{
							{
								Key:   "bias_config",
								Value: string(newBiasConfigBytes),
							},
						}, false)
					helpers.RelaxLog(err)
				} else {
					_, err = helpers.EventlogLog(time.Now(), targetChannel.GuildID, targetChannel.ID,
						models.EventlogTargetTypeChannel, msg.Author.ID,
						models.EventlogTypeRobyulBiasConfigUpdate, "",
						[]models.ElasticEventlogChange{
							{
								Key:      "bias_config",
								OldValue: string(previousConfigBytes),
								NewValue: string(newBiasConfigBytes),
							},
						},
						nil, false)
					helpers.RelaxLog(err)
				}

				// refresh cache
				err = helpers.MDbIter(helpers.MdbCollection(models.BiasTable).Find(nil)).All(&biasChannels)
				helpers.Relax(err)

				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.bias.updated-config"))
				helpers.Relax(err)
				return
			})
		case "get-config":
			helpers.RequireMod(msg, func() {
				session.ChannelTyping(msg.ChannelID)

				if len(args) < 2 {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					helpers.Relax(err)
					return
				}

				targetChannel, err := helpers.GetChannelFromMention(msg, args[1])
				if err != nil || targetChannel.ID == "" {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					return
				}
				targetGuild, err := helpers.GetGuild(targetChannel.GuildID)
				helpers.Relax(err)

				var channelConfig models.BiasEntry
				err = helpers.MdbOne(
					helpers.MdbCollection(models.BiasTable).Find(bson.M{"channelid": targetChannel.ID}),
					&channelConfig,
				)
				if helpers.IsMdbNotFound(err) {
					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.bias.no-bias-config"))
					helpers.Relax(err)
					return
				}

				channelConfigJson, err := json.MarshalIndent(channelConfig.Categories, "", "    ")
				helpers.Relax(err)

				_, err = session.ChannelFileSend(msg.ChannelID, sanitize.Path(targetGuild.Name)+"-"+sanitize.Path(targetChannel.Name)+"-robyul-bias-config.json", bytes.NewReader(channelConfigJson))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)

				return
			})
		case "delete-config", "remove-config":
			helpers.RequireAdminOrStaff(msg, func() {
				session.ChannelTyping(msg.ChannelID)

				if len(args) < 2 {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					helpers.Relax(err)
					return
				}

				targetChannel, err := helpers.GetChannelFromMention(msg, args[1])
				if err != nil || targetChannel.ID == "" {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					return
				}

				var channelConfig models.BiasEntry
				err = helpers.MdbOne(
					helpers.MdbCollection(models.BiasTable).Find(bson.M{"channelid": targetChannel.ID}),
					&channelConfig,
				)
				if helpers.IsMdbNotFound(err) {
					_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.bias.no-bias-config"))
					helpers.Relax(err)
					return
				}

				previousConfigBytes, err := json.Marshal(channelConfig.Categories)
				helpers.Relax(err)

				err = helpers.MDbDelete(models.BiasTable, channelConfig.ID)
				helpers.Relax(err)

				// refresh cache
				err = helpers.MDbIter(helpers.MdbCollection(models.BiasTable).Find(nil)).All(&biasChannels)
				helpers.Relax(err)

				_, err = helpers.EventlogLog(time.Now(), targetChannel.GuildID, targetChannel.ID,
					models.EventlogTargetTypeChannel, msg.Author.ID,
					models.EventlogTypeRobyulBiasConfigDelete, "",
					nil,
					[]models.ElasticEventlogOption{
						{
							Key:   "bias_config",
							Value: string(previousConfigBytes),
						},
					}, false)
				helpers.RelaxLog(err)

				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.bias.delete-config-success"))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			})
		case "stats":
			session.ChannelTyping(msg.ChannelID)

			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)
			guild, err := helpers.GetGuild(channel.GuildID)
			helpers.Relax(err)

			members := make([]*discordgo.Member, 0)
			for _, botGuild := range session.State.Guilds {
				if botGuild.ID == guild.ID {
					for _, member := range guild.Members {
						members = append(members, member)
					}
				}
			}

			statsText := ""

			statsPrinted := 0
			for _, biasChannel := range biasChannels {
				if biasChannel.GuildID == channel.GuildID {
					biasDiscordChannel, err := helpers.GetChannel(biasChannel.ChannelID)
					if err != nil || biasDiscordChannel == nil {
						continue
					}

					for _, biasCategory := range biasChannel.Categories {
						categoryNumbers := make(BiasRoleStatList, 0)
						if biasCategory.Hidden == true && biasCategory.Pool == "" {
							continue
						}
						for _, biasRole := range biasCategory.Roles {
							discordRole := m.GetDiscordRole(biasRole, guild)
							if discordRole != nil {
								categoryNumbers = append(categoryNumbers, BiasRoleStat{
									RoleName: discordRole.Name, Members: 0,
								})
								for _, member := range members {
									for _, memberRole := range member.Roles {
										if memberRole == discordRole.ID {
											// user has the role
											for i, biasRoleStat := range categoryNumbers {
												if biasRoleStat.RoleName == discordRole.Name {
													categoryNumbers[i].Members++
												}
											}
										}
									}
								}
							}
						}
						sort.Sort(categoryNumbers)
						if len(categoryNumbers) > 0 {
							statsText += fmt.Sprintf("__**%s:**__\n", biasCategory.Label)
							for _, biasRoleStat := range categoryNumbers {
								statsText += fmt.Sprintf("**%s**: %d Members\n",
									biasRoleStat.RoleName, biasRoleStat.Members)
							}
						}
					}
					statsPrinted++
				}
			}

			if statsPrinted <= 0 {
				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.bias.no-stats"))
				helpers.Relax(err)
			} else {
				for _, page := range helpers.Pagify(statsText, "\n") {
					_, err = helpers.SendMessage(msg.ChannelID, page)
					helpers.Relax(err)
				}
			}
		}
	}
}

func (m *Bias) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {
	go func() {
		defer helpers.Recover()

		for _, biasChannel := range biasChannels {
			if msg.ChannelID == biasChannel.ChannelID {
				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)
				guild, err := helpers.GetGuild(channel.GuildID)
				helpers.Relax(err)
				member, err := helpers.GetGuildMemberWithoutApi(guild.ID, msg.Author.ID)
				helpers.Relax(err)
				guildRoles, err := session.GuildRoles(guild.ID)
				if err != nil {
					if err, ok := err.(*discordgo.RESTError); ok && err.Message.Code == 50013 {
						newMessages, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.bias.generic-error"))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						// Delete messages after ten seconds
						time.Sleep(10 * time.Second)
						for _, newMessage := range newMessages {
							session.ChannelMessageDelete(newMessage.ChannelID, newMessage.ID)
						}
						session.ChannelMessageDelete(msg.ChannelID, msg.ID)
						return
					} else {
						helpers.Relax(err)
					}
				}
				helpers.Relax(err)
				var messagesToDelete []*discordgo.Message
				messagesToDelete = append(messagesToDelete, msg)
				isRequest := true
				if strings.HasPrefix(content, helpers.GetPrefixForServer(channel.GuildID)) {
					isRequest = false
				}
				if isRequest {
					session.ChannelTyping(msg.ChannelID)
					// split up multiple requests
					requests := make([]string, 0)
					var lastStart int
					nextLookup := content
					var lastSign string
					for {
						nextRequestIndex := strings.IndexFunc(nextLookup, func(r rune) bool {
							return r == '+' || r == '-'
						})
						if lastSign == "" && nextRequestIndex == -1 {
							requests = append(requests, content)
							break
						} else {
							if nextRequestIndex >= 0 {
								newRequest := nextLookup[0:nextRequestIndex]
								if strings.TrimSpace(newRequest) != "" {
									requests = append(requests, lastSign+strings.TrimSpace(newRequest))
								}
							} else if strings.TrimSpace(nextLookup) != "" {
								requests = append(requests, lastSign+strings.TrimSpace(nextLookup))
							}
							lastStart = nextRequestIndex
							if len(nextLookup) > 0 && lastStart >= 0 {
								if len(nextLookup) >= lastStart+1 {
									lastSign = string(nextLookup[lastStart])
									nextLookup = nextLookup[lastStart+1:]
								}
							} else {
								break
							}
						}
					}
					// find out which changes we should do and apply changes
					rolesAdded := make([]string, 0)
					rolesRemoved := make([]string, 0)
					rolesErrors := make([]string, 0)

					var requestIsAddRole bool
					var errorText string
					for _, request := range requests {
						//fmt.Println("request:", request)
						requestIsAddRole = true
						if strings.HasPrefix(request, "-") {
							requestIsAddRole = false
						}
						errorText = ""

						requestedRoleName := m.CleanUpRoleName(request)
					FindRoleLoop:
						for _, category := range biasChannel.Categories {
						TryRoleLoop:
							for _, role := range category.Roles {
								for _, label := range role.Aliases {
									if strings.ToLower(label) == requestedRoleName {
										discordRole := m.GetDiscordRole(role, guild)
										if discordRole != nil && discordRole.ID != "" {
											memberHasRole := m.MemberHasRole(member, discordRole)
											//fmt.Println("member has role", discordRole.Name, "?", memberHasRole)
											if requestIsAddRole == true && memberHasRole == true {
												errorText = helpers.GetText("plugins.bias.add-role-already")
												continue TryRoleLoop
											}
											if requestIsAddRole == false && memberHasRole == false {
												errorText = helpers.GetText("plugins.bias.remove-role-not-found")
												continue TryRoleLoop
											}
											categoryRolesAssigned := m.CategoryRolesAssigned(member, guildRoles, category)
											if requestIsAddRole == true && (category.Limit >= 0 && len(categoryRolesAssigned) >= category.Limit) {
												errorText = helpers.GetText("plugins.bias.role-limit-reached")
												continue TryRoleLoop
											}
											if requestIsAddRole == true && category.Pool != "" {
												for _, poolCategories := range biasChannel.Categories {
													if poolCategories.Pool == category.Pool {
														for _, poolRole := range poolCategories.Roles {
															if poolRole.Print == role.Print {
																poolDiscordRole := m.GetDiscordRole(poolRole, guild)
																if poolDiscordRole != nil && poolDiscordRole.ID != "" && m.MemberHasRole(member, poolDiscordRole) {
																	errorText = helpers.GetText("plugins.bias.add-role-already")
																	continue TryRoleLoop
																}
															}
														}
													}
												}
											}

											errorText = ""
											if requestIsAddRole {
												if role.Name != "" && discordRole != nil {
													err = session.GuildMemberRoleAdd(guild.ID, msg.Author.ID, discordRole.ID)
													if err != nil {
														//fmt.Println("failed to add role", discordRole.Name)
														errorText = helpers.GetText("plugins.bias.generic-error")
													} else {
														//fmt.Println("added role", discordRole.Name)
														rolesAdded = append(rolesAdded, role.Print)
													}
												}
											} else {
												if role.Name != "" && discordRole != nil {
													err = session.GuildMemberRoleRemove(guild.ID, msg.Author.ID, discordRole.ID)
													if err != nil {
														//fmt.Println("failed to remove role", discordRole.Name)
														errorText = helpers.GetText("plugins.bias.generic-error")
													} else {
														//fmt.Println("removed role", discordRole.Name)
														rolesRemoved = append(rolesRemoved, role.Print)
													}
												}
											}

											member, err = helpers.GetGuildMemberWithoutApi(channel.GuildID, msg.Author.ID)
											helpers.Relax(err)

											break FindRoleLoop
										}

									}
								}
							}
						}

						if errorText != "" {
							rolesErrors = append(rolesErrors, errorText)
						}
					}
					// Print message
					//fmt.Printf("added: %+v\n", rolesAdded)
					//fmt.Printf("removed: %+v\n", rolesRemoved)
					//fmt.Printf("errors: %+v\n", rolesErrors)
					if len(rolesAdded) <= 0 && len(rolesRemoved) <= 0 && len(rolesErrors) <= 0 {
						newMessage, err := helpers.SendMessage(msg.ChannelID, fmt.Sprintf("<@%s> %s", msg.Author.ID, helpers.GetText("plugins.bias.role-not-found")))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						messagesToDelete = append(messagesToDelete, newMessage...)
					} else {
						if len(rolesAdded) == 1 && len(rolesRemoved) == 0 && len(rolesErrors) == 0 {
							newMessage, err := helpers.SendMessage(msg.ChannelID, fmt.Sprintf("<@%s> %s", msg.Author.ID, helpers.GetText("plugins.bias.role-added")))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							messagesToDelete = append(messagesToDelete, newMessage...)
						} else if len(rolesAdded) == 0 && len(rolesRemoved) == 1 && len(rolesErrors) == 0 {
							newMessage, err := helpers.SendMessage(msg.ChannelID, fmt.Sprintf("<@%s> %s", msg.Author.ID, helpers.GetText("plugins.bias.role-removed")))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							messagesToDelete = append(messagesToDelete, newMessage...)
						} else if len(rolesAdded) == 0 && len(rolesRemoved) == 0 && len(rolesErrors) == 1 {
							newMessage, err := helpers.SendMessage(msg.ChannelID, fmt.Sprintf("<@%s> %s", msg.Author.ID, rolesErrors[0]))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							messagesToDelete = append(messagesToDelete, newMessage...)
						} else {
							newMessage, err := helpers.SendMessage(msg.ChannelID, fmt.Sprintf("<@%s> %s", msg.Author.ID,
								helpers.GetTextF(
									"plugins.bias.roles-batch",
									len(rolesAdded), len(rolesRemoved), len(rolesErrors),
								)))
							helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
							messagesToDelete = append(messagesToDelete, newMessage...)
						}
					}
				}
				// Delete messages after ten seconds
				time.Sleep(10 * time.Second)
				for _, messageToDelete := range messagesToDelete {
					if messageToDelete != nil {
						session.ChannelMessageDelete(msg.ChannelID, messageToDelete.ID)
					}
				}
			}
		}
	}()
}

func (m *Bias) CategoryRolesAssigned(member *discordgo.Member, guildRoles []*discordgo.Role, category models.BiasEntryCategory) []models.BiasEntryRole {
	var rolesAssigned []models.BiasEntryRole
	for _, discordRoleId := range member.Roles {
		for _, discordGuildRole := range guildRoles {
			if discordRoleId == discordGuildRole.ID {
				for _, assignableRole := range category.Roles {
					if strings.ToLower(assignableRole.Name) == strings.ToLower(discordGuildRole.Name) || assignableRole.Name == discordGuildRole.ID {
						rolesAssigned = append(rolesAssigned, assignableRole)
					}
				}
			}
		}
	}

	return rolesAssigned
}

func (m *Bias) GetDiscordRole(role models.BiasEntryRole, guild *discordgo.Guild) *discordgo.Role {
	for _, discordRole := range guild.Roles {
		if strings.ToLower(role.Name) == strings.ToLower(discordRole.Name) || role.Name == discordRole.ID {
			return discordRole
		}
	}
	var discordRole *discordgo.Role
	return discordRole
}

func (m *Bias) MemberHasRole(member *discordgo.Member, role *discordgo.Role) bool {
	for _, memberRole := range member.Roles {
		if memberRole == role.ID {
			return true
		}
	}
	return false
}

func (m *Bias) CleanUpRoleName(inputName string) string {
	inputName = strings.TrimPrefix(inputName, "+")
	inputName = strings.TrimPrefix(inputName, "-")
	inputName = strings.TrimSpace(inputName)
	inputName = strings.TrimPrefix(inputName, "name")
	inputName = strings.TrimSpace(inputName)
	inputName = strings.ToLower(inputName)
	return inputName
}

func (m *Bias) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {

}
func (m *Bias) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {

}
func (m *Bias) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {
	// Native emojis or custom emoji names (without :)
	go func() {
		defer helpers.Recover()

		for _, biasChannel := range biasChannels {
			if reaction.ChannelID == biasChannel.ChannelID {
				channel, err := helpers.GetChannel(reaction.ChannelID)
				helpers.Relax(err)
				guild, err := helpers.GetGuild(channel.GuildID)
				helpers.Relax(err)
				member, err := helpers.GetGuildMemberWithoutApi(guild.ID, reaction.UserID)
				helpers.Relax(err)
				if member.User.Bot {
					return
				}
				guildRoles, err := session.GuildRoles(guild.ID)
				if err != nil {
					if err, ok := err.(*discordgo.RESTError); ok && err.Message.Code == 50013 {
						newMessages, err := helpers.SendMessage(reaction.ChannelID, helpers.GetText("plugins.bias.generic-error"))
						if err != nil {
							if errD, ok := err.(*discordgo.RESTError); ok {
								if errD.Message.Code == discordgo.ErrCodeMissingPermissions {
									return
								}
							}
							helpers.Relax(err)
						}
						// Delete messages after ten seconds
						time.Sleep(10 * time.Second)
						for _, newMessage := range newMessages {
							session.ChannelMessageDelete(newMessage.ChannelID, newMessage.ID)
						}
						return
					}
					helpers.Relax(err)
				}

				var errorText string
				roleAdded := false

				//fmt.Println("got reaction:", reaction.Emoji.Name)
			FindRoleLoop:
				for _, category := range biasChannel.Categories {
				TryRoleLoop:
					for _, role := range category.Roles {
						for _, reactionAlias := range role.Reactions {
							if strings.ToLower(reactionAlias) == strings.ToLower(reaction.Emoji.Name) {
								discordRole := m.GetDiscordRole(role, guild)
								if discordRole != nil && discordRole.ID != "" {
									memberHasRole := m.MemberHasRole(member, discordRole)
									//fmt.Println("member has role", discordRole.Name, "?", memberHasRole)
									if memberHasRole == true {
										errorText = helpers.GetText("plugins.bias.add-role-already")
										continue TryRoleLoop
									}
									categoryRolesAssigned := m.CategoryRolesAssigned(member, guildRoles, category)
									if category.Limit >= 0 && len(categoryRolesAssigned) >= category.Limit {
										errorText = helpers.GetText("plugins.bias.role-limit-reached")
										continue TryRoleLoop
									}
									if category.Pool != "" {
										for _, poolCategories := range biasChannel.Categories {
											if poolCategories.Pool == category.Pool {
												for _, poolRole := range poolCategories.Roles {
													if poolRole.Print == role.Print {
														poolDiscordRole := m.GetDiscordRole(poolRole, guild)
														if poolDiscordRole != nil && poolDiscordRole.ID != "" && m.MemberHasRole(member, poolDiscordRole) {
															errorText = helpers.GetText("plugins.bias.add-role-already")
															continue TryRoleLoop
														}
													}
												}
											}
										}
									}
									errorText = ""
									if role.Name != "" && discordRole != nil {
										err = session.GuildMemberRoleAdd(guild.ID, reaction.UserID, discordRole.ID)
										if err != nil {
											//fmt.Println("failed to add role", discordRole.Name)
											errorText = helpers.GetText("plugins.bias.generic-error")
										} else {
											//fmt.Println("added role", discordRole.Name)
											roleAdded = true
										}
									}

									break FindRoleLoop
								}
							}
						}
					}
				}

				var newMessages []*discordgo.Message

				if roleAdded {
					newMessages, err = helpers.SendMessage(reaction.ChannelID, fmt.Sprintf("<@%s> %s", reaction.UserID, helpers.GetText("plugins.bias.role-added")))
					helpers.RelaxMessage(err, reaction.ChannelID, "")
				} else if errorText != "" {
					newMessages, err = helpers.SendMessage(reaction.ChannelID, fmt.Sprintf("<@%s> %s", reaction.UserID, errorText))
					helpers.RelaxMessage(err, reaction.ChannelID, "")
				}

				if len(newMessages) > 0 {
					time.Sleep(10 * time.Second)
					for _, newMessage := range newMessages {
						session.ChannelMessageDelete(newMessage.ChannelID, newMessage.ID)
					}
				}
			}
		}
	}()

}
func (m *Bias) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {

}
func (m *Bias) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {

}
func (m *Bias) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}

func (m *Bias) OnMessageDelete(msg *discordgo.MessageDelete, session *discordgo.Session) {

}

type BiasRoleStat struct {
	RoleName string
	Members  int
}

type BiasRoleStatList []BiasRoleStat

func (p BiasRoleStatList) Len() int           { return len(p) }
func (p BiasRoleStatList) Less(i, j int) bool { return p[i].Members > p[j].Members }
func (p BiasRoleStatList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
