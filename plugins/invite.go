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
        fmt.Sprintf(
            "Woah thanks :heart_eyes: \n To add me to your discord server visit https://discordapp.com/oauth2/authorize?client_id=%s&scope=bot&permissions=%s :smiley:",
            utils.GetConfig().Path("discord.id").Data().(string),
            utils.GetConfig().Path("discord.perms").Data().(string),
        ),
    )
}