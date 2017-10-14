package plugins

import (
	"strings"

	"fmt"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
	rethink "github.com/gorethink/gorethink"
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

func (p *Persistency) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	defer helpers.Recover()

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

	*out = p.newMsg(helpers.GetText("bot.arguments.invalid"))
	return p.actionFinish
}

func (p *Persistency) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) PersistencyAction {
	_, err := cache.GetSession().ChannelMessageSendComplex(in.ChannelID, *out)
	helpers.Relax(err)

	return nil
}

func (p *Persistency) newMsg(content string) *discordgo.MessageSend {
	return &discordgo.MessageSend{Content: helpers.GetText(content)}
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

		for _, roleToApply := range rolesToApply {
			err := session.GuildMemberRoleAdd(member.GuildID, member.User.ID, roleToApply.ID)
			if err != nil {
				helpers.RelaxLog(err)
			} else {
				successfullyApplied++
			}
		}

		p.logger().WithField("UserID", member.User.ID).Debug(fmt.Sprintf("applied roles on join: %d/%d/%d (applied/found/cached)",
			successfullyApplied, len(rolesToApply), len(cachedRoles)))
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
	guildSettings := helpers.GuildSettingsGetCached(guildID)
	guild, err := helpers.GetGuild(guildID)
	if err != nil {
		helpers.RelaxLog(err)
		return
	}

	for _, guildRole := range guild.Roles {
		if guildRole.Name == guildSettings.MutedRoleName {
			persistentRoles = append(persistentRoles, *guildRole)
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
