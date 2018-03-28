package plugins

import (
	"strings"

	"time"

	"fmt"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/globalsign/mgo/bson"
	"github.com/sirupsen/logrus"
)

type botStatusAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next botStatusAction)

type BotStatus struct{}

func (bs *BotStatus) Commands() []string {
	return []string{
		"bot-status",
	}
}

func (bs *BotStatus) Init(session *discordgo.Session) {
	go func() {
		defer helpers.Recover()

		time.Sleep(time.Second * 60)
		go bs.gameStatusRotationLoop()
	}()
}

func (bs *BotStatus) gameStatusRotationLoop() {
	defer helpers.Recover()
	defer func() {
		go func() {
			bs.logger().Error("The gameStatusRotationLoop died. Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			bs.gameStatusRotationLoop()
		}()
	}()

	var err error
	var newStatus string
	for {
		var entryBucket models.BotStatusEntry
		err = helpers.MdbPipeOneWithoutLogging(models.BotStatusTable, []bson.M{{"$sample": bson.M{"size": 1}}}, &entryBucket)
		if err != nil {
			if !helpers.IsMdbNotFound(err) {
				helpers.RelaxLog(err)
			}
			time.Sleep(60 * time.Second)
			continue
		}

		newStatus = bs.replaceText(entryBucket.Text)

		err = cache.GetSession().UpdateStatusComplex(discordgo.UpdateStatusData{
			Game: &discordgo.Game{
				Name: newStatus,
				Type: entryBucket.Type,
			},
			Status: "online",
		})
		helpers.RelaxLog(err)

		bs.logger().Infof("set the Bot Status to: \"%s\" using the rotation loop", newStatus)

		time.Sleep(45 * time.Minute)
	}
}

func (bs *BotStatus) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	session.ChannelTyping(msg.ChannelID)

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := bs.actionStart
	for action != nil {
		action = action(args, msg, &result)
	}
}

func (bs *BotStatus) actionStart(args []string, in *discordgo.Message, out **discordgo.MessageSend) botStatusAction {
	cache.GetSession().ChannelTyping(in.ChannelID)

	if len(args) < 1 {
		*out = bs.newMsg("bot.arguments.too-few")
		return bs.actionFinish
	}

	switch args[0] {
	case "add":
		return bs.actionAdd
	case "remove", "rem", "delete", "del":
		return bs.actionRemove
	case "set":
		return bs.actionSet
	case "list":
		return bs.actionList
	}

	*out = bs.newMsg("bot.arguments.invalid")
	return bs.actionFinish
}

func (bs *BotStatus) replaceText(text string) (result string) {
	users := make(map[string]string)
	channels := make(map[string]string)
	for _, guild := range cache.GetSession().State.Guilds {
		for _, u := range guild.Members {
			users[u.User.ID] = u.User.Username
		}
		for _, c := range guild.Channels {
			channels[c.ID] = c.Name
		}
	}

	text = strings.Replace(text, "{GUILD_COUNT}", humanize.Comma(int64(len(cache.GetSession().State.Guilds))), -1)
	text = strings.Replace(text, "{MEMBER_COUNT}", humanize.Comma(int64(len(users))), -1)
	text = strings.Replace(text, "{CHANNEL_COUNT}", humanize.Comma(int64(len(channels))), -1)

	return text
}

// [p]bot-status set <status text>
func (bs *BotStatus) actionSet(args []string, in *discordgo.Message, out **discordgo.MessageSend) botStatusAction {
	if !helpers.IsRobyulMod(in.Author.ID) {
		*out = bs.newMsg("robyulmod.no_permission")
		return bs.actionFinish
	}

	if len(args) < 3 {
		*out = bs.newMsg("bot.arguments.too-few")
		return bs.actionFinish
	}

	statusMessage := strings.TrimSpace(strings.Replace(strings.Join(args, " "), strings.Join(args[:2], " "), "", 1))
	statusType := bs.textToGameType(args[1])

	newStatus := bs.replaceText(statusMessage)

	err := cache.GetSession().UpdateStatusComplex(discordgo.UpdateStatusData{
		Game: &discordgo.Game{
			Name: newStatus,
			Type: statusType,
		},
		Status: "online",
	})
	helpers.Relax(err)

	bs.logger().WithField("UserID", in.Author.ID).Infof("Set the Bot Status to: \"%s\" using the set command", newStatus)

	*out = bs.newMsg(helpers.GetTextF("plugins.botstatus.set-success", newStatus))
	return bs.actionFinish
}

