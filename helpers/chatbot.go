package helpers

import (
	"net/url"

	"time"

	"github.com/CleverbotIO/go-cleverbot.io"
	"github.com/Jeffail/gabs"
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
)

var (
	cleverbotIOSessions = make(map[string]*cleverbot.Session, 0)
)

func ChatbotSendProgramO(session *discordgo.Session, channel string, message string) {
	defer Recover()

	var msg string

	url, err := url.Parse(GetConfig().Path("program-o.api-url").Data().(string))
	if err != nil {
		RelaxLog(err)
		return
	}

	query := url.Query()
	query.Set("convo_id", channel)
	query.Set("format", "Json")
	query.Set("debug_mode", "1")
	query.Set("debug_level", "0")
	query.Set("say", message)
	url.RawQuery = query.Encode()

	resultRaw, err := NetGetUAWithError(url.String(), DEFAULT_UA)
	if err != nil {
		cache.GetLogger().WithField("module", "chatbot").Errorf("getting a chatbot response failed: %s", err.Error())
		return
	}

	result, err := gabs.ParseJSON(resultRaw)
	if err != nil {
		cache.GetLogger().WithField("module", "chatbot").Errorf("parsing a chatbot response failed: %s", err.Error())
		return
	}

	msg = result.Path("botsay").Data().(string)

	SendMessage(channel, msg)
}

func CleverbotIORequest(channel, message string) (response string, err error) {
	var currentSession *cleverbot.Session

	if _, ok := cleverbotIOSessions[channel]; ok {
		currentSession = cleverbotIOSessions[channel]
	} else {
		cleverbotUser := GetConfig().Path("cleverbot-io.user").Data().(string)
		cleverbotKey := GetConfig().Path("cleverbot-io.key").Data().(string)
		if cleverbotUser == "" || cleverbotKey == "" {
			return "", errors.New("no cleverbot.io user or key set")
		}
		currentSession, err = cleverbot.New(
			cleverbotUser,
			cleverbotKey,
			channel+"-"+time.Now().Format("20060102150405"),
		)
		if err != nil {
			return "", err
		}
		cleverbotIOSessions[channel] = currentSession
	}

	return currentSession.Ask(message)
}

func ChatbotSendCleverbotIO(session *discordgo.Session, channel, message string, author *discordgo.User) {
	message, err := CleverbotIORequest(channel, message)
	if err != nil {
		RelaxLog(err)
		SendMessage(channel, GetText("bot.errors.chatbot"))
		return
	}

	SendMessage(channel, author.Mention()+" "+message)
}
