package plugins

import (
	"strings"

	"fmt"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/helpers/dgwidgets"
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

type configAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next configAction)

type Config struct{}

func (m *Config) Commands() []string {
	return []string{
		"config",
	}
}

func (m *Config) Init(session *discordgo.Session) {
}

func (m *Config) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermMod) {
		return
	}

	session.ChannelTyping(msg.ChannelID)

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := m.actionStart
	for action != nil {
		action = action(args, msg, &result)
	}
}

func (m *Config) actionStart(args []string, in *discordgo.Message, out **discordgo.MessageSend) configAction {
	cache.GetSession().ChannelTyping(in.ChannelID)

	return m.actionStatus
}

func (m *Config) actionStatus(args []string, in *discordgo.Message, out **discordgo.MessageSend) configAction {
	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)

	targetGuild, err := helpers.GetGuild(channel.GuildID)
	helpers.Relax(err)

	if len(args) >= 1 {
		if helpers.IsRobyulMod(in.Author.ID) {
			targetGuild, err = helpers.GetGuild(args[0])
			helpers.Relax(err)
		}
	}

	if !helpers.IsModByID(targetGuild.ID, in.Author.ID) && !helpers.IsRobyulMod(in.Author.ID) {
		*out = m.newMsg("mod.no_permission")
		return m.actionFinish
	}

	guildConfig := helpers.GuildSettingsGetCached(targetGuild.ID)

	prefix := helpers.GetPrefixForServer(targetGuild.ID)

	pages := make([]*discordgo.MessageEmbed, 0)

	adminsText := "Role Name Matching"
	if guildConfig.AdminRoleIDs != nil && len(guildConfig.AdminRoleIDs) > 0 {
		adminsText = "set, <@"
		adminsText += strings.Join(guildConfig.AdminRoleIDs, ">, <@")
		adminsText += ">"
	}

	modsText := "Role Name Matching"
	if guildConfig.ModRoleIDs != nil && len(guildConfig.ModRoleIDs) > 0 {
		modsText = "set, <@"
		modsText += strings.Join(guildConfig.ModRoleIDs, ">, <@")
		modsText += ">"
	}

	inspectsText := "Disabled"
	if guildConfig.InspectsChannel != "" {
		inspectsText = "Enabled, in <#" + guildConfig.InspectsChannel + ">"
	}

	nukeText := "Disabled"
	if guildConfig.NukeIsParticipating {
		nukeText = "Enabled, log in <#" + guildConfig.NukeLogChannel + ">"
	}

	troublemakerText := "Disabled"
	if guildConfig.TroublemakerIsParticipating {
		troublemakerText = "Enabled, log in <#" + guildConfig.TroublemakerLogChannel + ">"
	}

	levelsText := "Level Up Notification: "
	if guildConfig.LevelsNotificationCode != "" {
		levelsText += "Enabled"
		if guildConfig.LevelsNotificationDeleteAfter > 0 {
			levelsText += fmt.Sprintf(", deleting after %d seconds", guildConfig.LevelsNotificationDeleteAfter)
		}
	} else {
		levelsText += "Disabled"
	}
	levelsText += "\nIgnored Users: "
	if guildConfig.LevelsIgnoredUserIDs == nil || len(guildConfig.LevelsIgnoredUserIDs) <= 0 {
		levelsText += "None"
	} else {
		levelsText += "<@"
		levelsText += strings.Join(guildConfig.LevelsIgnoredUserIDs, ">, <@")
		levelsText += ">"
	}
	levelsText += "\nIgnored Channels: "
	if guildConfig.LevelsIgnoredChannelIDs == nil || len(guildConfig.LevelsIgnoredChannelIDs) <= 0 {
		levelsText += "None"
	} else {
		levelsText += "<#"
		levelsText += strings.Join(guildConfig.LevelsIgnoredChannelIDs, ">, <#")
		levelsText += ">"
	}
	levelsText += fmt.Sprintf("\nMax Badges: %d", helpers.GetMaxBadgesForGuild(targetGuild.ID))

	var autoRolesText string
	if (guildConfig.AutoRoleIDs == nil || len(guildConfig.AutoRoleIDs) <= 0) ||
		guildConfig.DelayedAutoRoles == nil || len(guildConfig.DelayedAutoRoles) <= 0 {
		autoRolesText += "None"
	} else {
		if guildConfig.AutoRoleIDs != nil && len(guildConfig.AutoRoleIDs) > 0 {
			autoRolesText += "<@&"
			autoRolesText += strings.Join(guildConfig.AutoRoleIDs, ">, <@&")
			autoRolesText += ">"
		}
		if guildConfig.DelayedAutoRoles != nil && len(guildConfig.DelayedAutoRoles) > 0 {
			if autoRolesText != "" {
				autoRolesText += ", "
			}
			for _, delayedAutoRole := range guildConfig.DelayedAutoRoles {
				autoRolesText += fmt.Sprintf("<@&%s> after %s",
					delayedAutoRole.RoleID, delayedAutoRole.Delay.String())
			}
		}
	}

	starboardText := "Disabled"
	if guildConfig.StarboardChannelID != "" {
		starboardText = "Enabled, in <#" + guildConfig.StarboardChannelID + ">"
	}

	chatlogText := "Enabled"
	if guildConfig.ChatlogDisabled {
		chatlogText = "Disabled"
	}

	eventlogText := "Disabled"
	if !guildConfig.EventlogDisabled {
		eventlogText = "Enabled"
		if guildConfig.EventlogChannelIDs != nil && len(guildConfig.EventlogChannelIDs) > 0 {
			eventlogText += ", log in <#"
			eventlogText += strings.Join(guildConfig.EventlogChannelIDs, ">, <#")
			eventlogText += ">"
		}
	}

	var persistencyText string
	if !guildConfig.PersistencyBiasEnabled && (guildConfig.PersistencyRoleIDs == nil || len(guildConfig.PersistencyRoleIDs) <= 0) {
		persistencyText += "Disabled"
	} else {
		if guildConfig.PersistencyBiasEnabled {
			persistencyText += "Enabled for Bias Roles"
		}
		if guildConfig.PersistencyRoleIDs != nil && len(guildConfig.PersistencyRoleIDs) > 0 {
			if persistencyText == "" {
				persistencyText += "Enabled for "
			} else {
				persistencyText += ", and "
			}
			persistencyText += "<@&"
			persistencyText += strings.Join(guildConfig.PersistencyRoleIDs, ">, <@&")
			persistencyText += ">"
		}
	}

	perspectiveText := "Disabled"
	if guildConfig.PerspectiveIsParticipating {
		perspectiveText = "Enabled, log in <#" + guildConfig.PerspectiveChannelID + ">"
	}

	customCommandsText := "Moderators can add commands"
	if guildConfig.CustomCommandsEveryoneCanAdd {
		customCommandsText = "Everyone can add commands"
	}
	if guildConfig.CustomCommandsAddRoleID != "" {
		customCommandsText = "<@&" + guildConfig.CustomCommandsAddRoleID + "> and Moderators can add commands"
	}

	// TODO: info if blacklisted, or limited guild

	pages = append(pages, &discordgo.MessageEmbed{
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: "Prefix",
				Value: fmt.Sprintf("`%s`\n`@%s#%s set prefix <new prefix>`",
					prefix,
					cache.GetSession().State.User.Username, cache.GetSession().State.User.Discriminator),
			},
			{
				Name:  "Admins",
				Value: adminsText,
			},
			{
				Name:  "Mods",
				Value: modsText,
			},
			{
				Name:  "Auto Inspects",
				Value: inspectsText + fmt.Sprintf("\n`%sauto-inspects-channel <#channel or channel id>`", prefix),
			},
			{
				Name:  "Nuke",
				Value: nukeText + fmt.Sprintf("\n`%snuke participate <#channel or channel id>`", prefix),
			},
			{
				Name:  "Levels",
				Value: levelsText,
			},
			{
				Name:  "Troublemaker Log",
				Value: troublemakerText + fmt.Sprintf("\n`%stroublemaker participate <#channel or channel id>`", prefix),
			},
			{
				Name:  "Auto Roles",
				Value: autoRolesText,
			},
			{
				Name:  "Starboard",
				Value: starboardText,
			},
			{
				Name:  "Chatlog",
				Value: chatlogText,
			},
			{
				Name:  "Eventlog",
				Value: eventlogText,
			},
			{
				Name:  "Persistency",
				Value: persistencyText,
			},
			{
				Name:  "Perspective",
				Value: perspectiveText,
			},
			{
				Name:  "Custom Commands",
				Value: customCommandsText,
			},
		},
	})

	for _, page := range pages {
		page.Title = "Robyul Config for " + targetGuild.Name
		page.Color = 0xFADED
		page.Footer = &discordgo.MessageEmbedFooter{
			Text: "Server #" + targetGuild.ID,
		}
		if targetGuild.Icon != "" {
			page.Footer.IconURL = discordgo.EndpointGuildIcon(targetGuild.ID, targetGuild.Icon)
		}
	}

	if len(pages) > 1 {
		p := dgwidgets.NewPaginator(in.ChannelID, in.Author.ID)
		p.Add(pages...)
		p.Spawn()
	} else if len(pages) == 1 {
		*out = &discordgo.MessageSend{Embed: pages[0]}
		return m.actionFinish
	}

	return nil
}

func (m *Config) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) configAction {
	_, err := helpers.SendComplex(in.ChannelID, *out)
	helpers.Relax(err)

	return nil
}

func (m *Config) newMsg(content string) *discordgo.MessageSend {
	return &discordgo.MessageSend{Content: helpers.GetText(content)}
}

func (m *Config) Relax(err error) {
	if err != nil {
		panic(err)
	}
}

func (m *Config) logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "config")
}
