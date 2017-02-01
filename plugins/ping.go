package plugins

import (
    "git.lukas.moe/sn0w/Karen/helpers"
    "github.com/sn0w/discordgo"
    "strconv"
    "time"
)

type Ping struct{}

func (p Ping) Commands() []string {
    return []string{
        "ping",
    }
}

func (p Ping) Init(session *discordgo.Session) {

}

func (p Ping) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    start := time.Now()

    m, err := session.ChannelMessageSend(msg.ChannelID, ":ping_pong: Pong! :grin:")
    helpers.Relax(err)

    end := time.Now()
    session.ChannelMessageEdit(
        msg.ChannelID,
        m.ID,
        m.Content + " (" + strconv.Itoa(int(end.Sub(start) / time.Millisecond) / 2) + "ms)",
    )
}
