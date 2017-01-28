package plugins

import (
    "github.com/sn0w/discordgo"
)

type Invite struct{}

func (i Invite) Commands() []string {
    return []string{
        "invite",
        "inv",
    }
}

func (i Invite) Init(session *discordgo.Session) {

}

func (i Invite) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    session.ChannelMessageSend(
        msg.ChannelID,
        "Woah thanks :heart_eyes: \n I'm still beta but you can apply for early-access here: <https://meetkaren.xyz/invite>",
    )
}
