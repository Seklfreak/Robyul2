package plugins

import (
    "fmt"
    "github.com/bwmarrin/discordgo"
)

type Avatar struct{}

func (a Avatar) Commands() []string {
    return []string{
        "avatar",
    }
}

func (a Avatar) Init(session *discordgo.Session) {

}

func (a Avatar) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    if len(msg.Mentions) > 0 {
        session.ChannelMessageSend(msg.ChannelID, "Here you go :smiley: \n " + fmt.Sprintf(
            "https://cdn.discordapp.com/avatars/%s/%s.jpg",
            msg.Mentions[0].ID,
            msg.Mentions[0].Avatar,
        ))
    } else {
        session.ChannelMessageSend(msg.ChannelID, "You should mention a user :thinking:")
    }
}
