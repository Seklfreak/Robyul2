package plugins

import (
    "github.com/bwmarrin/discordgo"
    "math/rand"
    "strings"
    "unicode"
    "github.com/Seklfreak/Robyul2/helpers"
    "strconv"
    "fmt"
    "time"
)

type Choice struct{}

func (c *Choice) Commands() []string {
    return []string{
        "choose",
        "choice",
        "roll",
    }
}

func (c *Choice) Init(session *discordgo.Session) {

}

func (c *Choice) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    switch command {
    case "choose", "choice": // [p]choose <option a> <option b> [...]
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
            _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
            helpers.Relax(err)
            return
        }

        choice := choices[rand.Intn(len(choices))]
        choice = strings.Trim(choice, "\"")
        choice = strings.Trim(choice, "\"")

        _, err := session.ChannelMessageSend(msg.ChannelID, "I've chosen `"+choice+"` :smiley:")
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
