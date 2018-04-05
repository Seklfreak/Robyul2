package helpers

import (
	"encoding/json"

	"sync"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
)

var (
	webhookCache     = make(map[string][]*discordgo.Webhook, 0) // map[channelID][]webhooks
	webhookCacheLock sync.Mutex
)

// TODO: move galleries and more to this instead of storing webhooks

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

// Gets webhooks for a channel (checks for permission, and uses cache)
// guildID		: the guild from which to get the webhook
// channelID	: the channel for which to get the webhook
// amount		: how many webhooks to get
func GetWebhooks(guildID, channelID string, amount int) (webhooks []*discordgo.Webhook, err error) {
	webhooks = make([]*discordgo.Webhook, 0)

	// check webhook cache
	webhookCacheLock.Lock()
	if _, ok := webhookCache[channelID]; ok && webhookCache[channelID] != nil {
		for _, cachedWebhook := range webhookCache[channelID] {
			webhooks = append(webhooks, cachedWebhook)
			if len(webhooks) >= amount {
				break
			}
		}
		cache.GetLogger().WithField("module", "webhooks").Infof(
			"got %d webhooks for #%s from cache", len(webhookCache[channelID]), channelID)
	}
	webhookCacheLock.Unlock()

	if len(webhooks) >= amount {
		return webhooks, nil
	}

	channelPermissions, err := cache.GetSession().State.UserChannelPermissions(cache.GetSession().State.User.ID, channelID)
	if err != nil {
		return webhooks, err
	}

	// checks if we are allowed to manage webhooks
	if channelPermissions&discordgo.PermissionManageWebhooks != discordgo.PermissionManageWebhooks &&
		channelPermissions&discordgo.PermissionAdministrator != discordgo.PermissionAdministrator {
		return webhooks, errors.New("no permission to manage webhooks")
	}

	// tries to use existing webhooks
	existingWebhooks, err := cache.GetSession().ChannelWebhooks(channelID)
	if err != nil {
		return webhooks, err
	}
	if existingWebhooks != nil && len(existingWebhooks) > 0 {
		for _, existingWebhook := range existingWebhooks {
			webhooks = append(webhooks, existingWebhook)
			if len(webhooks) >= amount {
				break
			}
		}
		cache.GetLogger().WithField("module", "webhooks").Infof(
			"got %d webhooks for #%s from existing webhooks", len(existingWebhooks), channelID)
	}

	// creates new webhooks as needed
	for {
		if len(webhooks) >= amount {
			break
		}
		newWebhook, err := cache.GetSession().WebhookCreate(channelID, "Robyul Webhook", "")
		if err != nil {
			return webhooks, err
		}
		webhooks = append(webhooks, newWebhook)
		cache.GetLogger().WithField("module", "webhooks").Infof(
			"created a new webhook for #%s", channelID)
	}

	// cache new webhooks
	webhookCacheLock.Lock()
	if _, ok := webhookCache[channelID]; ok && webhookCache[channelID] != nil {
		for _, newWebhook := range webhooks {
			alreadyCached := false
			for _, cachedWebhook := range webhookCache[channelID] {
				if cachedWebhook.ID == newWebhook.ID {
					alreadyCached = true
					break
				}
			}
			if !alreadyCached {
				webhookCache[channelID] = append(webhookCache[channelID], newWebhook)
			}
		}
	} else {
		webhookCache[channelID] = webhooks
	}
	webhookCacheLock.Unlock()

	return webhooks, nil
}
