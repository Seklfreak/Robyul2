package plugins

import (
    "github.com/bwmarrin/discordgo"
    "math/rand"
    "strings"
)

type Choice struct{}

func (c *Choice) Commands() []string {
    return []string{
        "choose",
        "choice",
    }
}

func (c *Choice) Init(session *discordgo.Session) {

}

func (c *Choice) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    if !strings.Contains(content, "|") {
        session.ChannelMessageSend(msg.ChannelID, ":x: You need to pass multiple options separeated by `|`")
        return
    }

    choices := strings.Split(content, "|")

    session.ChannelMessageSend(msg.ChannelID, "I've chosen `"+choices[rand.Intn(len(choices))]+"` :smiley:")
}
