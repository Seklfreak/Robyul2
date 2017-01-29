package plugins

import (
    "fmt"
    "github.com/sn0w/discordgo"
    "git.lukas.moe/sn0w/Karen/helpers"
)

type Avatar struct{}

func (a Avatar) Commands() []string {
    return []string{
        "avatar",
    }
}

func (a Avatar) Init(session *discordgo.Session) {

}

func (a Avatar) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    mentionCount := len(msg.Mentions)

    if mentionCount == 0 {
        session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.mentions.too-few"))
        return
    }

    if mentionCount > 1 {
        session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.mentions.too-many"))
        return
    }

    session.ChannelMessageSend(msg.ChannelID, "Here you go :smiley: \n " + fmt.Sprintf(
        "https://cdn.discordapp.com/avatars/%s/%s.jpg",
        msg.Mentions[0].ID,
        msg.Mentions[0].Avatar,
    ))
}
