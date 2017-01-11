package plugins

import "github.com/bwmarrin/discordgo"

type Lenny struct{}

func (l Lenny) Commands() []string {
    return []string{
        "lenny",
    }
}

func (l Lenny) Init(session *discordgo.Session) {

}

func (l Lenny) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    session.ChannelMessageSend(msg.ChannelID, "( ͡° ͜ʖ ͡°)")
}
