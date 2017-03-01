package plugins

import (
	"fmt"
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	"strings"
	"time"
)

type Bias struct{}

type AssignableRole_Channels struct {
	Channels []AssignableRole_Channel
}

type AssignableRole_Channel struct {
	ServerID   string
	ChannelID  string
	Categories []AssignableRole_Category
}

type AssignableRole_Category struct {
	Label  string
	Pool   string
	Hidden bool
	Limit  int
	Roles  []AssignableRole_Role
}

type AssignableRole_Role struct {
	Name    string
	Print   string
	Aliases []string
}

func (m *Bias) Commands() []string {
	return []string{
		"bias-help",
	}
}

var (
	biasChannels = AssignableRole_Channels{
		Channels: []AssignableRole_Channel{
			{
				ServerID:  "250216966436945920",
				ChannelID: "250220888077631489",
				Categories: []AssignableRole_Category{
					{
						Label:  "Bias Roles",
						Pool:   "bias-roles",
						Hidden: false,
						Limit:  1,
						Roles: []AssignableRole_Role{
							{
								Name:    "Somi 💎",
								Print:   "Somi",
								Aliases: []string{"Somi", "Jeon Somi", "Ennik Douma", "전소미", "소미"},
							},
							{
								Name:    "Sejeong 💎",
								Print:   "Sejeong",
								Aliases: []string{"Sejeong", "Kim Sejeong", "김세정", "세정"},
							},
							{
								Name:    "Yoojung 💎",
								Print:   "Yoojung",
								Aliases: []string{"Yoojung", "Choi Yoojung", "최유정", "유정"},
							},
							{
								Name:    "Chungha 💎",
								Print:   "Chungha",
								Aliases: []string{"Chungha", "Kim Chungha", "김청하", "청하"},
							},
							{
								Name:    "Sohye 💎",
								Print:   "Sohye",
								Aliases: []string{"Sohye", "Kim Sohye", "김소혜", "소혜"},
							},
							{
								Name:    "Jieqiong 💎",
								Print:   "Jieqiong",
								Aliases: []string{"Jieqiong", "Zhou Jieqiong", "Kyulkyung", "周洁琼", "주결경", "결경"},
							},
							{
								Name:    "Chaeyeon 💎",
								Print:   "Chaeyeon",
								Aliases: []string{"Chaeyeon", "Jung Chaeyeon", "정채연", "채연"},
							},
							{
								Name:    "Doyeon 💎",
								Print:   "Doyeon",
								Aliases: []string{"Doyeon", "Kim Doyeon", "김도연", "도연"},
							},
							{
								Name:    "Mina 💎",
								Print:   "Mina",
								Aliases: []string{"Mina", "Kang Mina", "강미나", "미나"},
							},
							{
								Name:    "Nayoung 💎",
								Print:   "Nayoung",
								Aliases: []string{"Nayoung", "Im Nayoung", "Lim Nayoung", "임나영", "나영"},
							},
							{
								Name:    "Yeonjung 💎",
								Print:   "Yeonjung",
								Aliases: []string{"Yeonjung", "Yu Yeonjung", "유연정", "연정"},
							},
							{
								Name:    "OT11",
								Print:   "OT11",
								Aliases: []string{"OT11", "I.O.I", "IOI", "아이오아이"},
							},
							{
								Name:    "DoDaeng",
								Print:   "DoDaeng",
								Aliases: []string{"DoDaeng"},
							},
						},
					},
					{
						Label:  "Secondary Bias Roles",
						Pool:   "bias-roles",
						Hidden: true,
						Limit:  2,
						Roles: []AssignableRole_Role{
							{
								Name:    "Somi",
								Print:   "Somi",
								Aliases: []string{"Somi", "Jeon Somi", "Ennik Douma", "전소미", "소미"},
							},
							{
								Name:    "Sejeong",
								Print:   "Sejeong",
								Aliases: []string{"Sejeong", "Kim Sejeong", "김세정", "세정"},
							},
							{
								Name:    "Yoojung",
								Print:   "Yoojung",
								Aliases: []string{"Yoojung", "Choi Yoojung", "최유정", "유정"},
							},
							{
								Name:    "Chungha",
								Print:   "Chungha",
								Aliases: []string{"Chungha", "Kim Chungha", "김청하", "청하"},
							},
							{
								Name:    "Sohye",
								Print:   "Sohye",
								Aliases: []string{"Sohye", "Kim Sohye", "김소혜", "소혜"},
							},
							{
								Name:    "Jieqiong",
								Print:   "Jieqiong",
								Aliases: []string{"Jieqiong", "Zhou Jieqiong", "Kyulkyung", "周洁琼", "주결경", "결경"},
							},
							{
								Name:    "Chaeyeon",
								Print:   "Chaeyeon",
								Aliases: []string{"Chaeyeon", "Jung Chaeyeon", "정채연", "채연"},
							},
							{
								Name:    "Doyeon",
								Print:   "Doyeon",
								Aliases: []string{"Doyeon", "Kim Doyeon", "김도연", "도연"},
							},
							{
								Name:    "Mina",
								Print:   "Mina",
								Aliases: []string{"Mina", "Kang Mina", "강미나", "미나"},
							},
							{
								Name:    "Nayoung",
								Print:   "Nayoung",
								Aliases: []string{"Nayoung", "Im Nayoung", "Lim Nayoung", "임나영", "나영"},
							},
							{
								Name:    "Yeonjung",
								Print:   "Yeonjung",
								Aliases: []string{"Yeonjung", "Yu Yeonjung", "유연정", "연정"},
							},
						},
					},
					{
						Label:  "Group Roles",
						Pool:   "",
						Hidden: false,
						Limit:  1,
						Roles: []AssignableRole_Role{
							{
								Name:    "DIA",
								Print:   "DIA",
								Aliases: []string{"DIA", "DIAMOND", "Do It Amazing", "다이아", "MBK"},
							},
							{
								Name:    "Gugudan",
								Print:   "Gugudan",
								Aliases: []string{"Gugudan", "gu9udan", "구구단", "Jellyfish"},
							},
							{
								Name:    "iTeen",
								Print:   "iTeen",
								Aliases: []string{"iTeen", "Fantagio"},
							},
							{
								Name:    "JYP",
								Print:   "JYP",
								Aliases: []string{"JYP"},
							},
							{
								Name:    "M&H",
								Print:   "M&H",
								Aliases: []string{"M&H"},
							},
							{
								Name:    "Pristin",
								Print:   "Pristin",
								Aliases: []string{"Pristin", "프리스틴", "Pledis Girlz", "Pledis"},
							},
							{
								Name:    "S&P",
								Print:   "S&P",
								Aliases: []string{"S&P"},
							},
							{
								Name:    "WJSN",
								Print:   "WJSN",
								Aliases: []string{"WJSN", "Cosmic Girls", "우주소녀", "Starship"},
							},
						},
					},
					{
						Label:  "Gaming Roles",
						Pool:   "",
						Hidden: false,
						Limit:  -1,
						Roles: []AssignableRole_Role{
							{
								Name:    "Overwatch",
								Print:   "Overwatch",
								Aliases: []string{"Overwatch", "OW"},
							},
							{
								Name:    "League of Legends",
								Print:   "League of Legends",
								Aliases: []string{"League of Legends", "LoL", "league"},
							},
							{
								Name:    "DOTA",
								Print:   "DOTA",
								Aliases: []string{"DOTA", "DOTA2"},
							},
						},
					},
					{
						Label:  "Other Roles",
						Pool:   "",
						Hidden: false,
						Limit:  -1,
						Roles: []AssignableRole_Role{
							{
								Name:    "Karaoke",
								Print:   "Karaoke",
								Aliases: []string{"Karaoke", "norebang"},
							},
						},
					},
				},
			},
		},
	}
)

