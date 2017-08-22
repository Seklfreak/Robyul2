package plugins

import (
	"net/url"
	"strconv"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

type UrbanDict struct{}

func (u *UrbanDict) Commands() []string {
	return []string{
		"urban",
		"ub",
	}
}

func (u *UrbanDict) Init(session *discordgo.Session) {

}

func (u *UrbanDict) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	session.ChannelTyping(msg.ChannelID)

	if content == "" {
		session.ChannelMessageSend(msg.ChannelID, "You should pass a word to define <:blobthinking:317028940885524490>")
		return
	}

	endpoint := "http://api.urbandictionary.com/v0/define?term=" + url.QueryEscape(content)

	json := helpers.GetJSON(endpoint)

	res, e := json.Path("list").Children()
	helpers.Relax(e)

	if len(res) == 0 {
		session.ChannelMessageSend(msg.ChannelID, "No results <:blobneutral:317029459720929281>")
		return
	}

	object, e := res[0].ChildrenMap()
	helpers.Relax(e)

	children, e := json.Path("tags").Children()
	helpers.Relax(e)

	tags := ""
	for _, child := range children {
		tags += child.Data().(string) + ", "
	}

	description := object["definition"].Data().(string)
	if len(description) > 2000 {
		description = description[:1996] + " ..."
	}

	definitionEmbed := &discordgo.MessageEmbed{
		Color:       0x134FE6,
		Title:       object["word"].Data().(string),
		Description: description,
		URL:         object["permalink"].Data().(string),
		Fields:      []*discordgo.MessageEmbedField{},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "powered by urbandictionary.com",
		},
	}

	if object["example"].Data().(string) != "" {
		definitionEmbed.Fields = append(definitionEmbed.Fields, &discordgo.MessageEmbedField{Name: "Example(s)", Value: object["example"].Data().(string), Inline: false})
	}
	if tags != "" {
		definitionEmbed.Fields = append(definitionEmbed.Fields, &discordgo.MessageEmbedField{Name: "Tags", Value: tags, Inline: false})
	}
	if strconv.FormatFloat(object["thumbs_up"].Data().(float64), 'f', 0, 64) != "0" || strconv.FormatFloat(object["thumbs_down"].Data().(float64), 'f', 0, 64) != "0" {
		definitionEmbed.Fields = append(definitionEmbed.Fields, &discordgo.MessageEmbedField{
			Name: "Votes",
			Value: ":+1: " + strconv.FormatFloat(object["thumbs_up"].Data().(float64), 'f', 0, 64) +
				" | :-1: " + strconv.FormatFloat(object["thumbs_down"].Data().(float64), 'f', 0, 64),
			Inline: true,
		})
	}

	_, err := session.ChannelMessageSendEmbed(msg.ChannelID, definitionEmbed)
	helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
}
