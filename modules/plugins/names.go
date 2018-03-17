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
	"github.com/globalsign/mgo/bson"
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

// TODO: switch to robyul state

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
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermNames) {
		return
	}

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
	if err != nil && !strings.Contains(err.Error(), "no nickname entries") {
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
		lastSavedNickname = oldNick
	}

	if lastSavedNickname != newNick {
		err = n.SaveNickname(guildID, userID, newNick)
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
		lastSavedUsername = oldUsername
	}

	if lastSavedUsername != newUsername {
		err = n.SaveUsername(userID, newUsername)
		previousUsernames[userID] = newUsername
		return err
	}
	return nil
}

func (n *Names) GetLastNickname(guildID string, userID string) (nickname string, err error) {
	var entryBucket models.NamesEntry
	err = helpers.MdbOne(
		helpers.MdbCollection(models.NamesTable).Find(bson.M{"userid": userID, "guildid": guildID}).Sort("-changedat"),
		&entryBucket,
	)

	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return "", errors.New("no nickname entry")
		}
		return "", err
	}

	return entryBucket.Nickname, nil
}

func (n *Names) SaveNickname(guildID string, userID string, nickname string) (err error) {
	// don't store duplicates
	lastNickname, err := n.GetLastNickname(guildID, userID)
	if err == nil {
		if nickname == lastNickname {
			return nil
		}
	}
	// insert nickname
	_, err = helpers.MDbInsert(
		models.NamesTable,
		models.NamesEntry{
			ChangedAt: time.Now(),
			GuildID:   guildID,
			UserID:    userID,
			Nickname:  nickname,
			Username:  "",
		},
	)
	return err
}

func (n *Names) GetLastUsername(userID string) (username string, err error) {
	var entryBucket models.NamesEntry
	err = helpers.MdbOne(
		helpers.MdbCollection(models.NamesTable).Find(bson.M{"userid": userID, "guildid": "global"}).Sort("-changedat"),
		&entryBucket,
	)

	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return "", errors.New("no username entry")
		}
		return "", err
	}

	return entryBucket.Username, nil
}

func (n *Names) SaveUsername(userID string, username string) (err error) {
	// don't store duplicates
	lastUsername, err := n.GetLastUsername(userID)
	if err == nil {
		if username == lastUsername {
			return nil
		}
	}
	// insert username
	_, err = helpers.MDbInsert(
		models.NamesTable,
		models.NamesEntry{
			ChangedAt: time.Now(),
			GuildID:   "global",
			UserID:    userID,
			Nickname:  "",
			Username:  username,
		},
	)
	return err
}

func (n *Names) GetNicknames(guildID string, userID string) (nicknames []string, err error) {
	var entryBucket []models.NamesEntry
	err = helpers.MDbIter(helpers.MdbCollection(models.NamesTable).Find(bson.M{"userid": userID, "guildid": guildID}).Sort("changedat")).All(&entryBucket)

	if err != nil {
		return nicknames, err
	}

	if entryBucket == nil || len(entryBucket) <= 0 {
		return nicknames, errors.New("no nickname entries")
	}

	for _, entry := range entryBucket {
		if len(nicknames) <= 0 || nicknames[len(nicknames)-1] != entry.Nickname {
			nicknames = append(nicknames, entry.Nickname)
		}
	}
	return nicknames, nil
}

func (n *Names) GetUsernames(userID string) (usernames []string, err error) {
	var entryBucket []models.NamesEntry
	err = helpers.MDbIter(helpers.MdbCollection(models.NamesTable).Find(bson.M{"userid": userID, "guildid": "global"}).Sort("changedat")).All(&entryBucket)

	if err != nil {
		return usernames, err
	}

	if entryBucket == nil || len(entryBucket) <= 0 {
		return usernames, errors.New("no username entries")
	}

	for _, entry := range entryBucket {
		if len(usernames) <= 0 || usernames[len(usernames)-1] != entry.Username {
			usernames = append(usernames, entry.Username)
		}
	}
	return usernames, nil
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
