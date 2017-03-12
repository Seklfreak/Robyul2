package plugins

import (
    "git.lukas.moe/sn0w/Karen/helpers"
    "github.com/bwmarrin/discordgo"
    "fmt"
    "strings"
)

type Headpat struct{}

func (h *Headpat) Commands() []string {
    return []string{
        "headpat",
        "pat",
    }
}

func (l *Headpat) Init(session *discordgo.Session) {

}

func (l *Headpat) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    // Check mentions in the message
    mentionUsers := len(msg.Mentions)
    mentionRoles := len(msg.MentionRoles)

    // Delete spaces from params
    params := strings.Trim(content, " ")

    // Case 1: pat yourself
    if params == "me" || mentionUsers == 1 && (msg.Author.Username == msg.Mentions[0].Username) {
        session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf(
        "%s https://media.giphy.com/media/wUArrd4mE3pyU/giphy.gif",
        helpers.GetText("bot.mentions.pat-yourself"),
    ))
        return
    }
    // Case 2: pat @User#1234
    if mentionUsers == 1 {
        session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf(
        "%s , %s",
        msg.Mentions[0].Username,
        helpers.GetText("triggers.headpat.link"),
    ))
        return
    }
    // Case 3: pat multiple users
        if mentionUsers > 1 ||  mentionRoles >= 1 {
        session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.mentions.pat-group"))
        return
    }
    // Case 4: no params || wrong params
    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.mentions.who-to-pat"))

}
