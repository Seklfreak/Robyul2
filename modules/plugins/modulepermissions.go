package plugins

import (
	"strings"

	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

type modulePermissionsAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next modulePermissionsAction)

type ModulePermissions struct{}

func (mp *ModulePermissions) Commands() []string {
	return []string{
		"module",
		"modules",
		"modulepermissions",
	}
}

func (mp *ModulePermissions) Init(session *discordgo.Session) {
	err := helpers.RefreshModulePermissionsCache()
	helpers.Relax(err)
}

func (mp *ModulePermissions) Uninit(session *discordgo.Session) {

}

func (mp *ModulePermissions) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	defer helpers.Recover()

	session.ChannelTyping(msg.ChannelID)

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := mp.actionStart
	for action != nil {
		action = action(args, msg, &result)
	}
}

func (mp *ModulePermissions) actionStart(args []string, in *discordgo.Message, out **discordgo.MessageSend) modulePermissionsAction {
	cache.GetSession().ChannelTyping(in.ChannelID)

	if len(args) < 1 {
		*out = mp.newMsg("bot.arguments.too-few")
		return mp.actionFinish
	}

	switch args[0] {
	case "status", "list":
		return mp.actionStatus
	case "allow", "enable":
		return mp.actionAllow
	case "deny", "disable":
		return mp.actionDeny
	}

	*out = mp.newMsg("bot.arguments.invalid")
	return mp.actionFinish
}

func (mp *ModulePermissions) actionStatus(args []string, in *discordgo.Message, out **discordgo.MessageSend) modulePermissionsAction {
	if !helpers.IsMod(in) {
		*out = mp.newMsg("mod.no_permission")
		return mp.actionFinish
	}

	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)

	entries := helpers.GetModulePermissionEntries(channel.GuildID)

	var entryIsCategory bool
	var entryText, entryAllowText, entryDenyText, moduleAllowText, moduleDenyText,
		messageAllowRoles, messageDenyRoles, messageAllowChannels, messageDenyChannels,
		messageAllowCategory, messageDenyCategory, messageModuleList string

	for _, entry := range entries {
		entryIsCategory = false
		entryText = ""
		entryAllowText = ""
		entryDenyText = ""
		moduleAllowText = ""
		moduleDenyText = ""
		for _, module := range helpers.Modules {
			if entry.Allowed&module.Permission == module.Permission {
				moduleAllowText += "`" + helpers.GetModuleNameById(module.Permission) + "`, "
			}
			if entry.Denied&module.Permission == module.Permission {
				moduleDenyText += "`" + helpers.GetModuleNameById(module.Permission) + "`, "
			}
		}
		if entry.Allowed&helpers.ModulePermAllPlaceholder == helpers.ModulePermAllPlaceholder {
			moduleAllowText = "_ALL_"
		}
		if entry.Denied&helpers.ModulePermAllPlaceholder == helpers.ModulePermAllPlaceholder {
			moduleDenyText = "_ALL_"
		}
		if strings.HasSuffix(moduleAllowText, ", ") {
			moduleAllowText = moduleAllowText[:len(moduleAllowText)-2]
		}
		if strings.HasSuffix(moduleDenyText, ", ") {
			moduleDenyText = moduleDenyText[:len(moduleDenyText)-2]
		}
		switch entry.Type {
		case "channel":
			entryChannel, _ := helpers.GetChannel(entry.TargetID)
			if entryChannel != nil && entryChannel.ID != "" && entryChannel.Type == discordgo.ChannelTypeGuildCategory {
				entryIsCategory = true
			}
			entryText += "<#" + entry.TargetID + ">: "
			if entry.Allowed > 0 {
				entryAllowText += entryText + moduleAllowText + "\n"
				if entryIsCategory {
					messageAllowCategory += entryAllowText
				} else {
					messageAllowChannels += entryAllowText
				}
			}
			if entry.Denied > 0 {
				entryDenyText += entryText + moduleDenyText + "\n"
				if entryIsCategory {
					messageDenyCategory += entryDenyText
				} else {
					messageDenyChannels += entryDenyText
				}
			}
			break
		case "role":
			role, _ := cache.GetSession().State.Role(entry.GuildID, entry.TargetID)
			if role == nil || role.ID == "" {
				continue
			}
			entryText += role.Name + ": "
			if entry.Allowed > 0 {
				entryAllowText += entryText + moduleAllowText + "\n"
				messageAllowRoles += entryAllowText
			}
			if entry.Denied > 0 {
				entryDenyText += entryText + moduleDenyText + "\n"
				messageDenyRoles += entryDenyText
			}
			break
		}
	}

	for _, module := range helpers.Modules {
		messageModuleList += "`" + helpers.GetModuleNameById(module.Permission) + "`, "
	}
	if strings.HasSuffix(messageModuleList, ", ") {
		messageModuleList = messageModuleList[:len(messageModuleList)-2]
	}

	if messageAllowRoles == "" {
		messageAllowRoles = "_None_\n"
	}
	if messageDenyRoles == "" {
		messageDenyRoles = "_None_\n"
	}
	if messageAllowChannels == "" {
		messageAllowChannels = "_None_\n"
	}
	if messageDenyChannels == "" {
		messageDenyChannels = "_None_\n"
	}
	if messageAllowCategory == "" {
		messageAllowCategory = "_None_\n"
	}
	if messageDenyCategory == "" {
		messageDenyCategory = "_None_\n"
	}
	if messageModuleList == "" {
		messageModuleList = "_None_\n"
	}

	messageFinal := "__**:arrow_down: Allowed Roles**__\n" + messageAllowRoles +
		"__**:arrow_down: Denied Roles**__\n" + messageDenyRoles +
		"__**:arrow_down: Allowed Channels**__\n" + messageAllowChannels +
		"__**:arrow_down: Denied Channels**__\n" + messageDenyChannels +
		"__**:arrow_down: Allowed Categories**__\n" + messageAllowCategory +
		"__**:arrow_down: Denied Categories**__\n" + messageDenyCategory +
		"__**Module List**__\n" + messageModuleList
	*out = mp.newMsg(messageFinal)
	return mp.actionFinish
}

