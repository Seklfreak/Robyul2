package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
)

func UpdateBotlists() {
	defer Recover()

	numOfGuilds := len(cache.GetSession().State.Guilds)

	err := updateDiscordBotsOrg(numOfGuilds)
	if err != nil {
		RelaxLog(err)
	}
}

// https://discordbots.org/bot/283848369250500608
func updateDiscordBotsOrg(numOfGuilds int) (err error) {
	if GetConfig().Path("botlists.discordbotsorg-token").Data().(string) == "" {
		return nil
	}

	token := GetConfig().Path("botlists.discordbotsorg-token").Data().(string)

	if token == "" {
		return nil
	}

	url := fmt.Sprintf("https://discordbots.org/api/bots/%v/stats", cache.GetSession().State.User.ID)

	payload := struct {
		ServerCount int `json:"server_count"`
	}{
		ServerCount: numOfGuilds,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", token)

	client := &http.Client{
		Timeout: time.Duration(10 * time.Second),
	}

	_, err = client.Do(request)
	if err != nil {
		return err
	}

	cache.GetLogger().WithField("module", "botlists").Infof("Updated discordbots.org: %d servers", numOfGuilds)
	return nil
}
