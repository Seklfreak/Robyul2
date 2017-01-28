package plugins

import "github.com/sn0w/discordgo"

type Donate struct{}

func (d Donate) Commands() []string {
    return []string{
        "donate",
    }
}

func (d Donate) Init(session *discordgo.Session) {

}

func (d Donate) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    session.ChannelMessageSend(msg.ChannelID, "Thank you so much :3 \n https://www.patreon.com/sn0w")
}