func (m *Bias) Init(session *discordgo.Session) {

}

func (m *Bias) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	helpers.RequireAdmin(msg, func() {
		for _, biasChannel := range biasChannels.Channels {
			if msg.ChannelID == biasChannel.ChannelID {
				exampleRoleName := ""
				biasListText := ""
				for _, biasCategory := range biasChannel.Categories {
					if biasCategory.Hidden == true {
						continue
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
					// TODO: Calculate limit
					if calculatedLimit == 1 {
						biasListText += " (**`One Role`** Max)"
					} else if calculatedLimit > 1 {
						biasListText += fmt.Sprintf(" (**`%s Roles`** Max)", strings.Title(helpers.HumanizeNumber(calculatedLimit)))
					}
				}
				_, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.bias.bias-help-message", biasListText, exampleRoleName, exampleRoleName))
				helpers.Relax(err)
				return
			}
		}

		_, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.bias.no-bias-config"))
		helpers.Relax(err)
	})
}

func (m *Bias) ActionAll(content string, msg *discordgo.Message, session *discordgo.Session) {
	for _, biasChannel := range biasChannels.Channels {
		if msg.ChannelID == biasChannel.ChannelID {
			channel, err := session.Channel(msg.ChannelID)
			helpers.Relax(err)
			guild, err := session.Guild(channel.GuildID)
			helpers.Relax(err)
			member, err := session.GuildMember(guild.ID, msg.Author.ID)
			helpers.Relax(err)
			var messagesToDelete []*discordgo.Message
			messagesToDelete = append(messagesToDelete, msg)
			var requestIsAddRole bool
			isRequest := false
			if strings.HasPrefix(content, "+") {
				requestIsAddRole = true
				isRequest = true
			} else if strings.HasPrefix(content, "-") {
				requestIsAddRole = false
				isRequest = true
			}
			if isRequest == true {
				requestedRoleName := m.CleanUpRoleName(content)
				denyReason := ""
				type Role_Information struct {
					Role        AssignableRole_Role
					DiscordRole *discordgo.Role
				}
				var roleToAddOrDelete Role_Information
			FindRoleLoop:
				for _, category := range biasChannel.Categories {
				TryRoleLoop:
					for _, role := range category.Roles {
						for _, label := range role.Aliases {
							if strings.ToLower(label) == requestedRoleName {
								discordRole := m.GetDiscordRole(role, guild)
								if discordRole.ID != "" {
									memberHasRole := m.MemberHasRole(member, discordRole)
									if requestIsAddRole == true && memberHasRole == true {
										denyReason = helpers.GetText("plugins.bias.add-role-already")
										continue TryRoleLoop
									}
									if requestIsAddRole == false && memberHasRole == false {
										denyReason = helpers.GetText("plugins.bias.remove-role-not-found")
										continue TryRoleLoop
									}
									categoryRolesAssigned := m.CategoryRolesAssigned(member, guild.ID, category)
									if requestIsAddRole == true && (category.Limit >= 0 && len(categoryRolesAssigned) >= category.Limit) {
										denyReason = helpers.GetText("plugins.bias.role-limit-reached")
										continue TryRoleLoop
									}
									if requestIsAddRole == true && category.Pool != "" {
										for _, poolCategories := range biasChannel.Categories {
											if poolCategories.Pool == category.Pool {
												for _, poolRole := range poolCategories.Roles {
													if poolRole.Print == role.Print {
														poolDiscordRole := m.GetDiscordRole(poolRole, guild)
														if poolDiscordRole.ID != "" && m.MemberHasRole(member, poolDiscordRole) {
															denyReason = helpers.GetText("plugins.bias.add-role-already")
															continue TryRoleLoop
														}
													}
												}
											}
										}
									}

									roleToAddOrDelete = Role_Information{Role: role, DiscordRole: discordRole}

									break FindRoleLoop
								}

							}
						}
					}
				}
				if roleToAddOrDelete.Role.Name != "" {
					if requestIsAddRole == true {
						session.GuildMemberRoleAdd(guild.ID, msg.Author.ID, roleToAddOrDelete.DiscordRole.ID)
						newMessage, err := session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("<@%s> %s", msg.Author.ID, helpers.GetText("plugins.bias.role-added")))
						helpers.Relax(err)
						messagesToDelete = append(messagesToDelete, newMessage)
					} else {
						session.GuildMemberRoleRemove(guild.ID, msg.Author.ID, roleToAddOrDelete.DiscordRole.ID)
						newMessage, err := session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("<@%s> %s", msg.Author.ID, helpers.GetText("plugins.bias.role-removed")))
						helpers.Relax(err)
						messagesToDelete = append(messagesToDelete, newMessage)
					}
				} else if denyReason != "" {
					newMessage, err := session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("<@%s> %s", msg.Author.ID, denyReason))
					helpers.Relax(err)
					messagesToDelete = append(messagesToDelete, newMessage)
				} else {
					newMessage, err := session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("<@%s> %s", msg.Author.ID, helpers.GetText("plugins.bias.role-not-found")))
					helpers.Relax(err)
					messagesToDelete = append(messagesToDelete, newMessage)
				}
			}
			// Delete messages after ten seconds
			time.Sleep(10 * time.Second)
			for _, messagsToDelete := range messagesToDelete {
				session.ChannelMessageDelete(msg.ChannelID, messagsToDelete.ID)

			}
		}
	}
}

