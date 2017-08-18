package plugins

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

type Choice struct{}

func (c *Choice) Commands() []string {
	return []string{
		"choose",
		"choice",
		"roll",
	}
}

var (
	splitChooseRegex *regexp.Regexp
)

func (c *Choice) Init(session *discordgo.Session) {
	splitChooseRegex = regexp.MustCompile(`'.*?'|".*?"|\S+`)
}

func (c *Choice) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	switch command {
	case "choose", "choice": // [p]choose <option a> <option b> [...]
		choices := splitChooseRegex.FindAllString(content, -1)

		if len(choices) <= 1 {
			_, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
			helpers.Relax(err)
			return
		}

		choice := choices[rand.Intn(len(choices))]
		choice = strings.Trim(choice, "\"")
		choice = strings.Trim(choice, "\"")

		_, err := session.ChannelMessageSend(msg.ChannelID, "I've chosen `"+choice+"` <:googlesmile:317031693951434752>")
		helpers.Relax(err)
		return
	case "roll": // [p]roll [<max numb, default: 100>]
		var err error
		maxN := 100
		if content != "" {
			maxN, err = strconv.Atoi(content)
			if err != nil || maxN < 1 {
				_, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				helpers.Relax(err)
				return
			}
		}
		rand.Seed(time.Now().Unix())
		_, err = session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("<@%s> :game_die: %d :game_die:", msg.Author.ID, rand.Intn(maxN)+1))
		helpers.Relax(err)
		return
	}
}
