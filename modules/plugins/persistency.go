package plugins

import (
	"strings"

	"fmt"

	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	rethink "github.com/gorethink/gorethink"
	"github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack"
)

type PersistencyAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next PersistencyAction)

type Persistency struct{}

func (p *Persistency) Commands() []string {
	return []string{
		"persistency",
	}
}

func (p *Persistency) Init(session *discordgo.Session) {
	session.AddHandler(p.OnGuildMemberListChunk)
	session.AddHandler(p.OnGuildMemberUpdate)
}

func (p *Persistency) Uninit(session *discordgo.Session) {

}

// TODO: Store Nicknames, VC Mute and Deafen state

func (p *Persistency) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermPersistency) {
		return
	}

	session.ChannelTyping(msg.ChannelID)

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := p.actionStart
	for action != nil {
		action = action(args, msg, &result)
	}
}

func (p *Persistency) actionStart(args []string, in *discordgo.Message, out **discordgo.MessageSend) PersistencyAction {
	cache.GetSession().ChannelTyping(in.ChannelID)

	if len(args) < 1 {
		*out = p.newMsg(helpers.GetText("bot.arguments.too-few"))
		return p.actionFinish
	}

	switch args[0] {
	case "roles", "role":
		return p.roleAction
	case "toggle":
		return p.toggleAction
	case "status", "list":
		return p.statusAction
	}

	*out = p.newMsg(helpers.GetText("bot.arguments.invalid"))
	return p.actionFinish
}

func (p *Persistency) roleAction(args []string, in *discordgo.Message, out **discordgo.MessageSend) PersistencyAction {
	if len(args) < 2 {
		*out = p.newMsg("bot.arguments.too-few")
		return p.actionFinish
	}

	switch args[1] {
	case "add":
		return p.roleAddAction
	case "remove", "delete":
		return p.roleRemoveAction
	}

	*out = p.newMsg(helpers.GetText("bot.arguments.invalid"))
	return p.actionFinish
}

// [p]persistency roles add <role name or id>
func (p *Persistency) roleAddAction(args []string, in *discordgo.Message, out **discordgo.MessageSend) PersistencyAction {
	if len(args) < 3 {
		*out = p.newMsg("bot.arguments.too-few")
		return p.actionFinish
	}

	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)
	guild, err := helpers.GetGuild(channel.GuildID)
	helpers.Relax(err)

	roleNameToMatch := strings.Join(args[2:], " ")

	var roleToAdd *discordgo.Role

	for _, guildRole := range guild.Roles {
		if strings.ToLower(guildRole.Name) == strings.ToLower(roleNameToMatch) || guildRole.ID == roleNameToMatch {
			roleToAdd = guildRole
		}
	}

	if roleToAdd.ID == "" {
		*out = p.newMsg(helpers.GetText("bot.arguments.invalid"))
		return p.actionFinish
	}

	guildSettings := helpers.GuildSettingsGetCached(guild.ID)

	var alreadyAdded bool
	for _, settingsRoleID := range guildSettings.PersistencyRoleIDs {
		if settingsRoleID == roleToAdd.ID {
			alreadyAdded = true
		}
	}

	if alreadyAdded {
		*out = p.newMsg("plugins.persistency.role-add-error-duplicate")
		return p.actionFinish
	}

	beforeValue := guildSettings.PersistencyRoleIDs

	guildSettings.PersistencyRoleIDs = append(guildSettings.PersistencyRoleIDs, roleToAdd.ID)

	err = helpers.GuildSettingsSet(guild.ID, guildSettings)
	helpers.Relax(err)

	_, err = helpers.EventlogLog(time.Now(), channel.GuildID, channel.GuildID,
		models.EventlogTargetTypeGuild, in.Author.ID,
		models.EventlogTypeRobyulPersistencyRoleAdd, "",
		[]models.ElasticEventlogChange{
			{
				Key:      "persistency_roleids",
				OldValue: strings.Join(beforeValue, ","),
				NewValue: strings.Join(guildSettings.PersistencyRoleIDs, ","),
			},
		},
		[]models.ElasticEventlogOption{
			{
				Key:   "persistency_roleids_added",
				Value: roleToAdd.ID,
			},
		}, false)
	helpers.RelaxLog(err)

	*out = p.newMsg("plugins.persistency.role-add-success", roleToAdd.Name)
	return p.actionFinish
}

