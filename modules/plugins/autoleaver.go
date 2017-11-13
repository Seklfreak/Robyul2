package plugins

import (
	"errors"
	"strings"

	"fmt"

	"time"

	"bytes"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
	rethink "github.com/gorethink/gorethink"
)

type autoleaverAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next autoleaverAction)

type Autoleaver struct{}

var (
	AutoleaverNotificationChannels = []string{
		"271740860180201473", // sekl's dev cord / #test
		"287175379423068160", // Robyul Cord / #private-noti
	}
)

func (a *Autoleaver) Commands() []string {
	return []string{
		"autoleaver",
	}
}

func (a *Autoleaver) Init(session *discordgo.Session) {
	session.AddHandler(a.OnGuildCreate)
	session.AddHandler(a.OnGuildDelete)
}

func (a *Autoleaver) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	defer helpers.Recover()

	session.ChannelTyping(msg.ChannelID)

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := a.actionStart
	for action != nil {
		action = action(args, msg, &result)
	}
}

func (a *Autoleaver) actionStart(args []string, in *discordgo.Message, out **discordgo.MessageSend) autoleaverAction {
	cache.GetSession().ChannelTyping(in.ChannelID)

	if len(args) < 1 {
		*out = a.newMsg(helpers.GetText("bot.arguments.too-few"))
		return a.actionFinish
	}

	switch args[0] {
	case "add":
		return a.actionAdd
	case "remove":
		return a.actionRemove
	case "check":
		return a.actionCheck
	case "import":
		return a.actionImport
	}

	*out = a.newMsg(helpers.GetText("bot.arguments.invalid"))
	return a.actionFinish
}

func (a *Autoleaver) actionAdd(args []string, in *discordgo.Message, out **discordgo.MessageSend) autoleaverAction {
	if !helpers.IsRobyulMod(in.Author.ID) {
		*out = a.newMsg(helpers.GetText("robyulmod.no_permission"))
		return a.actionFinish
	}

	if len(args) < 2 {
		*out = a.newMsg(helpers.GetText("bot.arguments.too-few"))
		return a.actionFinish
	}

	guildID := args[1]

	if whitelistEntryFound, _ := a.getServerWhitelistEntry(guildID); whitelistEntryFound.ID != "" {
		guildFound, _ := helpers.GetGuild(whitelistEntryFound.GuildID)
		if guildFound == nil || guildFound.ID == "" {
			guildFound = new(discordgo.Guild)
			guildFound.ID = whitelistEntryFound.GuildID
			guildFound.Name = "N/A"
		}

		*out = a.newMsg(helpers.GetTextF("plugins.autoleaver.add-error-duplicate", guildFound.Name, guildFound.ID))
		return a.actionFinish
	}

	whitelistEntry, err := a.addToServerWhitelist(guildID, in.Author.ID)
	helpers.Relax(err)

	guildAdded, _ := helpers.GetGuild(whitelistEntry.GuildID)
	if guildAdded == nil || guildAdded.ID == "" {
		guildAdded = new(discordgo.Guild)
		guildAdded.ID = whitelistEntry.GuildID
		guildAdded.Name = "N/A"
	}

	*out = a.newMsg(helpers.GetTextF("plugins.autoleaver.add-success", guildAdded.Name, guildAdded.ID))
	return a.actionFinish
}

