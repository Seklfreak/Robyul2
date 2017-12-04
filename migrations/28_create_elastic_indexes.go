package migrations

import (
	"context"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
)

func m28_create_elastic_indexes() {
	if !cache.HasElastic() {
		return
	}

	exists, err := cache.GetElastic().IndexExists(models.ElasticIndex).Do(context.Background())
	if err != nil {
		panic(err)
	}
	if exists {
		return
	}

	messageMapping := map[string]interface{}{
		"properties": map[string]interface{}{
			"CreatedAt": map[string]interface{}{
				"type": "date",
			},
			"MessageID": map[string]interface{}{
				"type": "text",
			},
			"Content": map[string]interface{}{
				"type":  "text",
				"index": false,
			},
			"ContentLength": map[string]interface{}{
				"type": "long",
			},
			"Attachments": map[string]interface{}{
				"type": "text",
			},
			"UserID": map[string]interface{}{
				"type": "text",
			},
			"GuildID": map[string]interface{}{
				"type": "text",
			},
			"ChannelID": map[string]interface{}{
				"type": "text",
			},
			"Embeds": map[string]interface{}{
				"type": "integer",
			},
		},
	}

	joinMapping := map[string]interface{}{
		"properties": map[string]interface{}{
			"CreatedAt": map[string]interface{}{
				"type": "date",
			},
			"GuildID": map[string]interface{}{
				"type": "text",
			},
			"UserID": map[string]interface{}{
				"type": "text",
			},
		},
	}

	leaveMapping := map[string]interface{}{
		"properties": map[string]interface{}{
			"CreatedAt": map[string]interface{}{
				"type": "date",
			},
			"GuildID": map[string]interface{}{
				"type": "text",
			},
			"UserID": map[string]interface{}{
				"type": "text",
			},
		},
	}

	reactionMapping := map[string]interface{}{
		"properties": map[string]interface{}{
			"CreatedAt": map[string]interface{}{
				"type": "date",
			},
			"UserID": map[string]interface{}{
				"type": "text",
			},
			"MessageID": map[string]interface{}{
				"type": "text",
			},
			"ChannelID": map[string]interface{}{
				"type": "text",
			},
			"GuildID": map[string]interface{}{
				"type": "text",
			},
			"EmojiID": map[string]interface{}{
				"type": "text",
			},
			"EmojiName": map[string]interface{}{
				"type": "text",
			},
		},
	}

	elastic := cache.GetElastic()
	_, err = elastic.CreateIndex(models.ElasticIndex).Do(context.Background())
	if err != nil {
		panic(err)
	}

	createMapping, err := elastic.PutMapping().Index(models.ElasticIndex).Type(models.ElasticTypeMessage).BodyJson(messageMapping).Do(context.Background())
	if err != nil {
		panic(err)
	}
	if !createMapping.Acknowledged {
		cache.GetLogger().WithField("module", "migrations").Error("ElasticSearch index not acknowledged")
	}

	createMapping, err = elastic.PutMapping().Index(models.ElasticIndex).Type(models.ElasticTypeJoin).BodyJson(joinMapping).Do(context.Background())
	if err != nil {
		panic(err)
	}
	if !createMapping.Acknowledged {
		cache.GetLogger().WithField("module", "migrations").Error("ElasticSearch index not acknowledged")
	}

	createMapping, err = elastic.PutMapping().Index(models.ElasticIndex).Type(models.ElasticTypeLeave).BodyJson(leaveMapping).Do(context.Background())
	if err != nil {
		panic(err)
	}
	if !createMapping.Acknowledged {
		cache.GetLogger().WithField("module", "migrations").Error("ElasticSearch index not acknowledged")
	}

	createMapping, err = elastic.PutMapping().Index(models.ElasticIndex).Type(models.ElasticTypeReaction).BodyJson(reactionMapping).Do(context.Background())
	if err != nil {
		panic(err)
	}
	if !createMapping.Acknowledged {
		cache.GetLogger().WithField("module", "migrations").Error("ElasticSearch index not acknowledged")
	}
}
