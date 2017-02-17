package plugins

import (
    "fmt"
    "github.com/sn0w/discordgo"
    "net/url"
)

type Google struct{}

func (g *Google) Commands() []string {
    return []string{
        "google",
        "goog",
    }
}

func (g *Google) Init(session *discordgo.Session) {

}

func (g *Google) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf(
        "<https://lmgtfy.com/?q=%s>",
        url.QueryEscape(content),
    ))
}
