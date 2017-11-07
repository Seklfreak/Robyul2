package plugins

import (
	"math/rand"
	"strings"
	"time"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	rethink "github.com/gorethink/gorethink"
)

type Donators struct{}

func (d *Donators) Commands() []string {
	return []string{
		"donators",
		"donations",
		"donate",
		"supporters",
		"support",
		"patreon",
		"patreons",
		"credits",
		"patrons",
	}
}

var (
	hearts = []string{"💕", "💞", "💗", "💝", "💘", "💖", "💓", "💕"}
)

func (d *Donators) Init(session *discordgo.Session) {
}

func (d *Donators) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	session.ChannelTyping(msg.ChannelID)

	args := strings.Fields(content)
	if len(args) > 0 {
		switch args[0] {
		case "add":
			helpers.RequireRobyulMod(msg, func() {
				if len(args) < 2 {
					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
					return
				}

				name := strings.TrimSpace(strings.Replace(content, args[0], "", 1))

				err := d.InsertDonator(name, "")
				helpers.Relax(err)

				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.donators.add-success", name))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			})
			return
		}
	}

	donators, err := d.GetDonators()
	helpers.Relax(err)

	if len(donators) <= 0 {
		helpers.SendMessage(
			msg.ChannelID,
			helpers.GetText("plugins.donators.none"),
		)
		return
	}

	randGen := rand.New(rand.NewSource(time.Now().UnixNano()))

	donatorsListText := ""
	for _, donator := range donators {
		donatorsListText += strings.TrimSpace(donator.Name) + " "
		if donator.HeartOverride != "" {
			donatorsListText += strings.TrimSpace(donator.HeartOverride)
		} else {
			donatorsListText += hearts[randGen.Intn(len(hearts))]
		}
		donatorsListText += "\n"
	}

	donatorsText := helpers.GetTextF("plugins.donators.list", donatorsListText)

	for _, page := range helpers.Pagify(donatorsText, "\n") {
		helpers.SendMessage(msg.ChannelID, page)
	}
}

func (d *Donators) InsertDonator(Name string, HeartOverride string) (err error) {
	insert := rethink.Table(models.DonatorsTable).Insert(models.DonatorEntry{
		Name:          Name,
		HeartOverride: HeartOverride,
		AddedAt:       time.Now(),
	})
	_, err = insert.RunWrite(helpers.GetDB())
	return err
}

func (d *Donators) GetDonators() (entries []models.DonatorEntry, err error) {
	listCursor, err := rethink.Table(models.DonatorsTable).OrderBy(rethink.Asc("added_at")).Run(helpers.GetDB())
	if err != nil {
		return []models.DonatorEntry{}, err
	}
	defer listCursor.Close()
	err = listCursor.All(&entries)
	return entries, err
}
