package plugins

import "github.com/bwmarrin/discordgo"

type Support struct{}

func (s *Support) Commands() []string {
    return []string{
        "support",
        "discord",
    }
}

func (s *Support) Init(session *discordgo.Session) {

}

func (s *Support) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    session.ChannelMessageSend(msg.ChannelID, "Here you go :smiley: \n https://discord.gg/wNPejct")
}