func (a *Autoleaver) actionImport(args []string, in *discordgo.Message, out **discordgo.MessageSend) autoleaverAction {
	if !helpers.IsRobyulMod(in.Author.ID) {
		*out = a.newMsg(helpers.GetText("robyulmod.no_permission"))
		return a.actionFinish
	}

	if len(in.Attachments) < 1 {
		*out = a.newMsg(helpers.GetText("bot.arguments.too-few"))
		return a.actionFinish
	}

	guildIDs := helpers.NetGet(in.Attachments[0].URL)
	guildIDs = bytes.TrimPrefix(guildIDs, []byte("\xef\xbb\xbf")) // removes BOM
	guildIDLines := strings.Split(string(guildIDs), "\n")

	resultText := helpers.GetText("plugins.autoleaver.bulk-title") + "\n"

	var err error
	var guildID string
	var whitelistEntry models.AutoleaverWhitelistEntry
	var guildAdded *discordgo.Guild
	var guildsAdded int
	for _, guildIDLine := range guildIDLines {
		guildID = strings.TrimSpace(strings.Replace(guildIDLine, "\r", "", -1))

		if whitelistEntryFound, _ := a.getServerWhitelistEntry(guildID); whitelistEntryFound.ID != "" {
			guildFound, _ := helpers.GetGuild(whitelistEntryFound.GuildID)
			if guildFound == nil || guildFound.ID == "" {
				guildFound = new(discordgo.Guild)
				guildFound.ID = whitelistEntryFound.GuildID
				guildFound.Name = "N/A"
			}

			resultText += fmt.Sprintf(":white_check_mark: Guild already in Whitelist: %s `(#%s)`\n", guildFound.Name, guildFound.ID)
			continue
		}

		whitelistEntry, err = a.addToServerWhitelist(guildID, in.Author.ID)
		if err != nil {
			resultText += fmt.Sprintf(":x: Error adding Guild `#%s`: %s\n", guildID, err.Error())
			continue
		}

		guildAdded, _ = helpers.GetGuild(whitelistEntry.GuildID)
		if guildAdded == nil || guildAdded.ID == "" {
			guildAdded = new(discordgo.Guild)
			guildAdded.ID = whitelistEntry.GuildID
			guildAdded.Name = "N/A"
		}

		resultText += fmt.Sprintf(":white_check_mark: %s `(#%s)`\n", guildAdded.Name, guildAdded.ID)

		guildsAdded++
	}
	resultText += helpers.GetTextF("plugins.autoleaver.bulk-footer", guildsAdded) + "\n"

	for _, page := range helpers.Pagify(resultText, "\n") {
		_, err = helpers.SendMessage(in.ChannelID, page)
		helpers.RelaxMessage(err, in.ChannelID, in.ID)
	}

	return nil
}

func (a *Autoleaver) actionRemove(args []string, in *discordgo.Message, out **discordgo.MessageSend) autoleaverAction {
	if !helpers.IsRobyulMod(in.Author.ID) {
		*out = a.newMsg(helpers.GetText("robyulmod.no_permission"))
		return a.actionFinish
	}

	if len(args) < 2 {
		*out = a.newMsg(helpers.GetText("bot.arguments.too-few"))
		return a.actionFinish
	}

	guildID := args[1]

	var whitelistEntryFound models.AutoleaverWhitelistEntry
	if whitelistEntryFound, _ = a.getServerWhitelistEntry(guildID); whitelistEntryFound.ID == "" {
		guildFound, _ := helpers.GetGuild(whitelistEntryFound.ID)
		if guildFound == nil || guildFound.ID == "" {
			guildFound = new(discordgo.Guild)
			guildFound.ID = whitelistEntryFound.ID
			guildFound.Name = "N/A"
		}

		*out = a.newMsg(helpers.GetTextF("plugins.autoleaver.remove-error-not-found", guildFound.Name, guildFound.ID))
		return a.actionFinish
	}

	err := a.removeFromServerWhitelist(whitelistEntryFound)
	helpers.Relax(err)

	guildRemoved, _ := helpers.GetGuild(whitelistEntryFound.GuildID)
	if guildRemoved == nil || guildRemoved.ID == "" {
		guildRemoved = new(discordgo.Guild)
		guildRemoved.ID = whitelistEntryFound.GuildID
		guildRemoved.Name = "N/A"
	}

	*out = a.newMsg(helpers.GetTextF("plugins.autoleaver.remove-success", guildRemoved.Name, guildRemoved.ID))
	return a.actionFinish
}

