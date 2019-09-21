package plugins

import (
	"strings"

	"fmt"

	"bytes"

	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Seklfreak/Robyul2/shardmanager"
	"github.com/bwmarrin/discordgo"
	"github.com/globalsign/mgo/bson"
	"github.com/karrick/tparse/v2"
	"github.com/sirupsen/logrus"
)

var whitelistMods = []string{
	"257900648618655746", // mali
	"460594042007453697", // rachel
}

func isWhitelistMod(userID string) bool {
	if helpers.IsRobyulMod(userID) {
		return true
	}

	for _, whitelistModID := range whitelistMods {
		if whitelistModID == userID {
			return true
		}
	}

	return false
}

type autoleaverAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next autoleaverAction)

type Autoleaver struct{}

func (a *Autoleaver) Commands() []string {
	return []string{
		"autoleaver",
	}
}

func (a *Autoleaver) Init(session *shardmanager.Manager) {
	session.AddHandler(a.OnGuildCreate)
	session.AddHandler(a.OnGuildDelete)

	go func() {
		defer helpers.Recover()
		a.checkExpiredGuildsLoop()
	}()
}

func (a *Autoleaver) Uninit(session *shardmanager.Manager) {

}

func (a *Autoleaver) checkExpiredGuildsLoop() {
	defer helpers.Recover()
	defer func() {
		go func() {
			defer helpers.Recover()
			a.logger().Error("The checkExpiredGuildsLoop died. Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			a.checkExpiredGuildsLoop()
		}()
	}()

	var err error
	for {
		time.Sleep(5 * time.Second)

		err = a.removeExpiredGuilds()
		helpers.RelaxLog(err)
	}

}

func (a *Autoleaver) removeExpiredGuilds() error {
	var entryBucket []models.AutoleaverWhitelistEntry
	err := helpers.MDbIter(helpers.MdbCollection(models.AutoleaverWhitelistTable).Find(nil)).All(&entryBucket)
	if err != nil {
		return err
	}

	// check dates, remove if expired
	for _, entry := range entryBucket {
		if entry.Until.IsZero() {
			continue
		}

		if time.Now().Before(entry.Until) {
			continue
		}

		guildFound, _ := helpers.GetGuild(entry.GuildID)
		if guildFound == nil || guildFound.ID == "" {
			guildFound = new(discordgo.Guild)
			guildFound.ID = entry.GuildID
			guildFound.Name = "N/A"
		}

		err = helpers.MDbDelete(models.AutoleaverWhitelistTable, entry.ID)
		if err != nil {
			return err
		}

		expiredTet := helpers.GetTextF("plugins.autoleaver.noti-expired", guildFound.Name, guildFound.ID)

		notificationChannelID, _ := helpers.GetBotConfigString(models.AutoleaverLogChannelKey)
		if notificationChannelID != "" {
			_, err = helpers.SendMessage(notificationChannelID, expiredTet)
			if err != nil {
				a.logger().WithField("GuildID", guildFound.ID).Errorf("Expired Notification failed, Error: %s", err.Error())
			}
		}

		a.logger().Infoln("removed #" + entry.GuildID + " from whitelist because it expired")

		// TODO: leave guild
		// TODO: DM, after a specified percentage of creation date until deadline date
	}

	return nil
}

func (a *Autoleaver) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	session.ChannelTyping(msg.ChannelID)

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := a.actionStart
	for action != nil {
		action = action(args, msg, &result)
	}
}

func (a *Autoleaver) actionStart(args []string, in *discordgo.Message, out **discordgo.MessageSend) autoleaverAction {
	cache.GetSession().SessionForGuildS(in.GuildID).ChannelTyping(in.ChannelID)

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
	case "set-log":
		return a.actionSetLog
	}

	*out = a.newMsg(helpers.GetText("bot.arguments.invalid"))
	return a.actionFinish
}