func (m *Bias) CategoryRolesAssigned(member *discordgo.Member, guildID string, category AssignableRole_Category) []AssignableRole_Role {
	var rolesAssigned []AssignableRole_Role
	guildRoles, err := cache.GetSession().GuildRoles(guildID)
	helpers.Relax(err)
	for _, discordRoleId := range member.Roles {
		for _, discordGuildRole := range guildRoles {
			if discordRoleId == discordGuildRole.ID {
				for _, assignableRole := range category.Roles {
					if strings.ToLower(assignableRole.Name) == strings.ToLower(discordGuildRole.Name) {
						rolesAssigned = append(rolesAssigned, assignableRole)
					}
				}
			}
		}
	}

	return rolesAssigned
}

func (m *Bias) GetDiscordRole(role AssignableRole_Role, guild *discordgo.Guild) *discordgo.Role {
	var discordRole *discordgo.Role
	for _, discordRole = range guild.Roles {
		if strings.ToLower(role.Name) == strings.ToLower(discordRole.Name) {
			return discordRole
		}
	}
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
	inputName = strings.Replace(inputName, "+", "", 1)
	inputName = strings.Replace(inputName, "-", "", 1)
	inputName = strings.Replace(inputName, "name", "", 1)
	inputName = strings.Trim(inputName, " ")
	inputName = strings.ToLower(inputName)
	return inputName
}