// [p]persistency roles remove <role name or id>
func (p *Persistency) roleRemoveAction(args []string, in *discordgo.Message, out **discordgo.MessageSend) PersistencyAction {
	if len(args) < 3 {
		*out = p.newMsg("bot.arguments.too-few")
		return p.actionFinish
	}

	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)
	guild, err := helpers.GetGuild(channel.GuildID)
	helpers.Relax(err)

	roleNameToMatch := strings.Join(args[2:], " ")

	var roleToRemove *discordgo.Role

	for _, guildRole := range guild.Roles {
		if strings.ToLower(guildRole.Name) == strings.ToLower(roleNameToMatch) || guildRole.ID == roleNameToMatch {
			roleToRemove = guildRole
		}
	}

	if roleToRemove.ID == "" {
		*out = p.newMsg(helpers.GetText("bot.arguments.invalid"))
		return p.actionFinish
	}

	guildSettings := helpers.GuildSettingsGetCached(guild.ID)

	var removed bool
	newPersistentRoles := make([]string, 0)

	for _, settingsRoleID := range guildSettings.PersistencyRoleIDs {
		if settingsRoleID == roleToRemove.ID {
			removed = true
			continue
		}
		newPersistentRoles = append(newPersistentRoles, settingsRoleID)
	}

	if !removed {
		*out = p.newMsg("plugins.persistency.role-remove-error-not-found")
		return p.actionFinish
	}

	beforeValue := guildSettings.PersistencyRoleIDs

	guildSettings.PersistencyRoleIDs = newPersistentRoles

	_, err = helpers.EventlogLog(time.Now(), channel.GuildID, channel.GuildID,
		models.EventlogTargetTypeGuild, in.Author.ID,
		models.EventlogTypeRobyulPersistencyRoleRemove, "",
		[]models.ElasticEventlogChange{
			{
				Key:      "persistency_roleids",
				OldValue: strings.Join(beforeValue, ","),
				NewValue: strings.Join(guildSettings.PersistencyRoleIDs, ","),
			},
		},
		[]models.ElasticEventlogOption{
			{
				Key:   "persistency_roleids_removed",
				Value: roleToRemove.ID,
			},
		}, false)
	helpers.RelaxLog(err)

	err = helpers.GuildSettingsSet(guild.ID, guildSettings)
	helpers.Relax(err)

	*out = p.newMsg("plugins.persistency.role-remove-success")
	return p.actionFinish
}

func (p *Persistency) statusAction(args []string, in *discordgo.Message, out **discordgo.MessageSend) PersistencyAction {
	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)

	var message string
	message += "__**Persistent Roles:**__\n"

	message += "**Custom Roles:** "
	customRoles := p.GetPersistentCustomRoles(channel.GuildID)
	if len(customRoles) <= 0 {
		message += helpers.GetText("plugins.persistency.status-roles-none")
	} else {
		for _, persistentRole := range customRoles {
			message += "`" + persistentRole.Name + "`, "
		}
		message = strings.TrimRight(message, ", ")
	}
	message += "\n"

	message += "**Managed Roles:** "
	managedRoles := p.GetPersistentManagedRoles(channel.GuildID)
	if len(managedRoles) <= 0 {
		message += helpers.GetText("plugins.persistency.status-roles-none")
	} else {
		for _, persistentRole := range managedRoles {
			message += "`" + persistentRole.Name + "`, "
		}
		message = strings.TrimRight(message, ", ")
	}
	message += "\n"

	message += "**Bias Roles:** "
	biasRoles := p.GetPersistentBiasRoles(channel.GuildID)
	if len(biasRoles) <= 0 {
		message += helpers.GetText("plugins.persistency.status-roles-none")
	} else {
		for _, persistentRole := range biasRoles {
			message += "`" + persistentRole.Name + "`, "
		}
		message = strings.TrimRight(message, ", ")
	}
	message += "\n"

	message += fmt.Sprintf("_found %d role(s) in total_", len(customRoles)+len(managedRoles)+len(biasRoles))

	for _, page := range helpers.Pagify(message, ",") {
		_, err = helpers.SendMessage(in.ChannelID, page)
		helpers.RelaxMessage(err, in.ChannelID, in.ID)
	}

	return nil
}

func (p *Persistency) toggleAction(args []string, in *discordgo.Message, out **discordgo.MessageSend) PersistencyAction {
	if len(args) < 2 {
		*out = p.newMsg("bot.arguments.too-few")
		return p.actionFinish
	}

	switch args[1] {
	case "bias-roles":
		return p.toggleBiasAction
	}

	*out = p.newMsg(helpers.GetText("bot.arguments.invalid"))
	return p.actionFinish
}

