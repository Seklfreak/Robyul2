package plugins

import (
    "github.com/bwmarrin/discordgo"
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
        "Woah thanks :heart_eyes: \n I'm still beta but you can register for access here: <https://goo.gl/forms/9J9GYMg8c9IM6a5Z2>",
    )
}
