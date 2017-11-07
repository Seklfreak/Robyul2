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
			msg = "I cannot talk to you right now. <:googlespeaknoevil:317036753074651139>\n"
		} else {
			msg = "Error <:blobfrowningbig:317028438693117962>\n```\n" + err.Error() + "\n```"
			CleverbotRefreshSession(channel)
		}
	} else {
		msg = response
	}

	SendMessage(channel, msg)
}

// CleverbotRefreshSession refreshes the cleverbot session for said channel
func CleverbotRefreshSession(channel string) {
	cleverbotSessions[channel] = cleverbot.New(
		GetConfig().Path("cleverbot.key").Data().(string),
	)
}