func (a *Autoleaver) actionCheck(args []string, in *discordgo.Message, out **discordgo.MessageSend) autoleaverAction {
	if !helpers.IsRobyulMod(in.Author.ID) {
		*out = a.newMsg(helpers.GetText("robyulmod.no_permission"))
		return a.actionFinish
	}

	whitelistEntries, err := a.getServerWhitelist()
	if err != nil {
		if strings.Contains(err.Error(), "no whitelist entries") {
			*out = a.newMsg(helpers.GetText("plugins.autoleaver.check-no-entries"))
			return a.actionFinish
		}
		helpers.Relax(err)
	}

	notWhitelistedGuilds := make([]*discordgo.Guild, 0)

	var isWhitelisted bool
	for _, botGuild := range cache.GetSession().State.Guilds {
		isWhitelisted, err = a.isOnWhitelist(botGuild.ID, whitelistEntries)
		helpers.Relax(err)

		if !isWhitelisted {
			notWhitelistedGuilds = append(notWhitelistedGuilds, botGuild)
		}
	}

	if len(notWhitelistedGuilds) <= 0 {
		*out = a.newMsg(helpers.GetTextF("plugins.autoleaver.check-no-not-whitelisted", len(cache.GetSession().State.Guilds)))
		return a.actionFinish
	}

	notWhitelistedGuildsMessage := helpers.GetTextF("plugins.autoleaver.check-not-whitelisted-title", len(notWhitelistedGuilds)) + "\n"
	for _, notWhitelistedGuild := range notWhitelistedGuilds {
		notWhitelistedGuildsMessage += fmt.Sprintf("`%s` (`#%s`): Channels `%d`, Members: `%d`, Region: `%s`\n",
			notWhitelistedGuild.Name, notWhitelistedGuild.ID, len(notWhitelistedGuild.Channels), len(notWhitelistedGuild.Members), notWhitelistedGuild.Region)
	}
	notWhitelistedGuildsMessage += helpers.GetTextF("plugins.autoleaver.check-not-whitelisted-footer", len(notWhitelistedGuilds), len(cache.GetSession().State.Guilds)) + "\n"

	*out = a.newMsg(notWhitelistedGuildsMessage)
	return a.actionFinish
}

func (a *Autoleaver) isOnWhitelist(GuildID string, whitelist []models.AutoleaverWhitelistEntry) (bool, error) {
	var err error
	if whitelist == nil {
		whitelist, err = a.getServerWhitelist()
		if err != nil {
			return true, err
		}
	}

	for _, whitelistEntry := range whitelist {
		if whitelistEntry.GuildID == GuildID {
			return true, nil
		}
	}

	return false, nil
}

func (a *Autoleaver) getServerWhitelist() (entryBucket []models.AutoleaverWhitelistEntry, err error) {
	listCursor, err := rethink.Table(models.AutoleaverWhitelistTable).Run(helpers.GetDB())
	if err != nil {
		return entryBucket, err
	}

	defer listCursor.Close()
	err = listCursor.All(&entryBucket)
	if err == rethink.ErrEmptyResult {
		return entryBucket, errors.New("no whitelist entries")
	} else if err != nil {
		return entryBucket, err
	}

	return entryBucket, nil
}

func (a *Autoleaver) getServerWhitelistEntry(guildID string) (entryBucket models.AutoleaverWhitelistEntry, err error) {
	listCursor, err := rethink.Table(models.AutoleaverWhitelistTable).Filter(
		rethink.Row.Field("guild_id").Eq(guildID),
	).Run(helpers.GetDB())
	if err != nil {
		return entryBucket, err
	}
	defer listCursor.Close()
	err = listCursor.One(&entryBucket)

	if err == rethink.ErrEmptyResult {
		return entryBucket, errors.New("no whitelist entry")
	} else if err != nil {
		return entryBucket, err
	}

	return entryBucket, nil
}

func (a *Autoleaver) addToServerWhitelist(guildID string, userID string) (models.AutoleaverWhitelistEntry, error) {
	insert := rethink.Table(models.AutoleaverWhitelistTable).Insert(models.AutoleaverWhitelistEntry{
		AddedAt:       time.Now(),
		AddedByUserID: userID,
		GuildID:       guildID,
	})
	_, err := insert.RunWrite(helpers.GetDB())
	if err != nil {
		return models.AutoleaverWhitelistEntry{}, err
	} else {
		return a.getServerWhitelistEntry(guildID)
	}
}

