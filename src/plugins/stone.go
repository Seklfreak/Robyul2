package plugins

import (
    "github.com/bwmarrin/discordgo"
    "fmt"
)

type Stone struct{}

func (s Stone) Name() string {
    return "Stone"
}

func (s Stone) HelpHidden() bool {
    return false
}

func (s Stone) Description() string {
    return "Stone someone to death!!!1!11!"
}

func (s Stone) Commands() map[string]string {
    return map[string]string{
        "stone" : "<@mention>",
    }
}

func (s Stone) Init(session *discordgo.Session) {

}

func (s Stone) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf(
        "<@%s> IS GOING TO DIE!!!\n" +
            "COME ON GUYS! THROW SOME STONES WITH MEE!!!\n" +
            ":grimacing: :wavy_dash::anger::astonished:",
        msg.Mentions[0].ID,
    ))
}