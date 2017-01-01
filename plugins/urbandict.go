package plugins

import (
    "github.com/bwmarrin/discordgo"
    "github.com/sn0w/Karen/helpers"
    "net/url"
)

type UrbanDict struct{}

func (u UrbanDict) Commands() []string {
    return []string{
        "urban",
        "ub",
    }
}

func (u UrbanDict) Init(session *discordgo.Session) {

}

func (u UrbanDict) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
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

    session.ChannelMessageSend(
        msg.ChannelID,
        "The definition of `" + object["word"].Data().(string) + "` is:\n" +
            "```\n" +
            object["definition"].Data().(string) + "\n" +
            "```\n" +
            "Example: `" + object["example"].Data().(string) + "`\n" +
            "Tags: `" + tags + "`",
    )
}
