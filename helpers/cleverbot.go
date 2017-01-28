package helpers

import (
    "github.com/sn0w/discordgo"
    "github.com/ugjka/cleverbot-go"
)

const API_ID = "Karen Discord-Bot <lukas.breuer@outlook.com> (https://meetkaren.xyz)"

// cleverbotSessions stores all cleverbot connections
var cleverbotSessions map[string]*cleverbot.Session

// CleverbotSend sends a message to cleverbot and responds with it's answer.
func CleverbotSend(session *discordgo.Session, channel string, message string) {
    var msg string

    if cleverbotSessions[channel] == nil {
        if len(cleverbotSessions) == 0 {
            cleverbotSessions = make(map[string]*cleverbot.Session)
        }

        cleverbotSessions[channel] = cleverbot.New(API_ID)
    }

    response, err := cleverbotSessions[channel].Ask(message)
    if err != nil {
        msg = "Error :frowning:\n```\n" + err.Error() + "\n```"
    } else {
        msg = response
    }

    session.ChannelMessageSend(channel, msg)
}

// CleverbotRefreshSession refreshes the cleverbot session for said channel
func CleverbotRefreshSession(channel string) {
    cleverbotSessions[channel] = cleverbot.New(API_ID)
}