func (a *Autoleaver) removeFromServerWhitelist(whitelistEntry models.AutoleaverWhitelistEntry) error {
	if whitelistEntry.ID != "" {
		_, err := rethink.Table(models.AutoleaverWhitelistTable).Get(whitelistEntry.ID).Delete().RunWrite(helpers.GetDB())
		return err
	}
	return errors.New("empty whitelistEntry submitted")
}

func (a *Autoleaver) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) autoleaverAction {
	_, err := helpers.SendComplex(in.ChannelID, *out)
	helpers.Relax(err)

	return nil
}

func (a *Autoleaver) newMsg(content string) *discordgo.MessageSend {
	return &discordgo.MessageSend{Content: helpers.GetText(content)}
}

func (a *Autoleaver) Relax(err error) {
	if err != nil {
		panic(err)
	}
}

func (a *Autoleaver) logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "autoleaver")
}

func (a *Autoleaver) OnGuildCreate(session *discordgo.Session, guild *discordgo.GuildCreate) {
	go func() {
		defer helpers.Recover()

		// don't continue if bot didn't just join this guild
		if !cache.AddAutoleaverGuildID(guild.ID) {
			return
		}

		onWhitelist, err := a.isOnWhitelist(guild.ID, nil)
		helpers.Relax(err)

		owner, err := helpers.GetUser(guild.OwnerID)
		ownerName := "N/A"
		if err != nil {
			owner = new(discordgo.User)
		} else {
			ownerName = owner.Username + "#" + owner.Discriminator
		}
		membersCount := guild.MemberCount
		if len(guild.Members) > membersCount {
			membersCount = len(guild.Members)
		}

		joinText := helpers.GetTextF("plugins.autoleaver.noti-join", guild.Name, guild.ID, ownerName, guild.OwnerID, membersCount)
		for _, notificationChannelID := range AutoleaverNotificationChannels {
			_, err = helpers.SendMessage(notificationChannelID, joinText)
			if err != nil {
				a.logger().WithField("GuildID", guild.ID).Error(fmt.Sprintf("Join Notification failed, Error: %s", err.Error()))
			}
		}

		if onWhitelist {
			return
		}

		notWhitelistedJoinText := helpers.GetTextF("plugins.autoleaver.noti-join-not-whitelisted", guild.Name, guild.ID)
		for _, notificationChannelID := range AutoleaverNotificationChannels {
			_, err = helpers.SendMessage(notificationChannelID, notWhitelistedJoinText)
			if err != nil {
				a.logger().WithField("GuildID", guild.ID).Error(fmt.Sprintf("Not Whitelisted Join Notification failed, Error: %s", err.Error()))
			}
		}

		err = cache.GetSession().GuildLeave(guild.ID)
		helpers.Relax(err)
	}()
}

func (a *Autoleaver) OnGuildDelete(session *discordgo.Session, guild *discordgo.GuildDelete) {
	go func() {
		defer helpers.Recover()

		var err error

		owner, err := helpers.GetUser(guild.OwnerID)
		ownerName := "N/A"
		if err != nil {
			owner = new(discordgo.User)
		} else {
			ownerName = owner.Username + "#" + owner.Discriminator
		}

		joinText := helpers.GetTextF("plugins.autoleaver.noti-leave", guild.Name, guild.ID, ownerName, guild.OwnerID)
		for _, notificationChannelID := range AutoleaverNotificationChannels {
			_, err = helpers.SendMessage(notificationChannelID, joinText)
			if err != nil {
				a.logger().WithField("GuildID", guild.ID).Error(fmt.Sprintf("Leave Notification failed, Error: %s", err.Error()))
			}
		}
		cache.RemoveAutoleaverGuildID(guild.ID)
	}()
}

func (a *Autoleaver) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {

}

func (a *Autoleaver) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {

}

func (a *Autoleaver) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {

}

func (a *Autoleaver) OnMessageDelete(msg *discordgo.MessageDelete, session *discordgo.Session) {

}

func (a *Autoleaver) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {

}

func (a *Autoleaver) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {

}

func (a *Autoleaver) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {

}

func (a *Autoleaver) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}
