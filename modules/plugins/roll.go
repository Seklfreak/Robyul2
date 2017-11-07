package plugins

import (
	"math/rand"
	"regexp"
	"strconv"
	"strings"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

type Roll struct{}

func (r *Roll) Commands() []string {
	return []string{
		"roll",
	}
}

func (r *Roll) Init(session *discordgo.Session) {

}

func (r *Roll) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	regex := regexp.MustCompile(`[^\d-]`)
	args := strings.Fields(content)

	if len(args) == 2 {
		min, e := strconv.ParseInt(regex.ReplaceAllString(args[0], ""), 10, 64)
		if e != nil {
			helpers.SendMessage(msg.ChannelID, ":x: Please check your format")
			return
		}

		max, e := strconv.ParseInt(regex.ReplaceAllString(args[1], ""), 10, 64)
		if e != nil {
			helpers.SendMessage(msg.ChannelID, ":x: Please check your format")
			return
		}

		if min <= 0 || max <= 0 {
			helpers.SendMessage(msg.ChannelID, ":x: Only positive numbers are allowed")
			return
		}

		if min >= max {
			helpers.SendMessage(msg.ChannelID, ":x: Number ranges don't work like that. (`min >= max`)")
			return
		}

		helpers.SendMessage(
			msg.ChannelID,
			":crystal_ball: "+strconv.Itoa(rand.Intn(int(max-min))+int(min)),
		)
	} else {
		helpers.SendMessage(msg.ChannelID, ":x: You need to pass two numbers")
	}
}
