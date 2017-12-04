package plugins

import (
	"errors"
	"strings"

	"time"

	"sync"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	rethink "github.com/gorethink/gorethink"
	"github.com/sirupsen/logrus"
)

type namesAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next namesAction)

type Names struct{}

var (
	previousNicknames      map[string]map[string]string
	previousNicknamesMutex sync.RWMutex
	previousUsernames      map[string]string
	previousUsernamesMutex sync.RWMutex
)

func (n *Names) Commands() []string {
	return []string{
		"names",
		"nicknames",
	}
}

func (n *Names) Init(session *discordgo.Session) {
	previousNicknamesMutex.Lock()
	previousNicknames = make(map[string]map[string]string, 0)
	previousNicknamesMutex.Unlock()
	previousUsernamesMutex.Lock()
	previousUsernames = make(map[string]string, 0)
	previousUsernamesMutex.Unlock()
	session.AddHandler(n.OnGuildMemberListChunk)
	session.AddHandler(n.OnPresenceUpdate)
	session.AddHandler(n.OnGuildMemberUpdate)
}

func (n *Names) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	defer helpers.Recover()

	session.ChannelTyping(msg.ChannelID)

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := n.actionStart
	for action != nil {
		action = action(args, msg, &result)
	}
}

func (n *Names) actionStart(args []string, in *discordgo.Message, out **discordgo.MessageSend) namesAction {
	cache.GetSession().ChannelTyping(in.ChannelID)

	if len(args) < 1 {
		*out = n.newMsg(helpers.GetText("bot.arguments.too-few"))
		return n.actionFinish
	}

	return n.actionNames
}

func (n *Names) actionNames(args []string, in *discordgo.Message, out **discordgo.MessageSend) namesAction {
	user, err := helpers.GetUserFromMention(args[0])
	if err != nil || user == nil || user.ID == "" {
		*out = n.newMsg(helpers.GetText("bot.arguments.invalid"))
		return n.actionFinish
	}
	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)
	member, _ := helpers.GetGuildMember(channel.GuildID, user.ID)

	var pastUsernamesText, pastNicknamesText string

	pastUsernames, err := n.GetUsernames(user.ID)
	if err != nil && strings.Contains(err.Error(), "no username entries") {
		helpers.Relax(err)
	}

	if len(pastUsernames) <= 0 || pastUsernames[len(pastUsernames)-1] != user.Username+"#"+user.Discriminator {
		pastUsernames = append(pastUsernames, user.Username+"#"+user.Discriminator)
	}

	for i, pastUsername := range pastUsernames {
		pastUsernamesText += "`" + pastUsername + "`"
		if i < len(pastUsernames)-1 {
			pastUsernamesText += ", "
		}
		if i == len(pastUsernames) {
			pastUsernamesText += " and "
		}
	}

	if pastUsernamesText == "" {
		pastUsernamesText = "None"
	}

	pastNicknames, err := n.GetNicknames(channel.GuildID, user.ID)
	if err != nil && strings.Contains(err.Error(), "no nickname entries") {
		helpers.Relax(err)
	}

	if member != nil && member.User != nil && member.User.ID != "" {
		if member.Nick != "" {
			if len(pastNicknames) <= 0 || pastNicknames[len(pastNicknames)-1] != member.Nick {
				pastNicknames = append(pastNicknames, member.Nick)
			}
		}
	}

	for i, pastNickname := range pastNicknames {
		pastNicknamesText += "`" + pastNickname + "`"
		if i < len(pastNicknames)-1 {
			pastNicknamesText += ", "
		}
		if i == len(pastNicknames) {
			pastNicknamesText += " and "
		}
	}

	if pastNicknamesText == "" {
		pastNicknamesText = "None"
	}

	resultText := helpers.GetTextF("plugins.names.list-result",
		user.Username, user.Discriminator, user.ID, pastUsernamesText, pastNicknamesText)
	for _, page := range helpers.Pagify(resultText, ",") {
		_, err := helpers.SendMessage(in.ChannelID, page)
		helpers.RelaxMessage(err, in.ChannelID, in.ID)
	}

	return nil
}

