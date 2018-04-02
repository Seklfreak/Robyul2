package plugins

import (
	"encoding/xml"

	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

type RandomCat struct{}

var RandomCatEndpoint string

func (rc RandomCat) Commands() []string {
	return []string{
		"cat",
	}
}

func (rc RandomCat) Init(session *discordgo.Session) {
	RandomCatEndpoint = "https://thecatapi.com/api/images/get?api_key=" +
		helpers.GetConfig().Path("thecatapi-api-key").Data().(string) +
		"&format=xml&results_per_page=1&type=jpg,gif,png"
}

func (rc RandomCat) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermAnimals) {
		return
	}

	session.ChannelTyping(msg.ChannelID)

	content = helpers.GetText("plugins.randomcat.error")
	link := rc.getRandomCatLink()
	if link != "" {
		content = helpers.GetTextF("plugins.randomcat.success", link)
	} else {
		cache.GetLogger().Error("received a empty randon cat link")
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
						link = rc.getRandomCatLink()
						if link != "" {
							helpers.EditMessage(messages[0].ChannelID, messages[0].ID,
								helpers.GetTextF("plugins.randomcat.success", link))
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

func (rc RandomCat) getRandomCatLink() (link string) {
	data, err := helpers.NetGetUAWithError(RandomCatEndpoint, helpers.DEFAULT_UA)
	helpers.Relax(err)

	var response RandomCatApiResponse
	xml.Unmarshal(data, &response)
	return response.Url
}

type RandomCatApiResponse struct {
	Source_url string `xml:"data>images>image>source_url"`
	Url        string `xml:"data>images>image>url"`
	Id         string `xml:"data>images>image>id"`
}
