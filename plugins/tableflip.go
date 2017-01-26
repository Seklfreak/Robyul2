package plugins

import "github.com/bwmarrin/discordgo"

type TableFlip struct{}

func (t TableFlip) Commands() []string {
    return []string{
        "tableflip",
        "tbflip",
        "tflip",
    }
}

func (t TableFlip) Init(session *discordgo.Session) {

}

func (t TableFlip) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    session.ChannelMessageSend(msg.ChannelID, "(╯°□°）╯︵ ┻━┻")
}
