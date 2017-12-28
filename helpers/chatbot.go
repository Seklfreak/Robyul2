package helpers

import (
	"net/url"

	"github.com/Jeffail/gabs"
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/bwmarrin/discordgo"
)

func ChatbotSend(session *discordgo.Session, channel string, message string) {
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