func (p *Persistency) toggleBiasAction(args []string, in *discordgo.Message, out **discordgo.MessageSend) PersistencyAction {
	if !helpers.IsMod(in) {
		*out = p.newMsg("mod.no_permission")
		return p.actionFinish
	}

	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)

	config := helpers.GuildSettingsGetCached(channel.GuildID)

	beforeValue := config.PersistencyBiasEnabled

	if config.PersistencyBiasEnabled {
		config.PersistencyBiasEnabled = false

		*out = p.newMsg(helpers.GetText("plugins.persistency.bias-persistency-disabled"))
	} else {
		config.PersistencyBiasEnabled = true

		*out = p.newMsg(helpers.GetText("plugins.persistency.bias-persistency-enabled"))
	}

	_, err = helpers.EventlogLog(time.Now(), channel.GuildID, channel.GuildID,
		models.EventlogTargetTypeGuild, in.Author.ID,
		models.EventlogTypeRobyulPersistencyBiasRoles, "",
		[]models.ElasticEventlogChange{
			{
				Key:      "persistency_biasroles_persist",
				OldValue: helpers.StoreBoolAsString(beforeValue),
				NewValue: helpers.StoreBoolAsString(config.PersistencyBiasEnabled),
			},
		},
		nil, false)
	helpers.RelaxLog(err)

	err = helpers.GuildSettingsSet(channel.GuildID, config)
	helpers.Relax(err)

	return p.actionFinish
}

func (p *Persistency) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) PersistencyAction {
	_, err := helpers.SendComplex(in.ChannelID, *out)
	helpers.Relax(err)

	return nil
}

func (p *Persistency) newMsg(content string, replacements ...interface{}) *discordgo.MessageSend {
	if len(replacements) < 1 {
		return &discordgo.MessageSend{Content: helpers.GetText(content)}
	}
	return &discordgo.MessageSend{Content: helpers.GetTextF(content, replacements...)}
}

func (p *Persistency) Relax(err error) {
	if err != nil {
		panic(err)
	}
}

func (p *Persistency) logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "persistency")
}

func (p *Persistency) OnGuildMemberListChunk(session *discordgo.Session, members *discordgo.GuildMembersChunk) {
	for _, member := range members.Members {
		err := p.cacheRoles(member.GuildID, member.User.ID, member.Roles)
		helpers.RelaxLog(err)
	}
}

func (p *Persistency) OnGuildMemberUpdate(session *discordgo.Session, member *discordgo.GuildMemberUpdate) {
	go func() {
		defer helpers.Recover()

		err := p.cacheRoles(member.GuildID, member.User.ID, member.Roles)
		helpers.RelaxLog(err)
	}()
}

func (p *Persistency) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {
	go func() {
		defer helpers.Recover()

		persistentRoles := p.GetPersistentRoles(member.GuildID)
		rolesToApply := make([]discordgo.Role, 0)

		cachedRoles := p.getCachedRoles(member.GuildID, member.User.ID)
		for _, roleID := range cachedRoles {
			for _, persistentRole := range persistentRoles {
				if persistentRole.ID == roleID {
					rolesToApply = append(rolesToApply, persistentRole)
				}
			}
		}

		if len(rolesToApply) <= 0 {
			return
		}

		var successfullyApplied int
		var failedApplied int

		for _, roleToApply := range rolesToApply {
			err := session.GuildMemberRoleAdd(member.GuildID, member.User.ID, roleToApply.ID)
			if err != nil {
				failedApplied++
				if errD, ok := err.(*discordgo.RESTError); ok {
					if errD.Message.Code == discordgo.ErrCodeMissingAccess {
						continue
					}
				}
				helpers.RelaxLog(err)
			} else {
				successfullyApplied++
			}
		}

		p.logger().WithField("UserID", member.User.ID).Debug(fmt.Sprintf("applied roles on join: %d/%d/%d/%d (applied/failed/found/cached)",
			successfullyApplied, failedApplied, len(rolesToApply), len(cachedRoles)))
	}()
}

func (p *Persistency) getRoleCacheRedisKey(GuildID string, UserID string) (key string) {
	key = "robyul2-discord:persistency:" + GuildID + ":" + UserID + ":roles"
	return
}

func (p *Persistency) cacheRoles(GuildID string, UserID string, roleIDs []string) (err error) {
	marshalled, err := msgpack.Marshal(roleIDs)
	if err != nil {
		return
	}

	err = cache.GetRedisClient().Set(p.getRoleCacheRedisKey(GuildID, UserID), marshalled, 0).Err()
	return err
}

func (p *Persistency) getCachedRoles(GuildID string, UserID string) (roleIDs []string) {
	marshalled, err := cache.GetRedisClient().Get(p.getRoleCacheRedisKey(GuildID, UserID)).Bytes()
	if err != nil {
		if !strings.Contains(err.Error(), "redis: nil") {
			helpers.RelaxLog(err)
		}
		return p.getRoleDBCache(GuildID, UserID)
	}

	err = msgpack.Unmarshal(marshalled, &roleIDs)
	if err != nil {
		helpers.RelaxLog(err)
		return
	}
	return roleIDs
}