// [p]bot-status add <status text>
func (bs *BotStatus) actionAdd(args []string, in *discordgo.Message, out **discordgo.MessageSend) botStatusAction {
	if !helpers.IsRobyulMod(in.Author.ID) {
		*out = bs.newMsg("robyulmod.no_permission")
		return bs.actionFinish
	}

	if len(args) < 3 {
		*out = bs.newMsg("bot.arguments.too-few")
		return bs.actionFinish
	}

	statusMessage := strings.TrimSpace(strings.Replace(strings.Join(args, " "), strings.Join(args[:2], " "), "", 1))
	statusType := bs.textToGameType(args[1])

	_, err := helpers.MDbInsert(
		models.BotStatusTable,
		models.BotStatusEntry{
			AddedByUserID: in.Author.ID,
			AddedAt:       time.Now(),
			Text:          statusMessage,
			Type:          statusType,
		},
	)
	helpers.Relax(err)

	*out = bs.newMsg(helpers.GetTextF("plugins.botstatus.add-success", statusMessage))
	return bs.actionFinish
}

// [p]bot-status remove <status id>
func (bs *BotStatus) actionRemove(args []string, in *discordgo.Message, out **discordgo.MessageSend) botStatusAction {
	if !helpers.IsRobyulMod(in.Author.ID) {
		*out = bs.newMsg("robyulmod.no_permission")
		return bs.actionFinish
	}

	if len(args) < 2 {
		*out = bs.newMsg("bot.arguments.too-few")
		return bs.actionFinish
	}

	var entryBucket models.BotStatusEntry
	err := helpers.MdbOne(
		helpers.MdbCollection(models.BotStatusTable).Find(bson.M{"_id": helpers.HumanToMdbId(args[1])}),
		&entryBucket,
	)
	if err != nil {
		if !helpers.IsMdbNotFound(err) {
			helpers.RelaxLog(err)
		}
		*out = bs.newMsg("bot.arguments.invalid")
		return bs.actionFinish
	}

	err = helpers.MDbDelete(models.BotStatusTable, entryBucket.ID)
	helpers.Relax(err)

	*out = bs.newMsg(helpers.GetTextF("plugins.botstatus.remove-success", entryBucket.Text))
	return bs.actionFinish
}

// [p]bot-status list
func (bs *BotStatus) actionList(args []string, in *discordgo.Message, out **discordgo.MessageSend) botStatusAction {
	if !helpers.IsRobyulMod(in.Author.ID) {
		*out = bs.newMsg("robyulmod.no_permission")
		return bs.actionFinish
	}

	var entryBucket []models.BotStatusEntry
	err := helpers.MDbIter(helpers.MdbCollection(models.BotStatusTable).Find(nil)).All(&entryBucket)
	helpers.Relax(err)

	if entryBucket == nil || len(entryBucket) <= 0 {
		*out = bs.newMsg("plugins.botstatus.list-empty")
		return bs.actionFinish
	}

	var message string
	for _, botStatus := range entryBucket {
		message += fmt.Sprintf("`%s`: %s `%s`\n",
			helpers.MdbIdToHuman(botStatus.ID), bs.gameTypeToText(botStatus.Type), botStatus.Text)
	}
	message += fmt.Sprintf("_found %d statuses in total_\n", len(entryBucket))

	*out = bs.newMsg(message)
	return bs.actionFinish
}

func (bs *BotStatus) textToGameType(text string) (gameType discordgo.GameType) {
	switch strings.ToLower(text) {
	case "playing", "game":
		return discordgo.GameTypeGame
	case "listening":
		return discordgo.GameTypeListening
	case "watching":
		return discordgo.GameTypeWatching
	}
	return discordgo.GameTypeGame
}

func (bs *BotStatus) gameTypeToText(gameType discordgo.GameType) (text string) {
	switch gameType {
	case discordgo.GameTypeGame:
		return "playing"
	case discordgo.GameTypeListening:
		return "listening to"
	case discordgo.GameTypeWatching:
		return "watching"
	}
	return ""
}

func (bs *BotStatus) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) botStatusAction {
	_, err := helpers.SendComplex(in.ChannelID, *out)
	helpers.Relax(err)

	return nil
}

func (bs *BotStatus) newMsg(content string) *discordgo.MessageSend {
	return &discordgo.MessageSend{Content: helpers.GetText(content)}
}

func (bs *BotStatus) logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "botstatus")
}
