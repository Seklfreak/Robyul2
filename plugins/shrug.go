package plugins

import "github.com/bwmarrin/discordgo"

type Shrug struct{}

func (s Shrug) Commands() []string {
    return []string{
        "shrug",
    }
}

func (s Shrug) Init(session *discordgo.Session) {

}

func (s Shrug) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    session.ChannelMessageSend(msg.ChannelID, "¯\\_(ツ)_/¯")
}
