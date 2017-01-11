package plugins

import (
    "github.com/bwmarrin/discordgo"
    "github.com/marcmak/calc/calc"
    "strconv"
)

type Calc struct{}

func (c Calc) Commands() []string {
    return []string{
        "calc",
        "math",
    }
}

func (c Calc) Init(session *discordgo.Session) {

}

func (c Calc) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    session.ChannelMessageSend(msg.ChannelID, strconv.FormatFloat(calc.Solve(content), 'E', 4, 64))
}