func (a *Autoleaver) actionAdd(args []string, in *discordgo.Message, out **discordgo.MessageSend) autoleaverAction {
	if !isWhitelistMod(in.Author.ID) {
		*out = a.newMsg(helpers.GetText("robyulmod.no_permission"))
		return a.actionFinish
	}

	if len(args) < 2 {
		*out = a.newMsg(helpers.GetText("bot.arguments.too-few"))
		return a.actionFinish
	}

	guildID := args[1]

	if !helpers.IsSnowflake(guildID) {
		inviteCodes := helpers.ExtractInviteCodes(guildID)
		if len(inviteCodes) < 1 {
			inviteCodes = append(inviteCodes, guildID)
		}
		invite, err := cache.GetSession().SessionForGuildS(in.GuildID).Invite(inviteCodes[0])
		if err == nil && invite != nil && invite.Guild != nil && invite.Guild.ID != "" {
			guildID = invite.Guild.ID
		} else {
			*out = a.newMsg(helpers.GetText("bot.arguments.invalid"))
			return a.actionFinish
		}
	}

	// temporary whitelisting if requested
	var err error
	var until time.Time
	if len(args) >= 3 {
		until, err = tparse.AddDuration(time.Now(), args[2])
		if err != nil {
			*out = a.newMsg("bot.arguments.invalid")
			return a.actionFinish
		}
	}

	var entryBucket models.AutoleaverWhitelistEntry
	err = helpers.MdbOne(
		helpers.MdbCollection(models.AutoleaverWhitelistTable).Find(bson.M{"guildid": guildID}),
		&entryBucket,
	)
	if err == nil {
		guildFound, _ := helpers.GetGuild(guildID)
		if guildFound == nil || guildFound.ID == "" {
			guildFound = new(discordgo.Guild)
			guildFound.ID = guildID
			guildFound.Name = "N/A"
		}

		if entryBucket.Until.IsZero() {
			*out = a.newMsg(helpers.GetTextF("plugins.autoleaver.add-error-duplicate", guildFound.Name, guildFound.ID))
			return a.actionFinish
		}
	}
	if err != nil && !helpers.IsMdbNotFound(err) {
		helpers.Relax(err)
	}

	err = helpers.MDbUpsert(
		models.AutoleaverWhitelistTable,
		bson.M{"guildid": guildID},
		models.AutoleaverWhitelistEntry{
			AddedAt:       time.Now(),
			GuildID:       guildID,
			AddedByUserID: in.Author.ID,
			Until:         until,
		},
	)

	guildAdded, _ := helpers.GetGuild(guildID)
	if guildAdded == nil || guildAdded.ID == "" {
		guildAdded = new(discordgo.Guild)
		guildAdded.ID = guildID
		guildAdded.Name = "N/A"
	}

	message := helpers.GetTextF("plugins.autoleaver.add-success", guildAdded.Name, guildAdded.ID)
	if !until.IsZero() {
		message += "\nWhitelisted until " + until.Format(time.ANSIC)
	}

	*out = a.newMsg(message)
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
	var guildAdded *discordgo.Guild
	var guildsAdded int
	var entryBucket models.AutoleaverWhitelistEntry
	for _, guildIDLine := range guildIDLines {
		guildID = strings.TrimSpace(strings.Replace(guildIDLine, "\r", "", -1))

		err = helpers.MdbOne(
			helpers.MdbCollection(models.AutoleaverWhitelistTable).Find(bson.M{"guildid": guildID}),
			&entryBucket,
		)
		if err == nil {
			guildFound, _ := helpers.GetGuild(guildID)
			if guildFound == nil || guildFound.ID == "" {
				guildFound = new(discordgo.Guild)
				guildFound.ID = guildID
				guildFound.Name = "N/A"
			}

			resultText += fmt.Sprintf(":white_check_mark: Guild already in Whitelist: %s `(#%s)`\n", guildFound.Name, guildFound.ID)
			continue
		}

		err = helpers.MDbUpsert(
			models.AutoleaverWhitelistTable,
			bson.M{"guildid": guildID},
			models.AutoleaverWhitelistEntry{
				AddedAt:       time.Now(),
				GuildID:       guildID,
				AddedByUserID: in.Author.ID,
			},
		)
		if err != nil {
			resultText += fmt.Sprintf(":x: Error adding Guild `#%s`: %s\n", guildID, err.Error())
			continue
		}

		guildAdded, _ = helpers.GetGuild(guildID)
		if guildAdded == nil || guildAdded.ID == "" {
			guildAdded = new(discordgo.Guild)
			guildAdded.ID = guildID
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
	if !isWhitelistMod(in.Author.ID) {
		*out = a.newMsg(helpers.GetText("robyulmod.no_permission"))
		return a.actionFinish
	}

	if len(args) < 2 {
		*out = a.newMsg(helpers.GetText("bot.arguments.too-few"))
		return a.actionFinish
	}

	guildID := args[1]

	var entryBucket models.AutoleaverWhitelistEntry
	err := helpers.MdbOne(
		helpers.MdbCollection(models.AutoleaverWhitelistTable).Find(bson.M{"guildid": guildID}),
		&entryBucket,
	)
	if err != nil {
		if !helpers.IsMdbNotFound(err) {
			helpers.Relax(err)
		}

		guildFound, _ := helpers.GetGuild(guildID)
		if guildFound == nil || guildFound.ID == "" {
			guildFound = new(discordgo.Guild)
			guildFound.ID = guildID
			guildFound.Name = "N/A"
		}

		*out = a.newMsg(helpers.GetTextF("plugins.autoleaver.remove-error-not-found", guildFound.Name, guildFound.ID))
		return a.actionFinish
	}

	err = helpers.MDbDelete(models.AutoleaverWhitelistTable, entryBucket.ID)
	helpers.Relax(err)

	guildRemoved, _ := helpers.GetGuild(guildID)
	if guildRemoved == nil || guildRemoved.ID == "" {
		guildRemoved = new(discordgo.Guild)
		guildRemoved.ID = guildID
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

	err := a.removeExpiredGuilds()
	helpers.Relax(err)

	var entryBucket []models.AutoleaverWhitelistEntry
	err = helpers.MDbIter(helpers.MdbCollection(models.AutoleaverWhitelistTable).Find(nil)).All(&entryBucket)
	helpers.Relax(err)
	if entryBucket == nil || len(entryBucket) < 1 {
		*out = a.newMsg(helpers.GetText("plugins.autoleaver.check-no-entries"))
		return a.actionFinish
	}

	notWhitelistedGuilds := make([]*discordgo.Guild, 0)

	var isWhitelisted bool
	for _, shard := range cache.GetSession().Sessions {
		for _, botGuild := range shard.State.Guilds {
			isWhitelisted, err = a.isOnWhitelist(botGuild.ID, entryBucket)
			helpers.Relax(err)

			if !isWhitelisted {
				notWhitelistedGuilds = append(notWhitelistedGuilds, botGuild)
			}
		}
	}

	status := cache.GetSession().GetFullStatus()

	if len(notWhitelistedGuilds) <= 0 {
		*out = a.newMsg(helpers.GetTextF("plugins.autoleaver.check-no-not-whitelisted", status.NumGuilds))
		return a.actionFinish
	}

	notWhitelistedGuildsMessage := helpers.GetTextF("plugins.autoleaver.check-not-whitelisted-title", len(notWhitelistedGuilds)) + "\n"
	for _, notWhitelistedGuild := range notWhitelistedGuilds {
		notWhitelistedGuildsMessage += fmt.Sprintf("`%s` (`#%s`): Channels `%d`, Members: `%d`, Region: `%s`\n",
			notWhitelistedGuild.Name, notWhitelistedGuild.ID, len(notWhitelistedGuild.Channels), len(notWhitelistedGuild.Members), notWhitelistedGuild.Region)
	}
	notWhitelistedGuildsMessage += helpers.GetTextF("plugins.autoleaver.check-not-whitelisted-footer", len(notWhitelistedGuilds), status.NumGuilds) + "\n"

	*out = a.newMsg(notWhitelistedGuildsMessage)
	return a.actionFinish
}

// [p]autoleaver set-log <#channel or channel id>
func (a *Autoleaver) actionSetLog(args []string, in *discordgo.Message, out **discordgo.MessageSend) autoleaverAction {
	if !helpers.IsRobyulMod(in.Author.ID) {
		*out = a.newMsg("robyulmod.no_permission")
		return a.actionFinish
	}

	var err error
	var targetChannel *discordgo.Channel
	if len(args) >= 2 {
		targetChannel, err = helpers.GetChannelFromMention(in, args[1])
		helpers.Relax(err)
	}

	if targetChannel != nil && targetChannel.ID != "" {
		err = helpers.SetBotConfigString(models.AutoleaverLogChannelKey, targetChannel.ID)
	} else {
		err = helpers.SetBotConfigString(models.AutoleaverLogChannelKey, "")
	}

	*out = a.newMsg("plugins.autoleaver.setlog-success")
	return a.actionFinish
}

func (a *Autoleaver) isOnWhitelist(GuildID string, whitelist []models.AutoleaverWhitelistEntry) (bool, error) {
	var err error
	if whitelist == nil {
		err = helpers.MDbIter(helpers.MdbCollection(models.AutoleaverWhitelistTable).Find(nil)).All(&whitelist)
		if err != nil {
			return true, err
		}
	}

	if whitelist != nil && len(whitelist) > 0 {
		for _, whitelistEntry := range whitelist {
			if whitelistEntry.GuildID == GuildID {
				return true, nil
			}
		}
	}

	return false, nil
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

		// go helpers.UpdateBotlists()

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

		notificationChannelID, _ := helpers.GetBotConfigString(models.AutoleaverLogChannelKey)
		if notificationChannelID != "" {
			_, err = helpers.SendMessage(notificationChannelID, joinText)
			if err != nil {
				a.logger().WithField("GuildID", guild.ID).Errorf("Join Notification failed, Error: %s", err.Error())
			}
		}

		_ = onWhitelist

		// if onWhitelist {
		// 	err = a.sendAllowedJoinMessage(guild.ID)
		// 	helpers.RelaxLog(err)
		// 	return
		// }

		notWhitelistedJoinText := helpers.GetTextF("plugins.autoleaver.noti-join-not-whitelisted", guild.Name, guild.ID)
		if notificationChannelID != "" {
			_, err = helpers.SendMessage(notificationChannelID, notWhitelistedJoinText)
			if err != nil {
				a.logger().WithField("GuildID", guild.ID).Errorf("Not Whitelisted Join Notification failed, Error: %s", err.Error())
			}
		}

		// send message to inform before leaving
		err = a.sendAutoleaveMessage(guild.ID)
		helpers.RelaxLog(err)

		// TODO: enable me!
		// err = cache.GetSession().GuildLeave(guild.ID)
		// helpers.Relax(err)
	}()
}

func (a *Autoleaver) sendAutoleaveMessage(guildID string) (err error) {
	targetChannelID, err := helpers.GetGuildDefaultChannel(guildID)
	if err == nil {
		helpers.SendMessage(targetChannelID, helpers.GetText("plugins.autoleaver.non-whitelisted-leave-message"))
		return nil
	}

	return nil
}

func (a *Autoleaver) sendAllowedJoinMessage(guildID string) (err error) {
	targetChannelID, err := helpers.GetGuildDefaultChannel(guildID)
	if err == nil {
		helpers.SendMessage(targetChannelID, helpers.GetTextF("plugins.autoleaver.yes-whitelisted-join-message", guildID))
		return nil
	}

	return nil
}

func (a *Autoleaver) OnGuildDelete(session *discordgo.Session, guild *discordgo.GuildDelete) {
	go func() {
		defer helpers.Recover()

		// go helpers.UpdateBotlists()

		var err error

		owner, err := helpers.GetUser(guild.OwnerID)
		ownerName := "N/A"
		if err != nil {
			owner = new(discordgo.User)
		} else {
			ownerName = owner.Username + "#" + owner.Discriminator
		}

		joinText := helpers.GetTextF("plugins.autoleaver.noti-leave", guild.Name, guild.ID, ownerName, guild.OwnerID)
		notificationChannelID, _ := helpers.GetBotConfigString(models.AutoleaverLogChannelKey)
		if notificationChannelID != "" {
			_, err = helpers.SendMessage(notificationChannelID, joinText)
			if err != nil {
				a.logger().WithField("GuildID", guild.ID).Errorf("Leave Notification failed, Error: %s", err.Error())
			}
		}

		// remove guild from autoleaver Guilds, if not just leaving because unavailability
		if guild.OwnerID != "" {
			cache.RemoveAutoleaverGuildID(guild.ID)
		}
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
