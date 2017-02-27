package plugins

import (
    "git.lukas.moe/sn0w/Karen/helpers"
    "github.com/bwmarrin/discordgo"
    "net/url"
    "strconv"
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
        session.ChannelMessageSend(msg.ChannelID, "You should pass a word to define :thinking:")
        return
    }

    endpoint := "http://api.urbandictionary.com/v0/define?term=" + url.QueryEscape(content)

    json := helpers.GetJSON(endpoint)

    res, e := json.Path("list").Children()
    helpers.Relax(e)

    if len(res) == 0 {
        session.ChannelMessageSend(msg.ChannelID, "No results :neutral_face:")
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

    session.ChannelMessageSendEmbed(
        msg.ChannelID,
        &discordgo.MessageEmbed{
            Color: 0x134FE6,
            Title: object["word"].Data().(string),
            Description: object["definition"].Data().(string),
            URL: object["permalink"].Data().(string),
            Fields: []*discordgo.MessageEmbedField{
                {Name: "Example(s)", Value: object["example"].Data().(string), Inline: false},
                {Name: "Tags", Value: tags, Inline: false},
                {Name: "Author", Value: object["author"].Data().(string), Inline: true},
                {
                    Name: "Votes",
                    Value: ":+1: " + strconv.FormatFloat(object["thumbs_up"].Data().(float64), 'f', 0, 64) +
                        " | :-1: " + strconv.FormatFloat(object["thumbs_down"].Data().(float64), 'f', 0, 64),
                    Inline: true,
                },
            },
            Footer: &discordgo.MessageEmbedFooter{
                Text: "powered by urbandictionary.com",
            },
        },
    )
}
