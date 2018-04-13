package helpers

import (
	"encoding/json"

	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack"
)

// Executes a webhook and waites for the response
// id	: the ID of the webhook to use
// token		: the token of the webhook to use
// data			: webhook params to send
func WebhookExecuteWithResult(id, token string, data *discordgo.WebhookParams) (message *discordgo.Message, err error) {
	uri := discordgo.EndpointWebhookToken(id, token) + "?wait=true"

	result, err := cache.GetSession().RequestWithBucketID("POST", uri, data, discordgo.EndpointWebhookToken("", ""))
	if err != nil {
		return message, err
	}

	err = json.Unmarshal(result, &message)
	return message, err
}

// Gets a webhook for a channel (checks for permission, and uses cache)
// guildID		: the guild from which to get the webhook
// channelID	: the channel for which to get the webhook
func GetWebhook(guildID, channelID string) (webhook *discordgo.Webhook, err error) {
	key := "robyul-discord:webhooks-cache:" + guildID + ":" + channelID
	redis := cache.GetRedisClient()

	// check redis webhook cache
	keyExists, err := redis.Exists(key).Result()
	RelaxLog(err)
	if err == nil && keyExists >= 1 {
		keyBytes, err := redis.Get(key).Bytes()
		RelaxLog(err)
		if err == nil {
			err = msgpack.Unmarshal(keyBytes, &webhook)
			RelaxLog(err)
			if err == nil && webhook.ID != "" && webhook.Token != "" {
				cache.GetLogger().WithField("module", "webhooks").Infof(
					"got webhook for #%s from cache", channelID)
				return webhook, nil
			}
		}
	}

	// get robyul's permissions for the target channel
	channelPermissions, err := cache.GetSession().State.UserChannelPermissions(cache.GetSession().State.User.ID, channelID)
	if err != nil {
		return nil, err
	}

	// checks if we are allowed to manage webhooks
	if channelPermissions&discordgo.PermissionManageWebhooks != discordgo.PermissionManageWebhooks &&
		channelPermissions&discordgo.PermissionAdministrator != discordgo.PermissionAdministrator {
		return nil, errors.New("no permission to manage webhooks")
	}

	// try to use existing webhooks
	existingWebhooks, err := cache.GetSession().ChannelWebhooks(channelID)
	if err != nil {
		return nil, err
	}
	if existingWebhooks != nil && len(existingWebhooks) > 0 {
		for _, existingWebhook := range existingWebhooks {
			if existingWebhook.ID != "" && existingWebhook.Token != "" {
				webhook = existingWebhook
				cache.GetLogger().WithField("module", "webhooks").Infof(
					"got webhook for #%s from existing webhooks", channelID)
			}
		}
	}

	// create new webhook if no existing webhook
	if webhook == nil || webhook.ID == "" || webhook.Token == "" {
		webhook, err = cache.GetSession().WebhookCreate(channelID, "Robyul Webhook - Do Not Delete!", "")
		if err != nil {
			return nil, err
		}
		cache.GetLogger().WithField("module", "webhooks").Infof(
			"created a new webhook for #%s", channelID)
	}

	// cache and return webhook is valid
	if webhook != nil && webhook.ID != "" && webhook.Token != "" {
		// cache existing webhook for 15 minutes on redis
		newKeyBytes, err := msgpack.Marshal(webhook)
		RelaxLog(err)
		if err == nil {
			_, err = redis.Set(key, newKeyBytes, time.Minute*15).Result()
			RelaxLog(err)
		}
		return webhook, nil
	}

	// return error
	return nil, errors.New("unable to create webhook")
}
