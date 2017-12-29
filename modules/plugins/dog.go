package plugins

import (
	"math/rand"
	"time"

	"strings"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	rethink "github.com/gorethink/gorethink"
)

type Dog struct{}

func (d *Dog) Commands() []string {
	return []string{
		"dog",
	}
}

func (d *Dog) Init(session *discordgo.Session) {

}

func (d *Dog) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermDog) {
		return
	}

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

				url := strings.TrimSpace(args[1])

				err := d.InsertLink(url, msg.Author.ID)
				helpers.Relax(err)

				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.dog.add-success", url))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			})
			return
		}
	}

	var entryBucket []models.DogLinkEntry
	listCursor, err := rethink.Table(models.DogLinksTable).Run(helpers.GetDB())
	helpers.Relax(err)
	defer listCursor.Close()
	err = listCursor.All(&entryBucket)
	helpers.Relax(err)

	if len(entryBucket) <= 0 {
		helpers.SendMessage(
			msg.ChannelID,
			helpers.GetText("plugins.dog.none"),
		)
		return
	}

	randGen := rand.New(rand.NewSource(time.Now().UnixNano()))

	_, err = helpers.SendMessage(
		msg.ChannelID,
		helpers.GetTextF("plugins.dog.result", entryBucket[randGen.Intn(len(entryBucket))].URL),
	)
	helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
}

func (d *Dog) InsertLink(URL string, UserID string) (err error) {
	insert := rethink.Table(models.DogLinksTable).Insert(models.DogLinkEntry{
		URL:           URL,
		AddedByUserID: UserID,
		AddedAt:       time.Now(),
	})
	_, err = insert.RunWrite(helpers.GetDB())
	return err
}
