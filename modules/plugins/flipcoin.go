package plugins

import (
    "github.com/bwmarrin/discordgo"
    "math/rand"
)

type FlipCoin struct {
    choices [2]string
}

func (f *FlipCoin) Commands() []string {
    return []string{
        "flip",
    }
}

func (f *FlipCoin) Init(session *discordgo.Session) {
    f.choices = [2]string{
        "Heads! :black_circle:",
        "Tails! :red_circle:",
    }
}

func (f *FlipCoin) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    session.ChannelMessageSend(
        msg.ChannelID,
        f.choices[rand.Intn(len(f.choices))],
    )
}
