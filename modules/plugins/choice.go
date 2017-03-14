package plugins

import (
    "github.com/bwmarrin/discordgo"
    "math/rand"
    "strings"
    "unicode"
    "github.com/Seklfreak/Robyul2/helpers"
)

type Choice struct{}

func (c *Choice) Commands() []string {
    return []string{
        "choose",
        "choice",
    }
}

func (c *Choice) Init(session *discordgo.Session) {

}

func (c *Choice) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    lastQuote := rune(0)
    f := func(c rune) bool {
        switch {
        case c == lastQuote:
            lastQuote = rune(0)
            return false
        case lastQuote != rune(0):
            return false
        case unicode.In(c, unicode.Quotation_Mark):
            lastQuote = c
            return false
        default:
            return unicode.IsSpace(c)

        }
    }
    choices := strings.FieldsFunc(content, f)

    if len(choices) <= 1 {
        _, err := session.ChannelMessageSend(msg.ChannelID, "You need to pass multiple arguments!")
        helpers.Relax(err)
        return
    }

    choice := choices[rand.Intn(len(choices))]
    choice = strings.Trim(choice, "\"")
    choice = strings.Trim(choice, "\"")

    session.ChannelMessageSend(msg.ChannelID, "I've chosen `"+choice+"` :smiley:")
}
