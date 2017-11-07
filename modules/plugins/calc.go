package plugins

import (
	"strconv"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	"github.com/marcmak/calc/calc"
)

type Calc struct{}

func (c *Calc) Commands() []string {
	return []string{
		"calc",
		"math",
	}
}

func (c *Calc) Init(session *discordgo.Session) {

}

func (c *Calc) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	defer func() {
		err := recover()
		if err != nil {
			helpers.SendMessage(msg.ChannelID, "I couldn't solve it :sob:")
		}
	}()

	helpers.SendMessage(msg.ChannelID, "<:googlenerd:317030369205682186> "+strconv.FormatFloat(calc.Solve(content), 'E', 4, 64))
}