func (mp *ModulePermissions) actionAllow(args []string, in *discordgo.Message, out **discordgo.MessageSend) modulePermissionsAction {
	if !helpers.IsMod(in) {
		*out = mp.newMsg("mod.no_permission")
		return mp.actionFinish
	}

	if len(args) < 3 {
		*out = mp.newMsg("bot.arguments.too-few")
		return mp.actionFinish
	}

	var permToAdd models.ModulePermissionsModule
	if "all" == strings.ToLower(args[1]) {
		permToAdd = helpers.ModulePermAll | helpers.ModulePermAllPlaceholder
	}
	for _, module := range helpers.Modules {
		for _, moduleName := range module.Names {
			if strings.ToLower(moduleName) == strings.ToLower(args[1]) {
				permToAdd = module.Permission
			}
		}
	}
	if permToAdd == 0 {
		*out = mp.newMsg("plugins.modulepermissions.module-not-found")
		return mp.actionFinish
	}

	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)
	guild, err := helpers.GetGuild(channel.GuildID)
	helpers.Relax(err)

	targetChannel, err := helpers.GetChannelFromMention(in, args[2])
	if err == nil && targetChannel != nil && targetChannel.ID != "" {
		previousPerms := helpers.GetAllowedForChannel(targetChannel.GuildID, targetChannel.ID)
		if previousPerms&permToAdd == permToAdd {
			err = helpers.SetAllowedForChannel(
				targetChannel.GuildID, targetChannel.ID, (previousPerms&^permToAdd)&^helpers.ModulePermAllPlaceholder)
			helpers.Relax(err)

			_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetChannel.ID,
				models.EventlogTargetTypeChannel, in.Author.ID,
				models.EventlogTypeRobyulModuleAllowChannelRemove, "",
				nil,
				[]models.ElasticEventlogOption{
					{
						Key:   "module_allow_channel_removed",
						Value: helpers.GetModuleNameById(permToAdd),
					},
				}, false)
			helpers.RelaxLog(err)

			*out = mp.newMsg("plugins.modulepermissions.set-allow-removed")
			return mp.actionFinish
		}

		err = helpers.SetAllowedForChannel(
			targetChannel.GuildID, targetChannel.ID, previousPerms|permToAdd)
		helpers.Relax(err)

		_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetChannel.ID,
			models.EventlogTargetTypeChannel, in.Author.ID,
			models.EventlogTypeRobyulModuleAllowChannelAdd, "",
			nil,
			[]models.ElasticEventlogOption{
				{
					Key:   "module_allow_channel_added",
					Value: helpers.GetModuleNameById(permToAdd),
				},
			}, false)
		helpers.RelaxLog(err)

		*out = mp.newMsg("plugins.modulepermissions.set-allow-added")
		return mp.actionFinish
	}

	var targetRole *discordgo.Role
	for _, guildRole := range guild.Roles {
		if guildRole.ID == args[2] ||
			strings.ToLower(guildRole.Name) == strings.ToLower(args[2]) ||
			(guildRole.ID == guild.ID && strings.ToLower(args[2]) == "everyone") {
			targetRole = guildRole
		}
	}
	if targetRole != nil && targetRole.ID != "" {
		previousPerms := helpers.GetAllowedForRole(guild.ID, targetRole.ID)
		if previousPerms&permToAdd == permToAdd {
			err = helpers.SetAllowedForRole(
				guild.ID, targetRole.ID, (previousPerms&^permToAdd)&^helpers.ModulePermAllPlaceholder)
			helpers.Relax(err)

			_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetRole.ID,
				models.EventlogTargetTypeRole, in.Author.ID,
				models.EventlogTypeRobyulModuleAllowRoleRemove, "",
				nil,
				[]models.ElasticEventlogOption{
					{
						Key:   "module_allow_role_removed",
						Value: helpers.GetModuleNameById(permToAdd),
					},
				}, false)
			helpers.RelaxLog(err)

			*out = mp.newMsg("plugins.modulepermissions.set-allow-removed")
			return mp.actionFinish
		}

		err = helpers.SetAllowedForRole(
			guild.ID, targetRole.ID, previousPerms|permToAdd)
		helpers.Relax(err)

		_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetRole.ID,
			models.EventlogTargetTypeRole, in.Author.ID,
			models.EventlogTypeRobyulModuleAllowRoleAdd, "",
			nil,
			[]models.ElasticEventlogOption{
				{
					Key:   "module_allow_role_added",
					Value: helpers.GetModuleNameById(permToAdd),
				},
			}, false)
		helpers.RelaxLog(err)

		*out = mp.newMsg("plugins.modulepermissions.set-allow-added")
		return mp.actionFinish
	}

	*out = mp.newMsg("bot.arguments.invalid")
	return mp.actionFinish
}

