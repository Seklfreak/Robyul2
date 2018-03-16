package plugins

import (
	"math/rand"
	"strings"
	"time"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
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

				_, err := helpers.MDbInsert(
					models.DonatorsTable,
					models.DonatorEntry{
						Name:          name,
						AddedAt:       time.Now(),
						HeartOverride: "",
					},
				)
				helpers.Relax(err)

				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.donators.add-success", name))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			})
			return
		}
	}

	var donators []models.DonatorEntry
	err := helpers.MDbIter(helpers.MdbCollection(models.DonatorsTable).Find(nil).Sort("addedat")).All(&donators)
	helpers.Relax(err)

	if donators == nil || len(donators) <= 0 {
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
