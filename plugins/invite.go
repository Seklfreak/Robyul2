package plugins

import (
    "github.com/bwmarrin/discordgo"
    "fmt"
    "github.com/sn0w/Karen/utils"
)

type Invite struct{}

func (i Invite) Name() string {
    return "Invite"
}

func (i Invite) HelpHidden() bool {
    return false
}

func (i Invite) Description() string {
    return "Get an invite link for me"
}

func (i Invite) Commands() map[string]string {
    return map[string]string{
        "invite" : "",
        "inv" : "Alias for invite",
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
