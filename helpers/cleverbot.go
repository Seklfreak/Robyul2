package helpers

import (
    "github.com/bwmarrin/discordgo"
    "github.com/ugjka/cleverbot-go"
)

// cleverbotSessions stores all cleverbot connections
var cleverbotSessions map[string]*cleverbot.Session

// CleverbotSend sends a message to cleverbot and responds with it's answer.
func CleverbotSend(session *discordgo.Session, channel string, message string) {
    var msg string

    if _, e := cleverbotSessions[channel]; !e {
        if len(cleverbotSessions) == 0 {
            cleverbotSessions = make(map[string]*cleverbot.Session)
        }

        CleverbotRefreshSession(channel)
    }

    response, err := cleverbotSessions[channel].Ask(message)
    if err != nil {
        if err == cleverbot.ErrTooManyRequests {
            msg = "I cannot talk to you right now. :speak_no_evil:\n" +
                "CleverBot costs money, and the plan I'm currently on has no requests left.\n" +
                "If you want to help 0xFADED buying larger plans, cosider making a donation on his patreon. :innocent:\n" +
                "Link: <https://www.patreon.com/sn0w>"
        } else {
            msg = "Error :frowning:\n```\n" + err.Error() + "\n```"
        }
    } else {
        msg = response
    }

    session.ChannelMessageSend(channel, msg)
}

// CleverbotRefreshSession refreshes the cleverbot session for said channel
func CleverbotRefreshSession(channel string) {
    cleverbotSessions[channel] = cleverbot.New(
        GetConfig().Path("cleverbot.key").Data().(string),
    )
}