func (n *Names) OnPresenceUpdate(session *discordgo.Session, presence *discordgo.PresenceUpdate) {
	if presence.GuildID == "" || presence.User == nil || presence.User.ID == "" {
		return
	}

	go func() {
		defer helpers.Recover()
		if presence.Presence.User.Username != "" {
			err := n.UpdateUsername(presence.Presence.User.ID, presence.Presence.User.Username+"#"+presence.Presence.User.Discriminator)
			helpers.Relax(err)
		}
	}()
}

func (n *Names) OnGuildMemberUpdate(session *discordgo.Session, member *discordgo.GuildMemberUpdate) {
	if member.Member == nil {
		return
	}

	go func() {
		defer helpers.Recover()

		if member.Member.Nick != "" {
			err := n.UpdateNickname(member.Member.GuildID, member.Member.User.ID, member.Member.Nick)
			helpers.Relax(err)
		}
	}()
}

func (n *Names) UpdateNickname(guildID string, userID string, newNick string) (err error) {
	previousNicknamesMutex.Lock()
	defer previousNicknamesMutex.Unlock()
	var oldNick string
	if previousNicknames[guildID] == nil {
		previousNicknames[guildID] = make(map[string]string, 0)
	}
	oldNick, _ = previousNicknames[guildID][userID]

	lastSavedNickname, err := n.GetLastNickname(guildID, userID)
	if err != nil && !strings.Contains(err.Error(), "no nickname entry") {
		helpers.RelaxLog(err)
	}

	if oldNick != "" && lastSavedNickname != oldNick {
		err = n.SaveNickname(guildID, userID, oldNick)
		helpers.RelaxLog(err)
		//n.logger().WithField("guildID", guildID).WithField("userID", userID).Debug(
		//	"saved old nickname: ", oldNick) // TODO
		lastSavedNickname = oldNick
	}

	if lastSavedNickname != newNick {
		err = n.SaveNickname(guildID, userID, newNick)
		//n.logger().WithField("guildID", guildID).WithField("userID", userID).Debug(
		//	"saved new nickname: ", newNick) // TODO
		previousNicknames[guildID][userID] = newNick
		return err
	}
	return nil
}

func (n *Names) UpdateUsername(userID string, newUsername string) (err error) {
	previousUsernamesMutex.Lock()
	defer previousUsernamesMutex.Unlock()
	var oldUsername string
	oldUsername, _ = previousUsernames[userID]

	lastSavedUsername, err := n.GetLastUsername(userID)
	if err != nil && !strings.Contains(err.Error(), "no username entry") {
		helpers.RelaxLog(err)
	}

	if oldUsername != "" && lastSavedUsername != oldUsername {
		err = n.SaveUsername(userID, oldUsername)
		helpers.RelaxLog(err)
		//n.logger().WithField("userID", userID).Debug(
		//	"saved old username: ", oldUsername) // TODO
		lastSavedUsername = oldUsername
	}

	if lastSavedUsername != newUsername {
		err = n.SaveUsername(userID, newUsername)
		//n.logger().WithField("userID", userID).Debug(
		//	"saved new username: ", newUsername) // TODO
		previousUsernames[userID] = newUsername
		return err
	}
	return nil
}

func (n *Names) GetLastNickname(guildID string, userID string) (nickname string, err error) {
	var entryBucket models.NamesEntry
	listCursor, err := rethink.Table(models.NamesTable).GetAllByIndex(
		"user_id", userID,
	).Filter(
		rethink.Row.Field("guild_id").Eq(guildID),
	).OrderBy(rethink.Desc("changed_at")).Limit(1).Run(helpers.GetDB())
	if err != nil {
		return "", err
	}
	defer listCursor.Close()
	err = listCursor.One(&entryBucket)

	if err == rethink.ErrEmptyResult {
		return "", errors.New("no nickname entry")
	} else if err != nil {
		return "", err
	}
	return entryBucket.Nickname, nil
}

