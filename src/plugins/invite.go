package plugins

import (
    "github.com/bwmarrin/discordgo"
    "fmt"
)

type Invite struct{}

func (i Invite) Name() string {
    return "Invite"
}

func (i Invite) Description() string {
    return "Get an invite link for me"
}

func (i Invite) Commands() map[string]string {
    return map[string]string{
        "invite" : "",
        "inv" : "",
    }
}

func (i Invite) Action(command string, msg *discordgo.Message, session *discordgo.Session) {
    session.ChannelMessageSend(
        msg.ChannelID,
        fmt.Sprintf(
            "\n To add me to your discord server visit https://discordapp.com/oauth2/authorize?client_id=%s&scope=bot&permissions=%d\n\n",
            "249908516880515072",
            104188928,
        ),
    )
}

func (i Invite) New() Plugin {
    return &Ping{}
}
