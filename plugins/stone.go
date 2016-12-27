package plugins

import (
    "fmt"
    "github.com/bwmarrin/discordgo"
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
    session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf(
        "<@%s> IS GOING TO DIE!!!\n" +
            "COME ON GUYS! THROW SOME STONES WITH MEE!!!\n" +
            ":grimacing: :wavy_dash::anger::astonished:",
        msg.Mentions[0].ID,
    ))
}
