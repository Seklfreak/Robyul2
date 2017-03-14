package plugins

import (
    "github.com/bwmarrin/discordgo"
    "strings"
)

type Leet struct{}

var leetReplacements = map[string]string{
    "A": "@", "B": "8", "C": "C",
    "D": "D", "E": "3", "F": "ƒ",
    "G": "6", "H": "H", "I": "1",
    "J": "J", "K": "|<", "L": "L",
    "M": "/\\/\\", "N": "|/|", "O": "0",
    "P": "¶", "Q": "9", "R": "R",
    "S": "5", "T": "T", "U": "µ",
    "V": "\\//", "W": "\\/\\/", "X": "%",
    "Y": "¥", "Z": "Z",
}

func (l *Leet) Commands() []string {
    return []string{
        "leet",
        "l33t",
    }
}

func (l *Leet) Init(session *discordgo.Session) {

}

func (l *Leet) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    content = strings.ToUpper(content)

    for ascii, leet := range leetReplacements {
        content = strings.Replace(content, ascii, leet, -1)
    }

    session.ChannelMessageSend(msg.ChannelID, "```\n"+content+"\n```")
}