func (n *Names) SaveNickname(guildID string, userID string, nickname string) (err error) {
	insert := rethink.Table(models.NamesTable).Insert(models.NamesEntry{
		ChangedAt: time.Now(),
		GuildID:   guildID,
		UserID:    userID,
		Nickname:  nickname,
		Username:  "",
	})
	_, err = insert.RunWrite(helpers.GetDB())
	return err
}

func (n *Names) GetLastUsername(userID string) (username string, err error) {
	var entryBucket models.NamesEntry
	listCursor, err := rethink.Table(models.NamesTable).GetAllByIndex(
		"user_id", userID,
	).Filter(
		rethink.Row.Field("guild_id").Eq("global"),
	).OrderBy(rethink.Desc("changed_at")).Limit(1).Run(helpers.GetDB())
	if err != nil {
		return "", err
	}
	defer listCursor.Close()
	err = listCursor.One(&entryBucket)

	if err == rethink.ErrEmptyResult {
		return "", errors.New("no username entry")
	} else if err != nil {
		return "", err
	}
	return entryBucket.Username, nil
}

func (n *Names) GetNicknames(guildID string, userID string) (nicknames []string, err error) {
	var entryBucket []models.NamesEntry
	listCursor, err := rethink.Table(models.NamesTable).GetAllByIndex(
		"user_id", userID,
	).Filter(
		rethink.Row.Field("guild_id").Eq(guildID),
	).OrderBy(rethink.Asc("changed_at")).Run(helpers.GetDB())
	if err != nil {
		return nicknames, err
	}
	defer listCursor.Close()
	err = listCursor.All(&entryBucket)

	if err == rethink.ErrEmptyResult {
		return nicknames, errors.New("no nickname entries")
	} else if err != nil {
		return nicknames, err
	}
	for _, entry := range entryBucket {
		nicknames = append(nicknames, entry.Nickname)
	}
	return nicknames, nil
}

func (n *Names) GetUsernames(userID string) (usernames []string, err error) {
	var entryBucket []models.NamesEntry
	listCursor, err := rethink.Table(models.NamesTable).GetAllByIndex(
		"user_id", userID,
	).Filter(
		rethink.Row.Field("guild_id").Eq("global"),
	).OrderBy(rethink.Asc("changed_at")).Run(helpers.GetDB())
	if err != nil {
		return usernames, err
	}
	defer listCursor.Close()
	err = listCursor.All(&entryBucket)

	if err == rethink.ErrEmptyResult {
		return usernames, errors.New("no username entries")
	} else if err != nil {
		return usernames, err
	}
	for _, entry := range entryBucket {
		usernames = append(usernames, entry.Username)
	}
	return usernames, nil
}

func (n *Names) SaveUsername(userID string, username string) (err error) {
	insert := rethink.Table(models.NamesTable).Insert(models.NamesEntry{
		ChangedAt: time.Now(),
		GuildID:   "global",
		UserID:    userID,
		Nickname:  "",
		Username:  username,
	})
	_, err = insert.RunWrite(helpers.GetDB())
	return err
}

func (n *Names) OnGuildMemberListChunk(session *discordgo.Session, members *discordgo.GuildMembersChunk) {
	previousUsernamesMutex.Lock()
	previousNicknamesMutex.Lock()
	defer previousUsernamesMutex.Unlock()
	defer previousNicknamesMutex.Unlock()
	for _, member := range members.Members {
		previousUsernames[member.User.ID] = member.User.Username + "#" + member.User.Discriminator
		if member.Nick != "" {
			if previousNicknames[members.GuildID] == nil {
				previousNicknames[members.GuildID] = make(map[string]string, 0)
			}
			previousNicknames[members.GuildID][member.User.ID] = member.Nick
		}
	}
}

func (n *Names) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) namesAction {
	_, err := helpers.SendComplex(in.ChannelID, *out)
	helpers.Relax(err)

	return nil
}

func (n *Names) newMsg(content string) *discordgo.MessageSend {
	return &discordgo.MessageSend{Content: helpers.GetText(content)}
}

func (n *Names) Relax(err error) {
	if err != nil {
		panic(err)
	}
}

func (n *Names) logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "names")
}
