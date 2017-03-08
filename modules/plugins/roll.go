package plugins

import (
    "github.com/bwmarrin/discordgo"
    "math/rand"
    "regexp"
    "strconv"
    "strings"
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
    args := strings.Split(content, " ")

    if len(args) == 2 {
        min, e := strconv.ParseInt(regex.ReplaceAllString(args[0], ""), 10, 64)
        if e != nil {
            session.ChannelMessageSend(msg.ChannelID, ":x: Please check your format")
            return
        }

        max, e := strconv.ParseInt(regex.ReplaceAllString(args[1], ""), 10, 64)
        if e != nil {
            session.ChannelMessageSend(msg.ChannelID, ":x: Please check your format")
            return
        }

        if min <= 0 || max <= 0 {
            session.ChannelMessageSend(msg.ChannelID, ":x: Only positive numbers are allowed")
            return
        }

        if min >= max {
            session.ChannelMessageSend(msg.ChannelID, ":x: Number ranges don't work like that. (`min >= max`)")
            return
        }

        session.ChannelMessageSend(
            msg.ChannelID,
            ":crystal_ball: "+strconv.Itoa(rand.Intn(int(max-min))+int(min)),
        )
    } else {
        session.ChannelMessageSend(msg.ChannelID, ":x: You need to pass two numbers")
    }
}