func (mp *ModulePermissions) actionDeny(args []string, in *discordgo.Message, out **discordgo.MessageSend) modulePermissionsAction {
	if len(args) < 3 {
		*out = mp.newMsg("bot.arguments.too-few")
		return mp.actionFinish
	}

	var permToAdd models.ModulePermissionsModule
	if "all" == strings.ToLower(args[1]) {
		permToAdd = helpers.ModulePermAll | helpers.ModulePermAllPlaceholder
	}
	for _, module := range helpers.Modules {
		for _, moduleName := range module.Names {
			if strings.ToLower(moduleName) == strings.ToLower(args[1]) {
				permToAdd = module.Permission
			}
		}
	}
	if permToAdd == 0 {
		*out = mp.newMsg("plugins.modulepermissions.module-not-found")
		return mp.actionFinish
	}

	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)
	guild, err := helpers.GetGuild(channel.GuildID)
	helpers.Relax(err)

	targetChannel, err := helpers.GetChannelOfAnyTypeFromMention(in, args[2])
	if err == nil && targetChannel != nil && targetChannel.ID != "" {
		previousPerms := helpers.GetDeniedForChannel(targetChannel.GuildID, targetChannel.ID)
		if previousPerms&permToAdd == permToAdd {
			err = helpers.SetDeniedForChannel(
				targetChannel.GuildID, targetChannel.ID, (previousPerms&^permToAdd)&^helpers.ModulePermAllPlaceholder)
			helpers.Relax(err)

			_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetChannel.ID,
				models.EventlogTargetTypeChannel, in.Author.ID,
				models.EventlogTypeRobyulModuleDenyChannelRemove, "",
				nil,
				[]models.ElasticEventlogOption{
					{
						Key:   "module_deny_channel_removed",
						Value: helpers.GetModuleNameById(permToAdd),
					},
				}, false)
			helpers.RelaxLog(err)

			*out = mp.newMsg("plugins.modulepermissions.set-deny-removed")
			return mp.actionFinish
		}

		err = helpers.SetDeniedForChannel(
			targetChannel.GuildID, targetChannel.ID, previousPerms|permToAdd)
		helpers.Relax(err)

		_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetChannel.ID,
			models.EventlogTargetTypeChannel, in.Author.ID,
			models.EventlogTypeRobyulModuleDenyChannelAdd, "",
			nil,
			[]models.ElasticEventlogOption{
				{
					Key:   "module_deny_channel_added",
					Value: helpers.GetModuleNameById(permToAdd),
				},
			}, false)
		helpers.RelaxLog(err)

		*out = mp.newMsg("plugins.modulepermissions.set-deny-added")
		return mp.actionFinish
	}

	var targetRole *discordgo.Role
	for _, guildRole := range guild.Roles {
		if guildRole.ID == args[2] ||
			strings.ToLower(guildRole.Name) == strings.ToLower(args[2]) ||
			(guildRole.ID == guild.ID && strings.ToLower(args[2]) == "everyone") {
			targetRole = guildRole
		}
	}
	if targetRole != nil && targetRole.ID != "" {
		previousPerms := helpers.GetDeniedForRole(guild.ID, targetRole.ID)
		if previousPerms&permToAdd == permToAdd {
			err = helpers.SetDeniedForRole(
				guild.ID, targetRole.ID, (previousPerms&^permToAdd)&^helpers.ModulePermAllPlaceholder)
			helpers.Relax(err)

			_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetRole.ID,
				models.EventlogTargetTypeRole, in.Author.ID,
				models.EventlogTypeRobyulModuleDenyRoleRemove, "",
				nil,
				[]models.ElasticEventlogOption{
					{
						Key:   "module_deny_role_removed",
						Value: helpers.GetModuleNameById(permToAdd),
					},
				}, false)
			helpers.RelaxLog(err)

			*out = mp.newMsg("plugins.modulepermissions.set-deny-removed")
			return mp.actionFinish
		}

		err = helpers.SetDeniedForRole(
			guild.ID, targetRole.ID, previousPerms|permToAdd)
		helpers.Relax(err)

		_, err = helpers.EventlogLog(time.Now(), channel.GuildID, targetRole.ID,
			models.EventlogTargetTypeRole, in.Author.ID,
			models.EventlogTypeRobyulModuleDenyRoleAdd, "",
			nil,
			[]models.ElasticEventlogOption{
				{
					Key:   "module_deny_role_added",
					Value: helpers.GetModuleNameById(permToAdd),
				},
			}, false)
		helpers.RelaxLog(err)

		*out = mp.newMsg("plugins.modulepermissions.set-deny-added")
		return mp.actionFinish
	}

	*out = mp.newMsg("bot.arguments.invalid")
	return mp.actionFinish
}

func (mp *ModulePermissions) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) modulePermissionsAction {
	_, err := helpers.SendComplex(in.ChannelID, *out)
	helpers.RelaxMessage(err, in.ChannelID, in.ID)

	return nil
}

func (mp *ModulePermissions) newMsg(content string) *discordgo.MessageSend {
	return &discordgo.MessageSend{Content: helpers.GetText(content)}
}

func (mp *ModulePermissions) logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "modulepermissions")
}
