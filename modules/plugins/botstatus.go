package plugins

import (
	"errors"
	"math/rand"
	"strings"

	"time"

	"fmt"

	"strconv"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	rethink "github.com/gorethink/gorethink"
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

	randGen := rand.New(rand.NewSource(time.Now().UnixNano()))

	var newStatus string
	for {
		statuses, err := bs.getAllBotStatuses()
		if err != nil {
			time.Sleep(60 * time.Second)
			continue
		}

		randInt := randGen.Intn(len(statuses))

		newStatus = bs.replaceText(statuses[randInt].Text)

		err = cache.GetSession().UpdateStatus(0, newStatus)
		helpers.RelaxLog(err)

		bs.logger().Infof("Set the Bot Status to: \"%s\" using the rotation loop", newStatus)

		time.Sleep(45 * time.Minute)
	}
}

func (bs *BotStatus) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	defer helpers.Recover()

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

	text = strings.Replace(text, "{GUILD_COUNT}", strconv.Itoa(len(cache.GetSession().State.Guilds)), -1)
	text = strings.Replace(text, "{MEMBER_COUNT}", strconv.Itoa(len(users)), -1)
	text = strings.Replace(text, "{CHANNEL_COUNT}", strconv.Itoa(len(channels)), -1)

	return text
}

// [p]bot-status set <status text>
func (bs *BotStatus) actionSet(args []string, in *discordgo.Message, out **discordgo.MessageSend) botStatusAction {
	if !helpers.IsRobyulMod(in.Author.ID) {
		*out = bs.newMsg("robyulmod.no_permission")
		return bs.actionFinish
	}

	if len(args) <= 0 {
		*out = bs.newMsg("bot.arguments.too-few")
		return bs.actionFinish
	}

	parts := strings.Split(in.Content, args[0])
	if len(parts) < 2 {
		*out = bs.newMsg("bot.arguments.too-few")
		return bs.actionFinish
	}
	statusMessage := strings.TrimSpace(strings.Join(parts[1:], args[0]))

	newStatus := bs.replaceText(statusMessage)
	err := cache.GetSession().UpdateStatus(0, newStatus)
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

	if len(args) <= 0 {
		*out = bs.newMsg("bot.arguments.too-few")
		return bs.actionFinish
	}

	parts := strings.Split(in.Content, args[0])
	if len(parts) < 2 {
		*out = bs.newMsg("bot.arguments.too-few")
		return bs.actionFinish
	}
	statusMessage := strings.TrimSpace(strings.Join(parts[1:], args[0]))

	err := bs.insertBotStatus(in.Author.ID, statusMessage)
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

	botStatus, err := bs.getBotStatusByID(args[1])
	if err != nil {
		*out = bs.newMsg("bot.arguments.invalid")
		return bs.actionFinish
	}

	err = bs.removeFromBotStatuses(botStatus)
	helpers.Relax(err)

	*out = bs.newMsg(helpers.GetTextF("plugins.botstatus.remove-success", botStatus.Text))
	return bs.actionFinish
}

// [p]bot-status list
func (bs *BotStatus) actionList(args []string, in *discordgo.Message, out **discordgo.MessageSend) botStatusAction {
	if !helpers.IsRobyulMod(in.Author.ID) {
		*out = bs.newMsg("robyulmod.no_permission")
		return bs.actionFinish
	}

	botStatuses, err := bs.getAllBotStatuses()
	helpers.Relax(err)

	if len(botStatuses) <= 0 {
		*out = bs.newMsg("plugins.botstatus.list-empty")
		return bs.actionFinish
	}

	var message string
	for _, botStatus := range botStatuses {
		message += fmt.Sprintf("`%s`: `%s`\n", botStatus.ID, botStatus.Text)
	}
	message += fmt.Sprintf("_found %d statuses in total_\n", len(botStatuses))

	*out = bs.newMsg(message)
	return bs.actionFinish
}

func (bs *BotStatus) insertBotStatus(authorID string, text string) (err error) {
	insert := rethink.Table(models.BotStatusTable).Insert(models.BotStatus{
		AddedByUserID: authorID,
		AddedAt:       time.Now(),
		Text:          text,
	})
	_, err = insert.RunWrite(helpers.GetDB())
	return err
}

func (bs *BotStatus) getBotStatusByID(id string) (entryBucket models.BotStatus, err error) {
	listCursor, err := rethink.Table(models.BotStatusTable).Get(id).Run(helpers.GetDB())
	if err != nil {
		return entryBucket, err
	}
	defer listCursor.Close()
	err = listCursor.One(&entryBucket)
	return entryBucket, err
}
func (bs *BotStatus) removeFromBotStatuses(botStatusEntry models.BotStatus) error {
	if botStatusEntry.ID != "" {
		_, err := rethink.Table(models.BotStatusTable).Get(botStatusEntry.ID).Delete().RunWrite(helpers.GetDB())
		return err
	}
	return errors.New("empty botStatusEntry submitted")
}

func (bs *BotStatus) getAllBotStatuses() (result []models.BotStatus, err error) {
	listCursor, err := rethink.Table(models.BotStatusTable).Run(helpers.GetDB())
	if err != nil {
		return nil, err
	}
	defer listCursor.Close()
	err = listCursor.All(&result)
	return result, err
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
