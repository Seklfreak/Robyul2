package plugins

import (
    "fmt"
    "github.com/sn0w/discordgo"
    "git.lukas.moe/sn0w/Karen/helpers"
)

type Stone struct{}

func (s Stone) Commands() []string {
    return []string{
        "stone",
    }
}

func (s Stone) Init(session *discordgo.Session) {

}

func (s Stone) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    mentionCount := len(msg.Mentions)

    if mentionCount == 0 {
        session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.mentions.too-few"))
        return
    }

    if mentionCount > 1 {
        session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.mentions.too-many"))
        return
    }

    session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf(
        "<@%s> IS GOING TO DIE!!!\n" +
            "COME ON GUYS! THROW SOME STONES WITH MEE!!!\n" +
            ":grimacing: :wavy_dash::anger::astonished:",
        msg.Mentions[0].ID,
    ))
}