func (p *Persistency) saveRoleDBCache(GuildID string, UserID string) (err error) {
	var persistedRoles models.PersistencyRolesEntry

	listCursor, _ := rethink.Table(models.PersistencyRolesTable).Filter(
		rethink.And(
			rethink.Row.Field("guild_id").Eq(GuildID),
			rethink.Row.Field("user_id").Eq(UserID),
		),
	).Run(helpers.GetDB())
	defer listCursor.Close()
	listCursor.One(&persistedRoles)

	persistedRoles.Roles = p.getCachedRoles(GuildID, UserID)
	if persistedRoles.ID != "" {
		// update
		_, err = rethink.Table(models.PersistencyRolesTable).Update(persistedRoles).Run(helpers.GetDB())
	} else {
		// insert
		persistedRoles.GuildID = GuildID
		persistedRoles.UserID = UserID

		insert := rethink.Table(models.PersistencyRolesTable).Insert(persistedRoles)
		_, err = insert.RunWrite(helpers.GetDB())
	}

	return err
}

func (p *Persistency) getRoleDBCache(GuildID string, UserID string) (roleIDs []string) {
	var persistedRoles models.PersistencyRolesEntry

	listCursor, _ := rethink.Table(models.PersistencyRolesTable).Filter(
		rethink.And(
			rethink.Row.Field("guild_id").Eq(GuildID),
			rethink.Row.Field("user_id").Eq(UserID),
		),
	).Run(helpers.GetDB())
	defer listCursor.Close()
	listCursor.One(&persistedRoles)

	return persistedRoles.Roles
}

func (p *Persistency) GetPersistentRoles(guildID string) (persistentRoles []discordgo.Role) {
	persistentRoles = make([]discordgo.Role, 0)
	persistentRoles = append(persistentRoles, p.GetPersistentManagedRoles(guildID)...)
	persistentRoles = append(persistentRoles, p.GetPersistentBiasRoles(guildID)...)
	persistentRoles = append(persistentRoles, p.GetPersistentCustomRoles(guildID)...)

	return
}

func (p *Persistency) GetPersistentManagedRoles(guildID string) (managedRoles []discordgo.Role) {
	managedRoles = make([]discordgo.Role, 0)
	guildSettings := helpers.GuildSettingsGetCached(guildID)
	guild, err := helpers.GetGuild(guildID)
	if err != nil {
		helpers.RelaxLog(err)
		return
	}

	for _, guildRole := range guild.Roles {
		// Look for mute role
		if guildRole.Name == guildSettings.MutedRoleName {
			managedRoles = append(managedRoles, *guildRole)
			continue
		}
	}
	return
}

func (p *Persistency) GetPersistentBiasRoles(guildID string) (biasRoles []discordgo.Role) {
	biasRoles = make([]discordgo.Role, 0)
	guildSettings := helpers.GuildSettingsGetCached(guildID)
	guild, err := helpers.GetGuild(guildID)
	if err != nil {
		helpers.RelaxLog(err)
		return
	}

NextGuildRole:
	for _, guildRole := range guild.Roles {
		// Look for bias roles (if enabled)
		if guildSettings.PersistencyBiasEnabled {
			for _, biasChannel := range biasChannels { // TODO: better access (through cache)
				if biasChannel.ServerID == guildID {
					for _, category := range biasChannel.Categories {
						for _, biasRole := range category.Roles {
							if strings.ToLower(biasRole.Name) == strings.ToLower(guildRole.Name) || biasRole.Name == guildRole.ID {
								biasRoles = append(biasRoles, *guildRole)
								continue NextGuildRole
							}
						}
					}
				}
			}
		}
	}
	return
}

func (p *Persistency) GetPersistentCustomRoles(guildID string) (customRoles []discordgo.Role) {
	customRoles = make([]discordgo.Role, 0)
	guildSettings := helpers.GuildSettingsGetCached(guildID)
	guild, err := helpers.GetGuild(guildID)
	if err != nil {
		helpers.RelaxLog(err)
		return
	}

	for _, guildRole := range guild.Roles {
		// Look for custom roles
		for _, customRoleID := range guildSettings.PersistencyRoleIDs {
			if customRoleID == guildRole.ID {
				customRoles = append(customRoles, *guildRole)
			}
		}
	}
	return
}

func (p *Persistency) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {
	go func() {
		defer helpers.Recover()

		err := p.saveRoleDBCache(member.GuildID, member.User.ID)
		helpers.RelaxLog(err)
	}()
}

func (p *Persistency) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {

}

func (p *Persistency) OnMessageDelete(msg *discordgo.MessageDelete, session *discordgo.Session) {

}

func (p *Persistency) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {

}

func (p *Persistency) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {

}

func (p *Persistency) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {

}

func (p *Persistency) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}
