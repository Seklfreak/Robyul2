package plugins

import "github.com/bwmarrin/discordgo"

type Tableflip struct{}

func (tFlip Tableflip) Commands() []string {
    return []string{
        "tableflip",
		"tbflip",
    }
}

func (tFlip Tableflip) Init(session *discordgo.Session) {

}

func (tFlip Tableflip) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    session.ChannelMessageSend(msg.ChannelID, "(╯°□°）╯︵ ┻━┻")
}
