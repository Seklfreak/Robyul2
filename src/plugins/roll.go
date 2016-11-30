package plugins

import (
    "github.com/bwmarrin/discordgo"
    "strings"
    "math/rand"
    "strconv"
    "regexp"
)

type Roll struct{}

func (r Roll) Name() string {
    return "Roll"
}

func (r Roll) Description() string {
    return "Roll a random number"
}

func (r Roll) Commands() map[string]string {
    return map[string]string{
        "roll" : "<min> <max>",
    }
}

func (r Roll) Init(session *discordgo.Session) {

}

func (r Roll) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    regex := regexp.MustCompile(`\D`)
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

        session.ChannelMessageSend(
            msg.ChannelID,
            ":crystal_ball: " + strconv.Itoa(rand.Intn(int(max - min)) + int(min)),
        )
    } else {
        session.ChannelMessageSend(msg.ChannelID, "Please check if your call was correct :frowning:")
    }
}