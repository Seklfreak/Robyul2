package plugins

import (
    "git.lukas.moe/sn0w/Karen/helpers"
    "github.com/sn0w/discordgo"
    "regexp"
)

type Minecraft struct{}

func (m Minecraft) Commands() []string {
    return []string{
        "minecraft",
        "mc",
    }
}

func (m Minecraft) Init(session *discordgo.Session) {

}

func (m Minecraft) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    // Deferred error handler
    defer func() {
        err := recover()

        if err != nil {
            if regexp.MustCompile("(?i)expected status 200.*").Match([]byte(err.(string))) {
                session.ChannelMessageSend(msg.ChannelID, "Make sure that name is correct. \n I didn't find a thing :neutral_face:")
                return
            }
        }

        panic(err)
    }()

    // Set typing
    session.ChannelTyping(msg.ChannelID)

    // Request to catch server errors and 404's
    url := "https://minotar.net/body/" + content + "/300.png"
    helpers.NetGet(url)

    // If NetGet didn't panic send the url
    session.ChannelMessageSend(msg.ChannelID, "Here you go :smiley: \n " + url)

}
