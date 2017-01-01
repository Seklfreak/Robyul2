package plugins

import (
    "fmt"
    "github.com/bwmarrin/discordgo"
    "github.com/sn0w/Karen/helpers"
)

type XKCD struct{}

func (x XKCD) Commands() []string {
    return []string{
        "xkcd",
    }
}

func (x XKCD) Init(session *discordgo.Session) {

}

func (x XKCD) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    session.ChannelTyping(msg.ChannelID)

    json := helpers.GetJSON("https://xkcd.com/info.0.json")
    session.ChannelMessageSend(
        msg.ChannelID,
        fmt.Sprintf(
            "#%d from %s/%s/%s\n%s\n%s",
            int(json.Path("num").Data().(float64)),
            json.Path("day").Data().(string),
            json.Path("month").Data().(string),
            json.Path("year").Data().(string),
            json.Path("title").Data().(string),
            json.Path("img").Data().(string),
        ),
    )
    session.ChannelMessageSend(msg.ChannelID, json.Path("alt").Data().(string))
}
