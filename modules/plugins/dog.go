package plugins

import (
	"strings"

	"time"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Seklfreak/Robyul2/shardmanager"
	"github.com/bwmarrin/discordgo"
	"github.com/globalsign/mgo/bson"
)

type Dog struct{}

func (m *Dog) Commands() []string {
	return []string{
		"dog",
	}
}

func (m *Dog) Init(session *shardmanager.Manager) {

}

func (m *Dog) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermAnimals) {
		return
	}

	session.ChannelTyping(msg.ChannelID)

	args := strings.Fields(content)
	if len(args) > 0 {
		switch args[0] {
		case "add":
			helpers.RequireRobyulMod(msg, func() {
				if len(args) < 2 && (len(args) < 1 && len(msg.Attachments) <= 0) {
					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
					return
				}

				var url string
				if len(msg.Attachments) > 0 {
					url = msg.Attachments[0].URL
				}
				if len(args) >= 2 {
					url = strings.TrimSpace(args[1])
				}

				if url == "" {
					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
					return
				}

				image, err := helpers.NetGetUAWithError(url, helpers.DEFAULT_UA)
				helpers.Relax(err)

				objectName, err := helpers.AddFile("", image, helpers.AddFileMetadata{
					ChannelID:          msg.ChannelID,
					UserID:             msg.Author.ID,
					AdditionalMetadata: nil,
				}, "dog", true)

				_, err = helpers.MDbInsert(
					models.DogLinksTable,
					models.DogLinkEntry{
						ObjectName:    objectName,
						AddedByUserID: msg.Author.ID,
						AddedAt:       time.Now(),
					},
				)
				helpers.Relax(err)

				url, err = helpers.GetFileLink(objectName)
				helpers.Relax(err)

				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.dog.add-success", url))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			})
			return
		}
	}

	content = helpers.GetText("plugins.dog.none")
	link := m.getRandomDogLink()
	if link != "" {
		content = helpers.GetTextF("plugins.dog.result", link)
	}

	messages, err := helpers.SendMessage(
		msg.ChannelID,
		content,
	)
	helpers.RelaxMessage(err, msg.ChannelID, msg.ID)

	if len(messages) <= 0 {
		return
	}

	err = session.MessageReactionAdd(msg.ChannelID, messages[0].ID, "🎲")
	if err == nil {
		if err == nil {
			rerollHandler := session.AddHandler(func(session *discordgo.Session, reaction *discordgo.MessageReactionAdd) {
				defer helpers.Recover()

				if reaction.MessageID == messages[0].ID {
					if reaction.UserID == session.State.User.ID {
						return
					}

					if reaction.UserID == msg.Author.ID && reaction.Emoji.Name == "🎲" {
						link = m.getRandomDogLink()
						if link != "" {
							helpers.EditMessage(messages[0].ChannelID, messages[0].ID,
								helpers.GetTextF("plugins.dog.result", link))
						}
						session.MessageReactionRemove(reaction.ChannelID, reaction.MessageID, reaction.Emoji.Name, reaction.UserID)
					}
				}
			})
			time.Sleep(5 * time.Minute)
			rerollHandler()
			session.MessageReactionRemove(msg.ChannelID, messages[0].ID, "🎲", session.State.User.ID)
		}
	}
}

func (m *Dog) getRandomDogLink() (link string) {
	var entryBucket models.DogLinkEntry
	// TODO: pipe aggregation
	err := helpers.MdbPipeOne(models.DogLinksTable,
		[]bson.M{{"$sample": bson.M{"size": 1}}},
		&entryBucket)
	helpers.RelaxLog(err)
	if err != nil && helpers.IsMdbNotFound(err) {
		return ""
	}

	if entryBucket.URL != "" {
		return entryBucket.URL
	}

	link, err = helpers.GetFileLink(entryBucket.ObjectName)
	helpers.RelaxLog(err)
	return link
}
