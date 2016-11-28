package plugins

import (
    "github.com/bwmarrin/discordgo"
    "fmt"
    "net/url"
)

type Google struct{}

func (g Google) Name() string {
    return "Google"
}

func (g Google) Description() string {
    return "If someone is too dumb/lazy to use google, use this."
}

func (g Google) Commands() map[string]string {
    return map[string]string{
        "google" : "<any search query>",
    }
}

func (g Google) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf(
        "<https://lmgtfy.com/?q=%s>",
        url.QueryEscape(content),
    ))
}