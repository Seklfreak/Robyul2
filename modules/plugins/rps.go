package plugins

import (
    "github.com/bwmarrin/discordgo"
    "regexp"
)

type RPS struct{}

func (r *RPS) Commands() []string {
    return []string{
        "rps",
    }
}

func (r *RPS) Init(session *discordgo.Session) {

}

func (r *RPS) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    switch {
    case regexp.MustCompile("(?i)rock").MatchString(content):
        session.ChannelMessageSend(msg.ChannelID, "I've chosen :newspaper:\nMy paper wraps your stone.\nI win :smiley:")
        return

    case regexp.MustCompile("(?i)paper").MatchString(content):
        session.ChannelMessageSend(msg.ChannelID, "I've chosen :scissors:\nMy scissors cuts your paper!\nI win :smiley:")
        return

    case regexp.MustCompile("(?i)scissors").MatchString(content):
        session.ChannelMessageSend(msg.ChannelID, "I've chosen :white_large_square:\nMy stone breaks your scissors.\nI win :smiley:")
        return
    }

    session.ChannelMessageSend(msg.ChannelID, "That's an odd or invalid choice for RPS :neutral_face:")
}
