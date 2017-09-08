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

	mapping := map[string]interface{}{
		"mappings": map[string]interface{}{
			models.ElasticTypeMessage: map[string]interface{}{
				"properties": map[string]interface{}{
					"CreatedAt": map[string]interface{}{
						"type": "date",
					},
					"MessageID": map[string]interface{}{
						"type": "string",
					},
					"Content": map[string]interface{}{
						"type":  "string",
						"index": "not_analyzed",
					},
					"ContentLength": map[string]interface{}{
						"type": "long",
					},
					"Attachments": map[string]interface{}{
						"type": "string",
					},
					"UserID": map[string]interface{}{
						"type": "string",
					},
					"GuildID": map[string]interface{}{
						"type": "string",
					},
					"ChannelID": map[string]interface{}{
						"type": "string",
					},
					"Embeds": map[string]interface{}{
						"type": "integer",
					},
				},
			},
			models.ElasticTypeJoin: map[string]interface{}{
				"properties": map[string]interface{}{
					"CreatedAt": map[string]interface{}{
						"type": "date",
					},
					"GuildID": map[string]interface{}{
						"type": "string",
					},
					"UserID": map[string]interface{}{
						"type": "string",
					},
				},
			},
			models.ElasticTypeLeave: map[string]interface{}{
				"properties": map[string]interface{}{
					"CreatedAt": map[string]interface{}{
						"type": "date",
					},
					"GuildID": map[string]interface{}{
						"type": "string",
					},
					"UserID": map[string]interface{}{
						"type": "string",
					},
				},
			},
			models.ElasticTypeReaction: map[string]interface{}{
				"properties": map[string]interface{}{
					"CreatedAt": map[string]interface{}{
						"type": "date",
					},
					"UserID": map[string]interface{}{
						"type": "string",
					},
					"MessageID": map[string]interface{}{
						"type": "string",
					},
					"ChannelID": map[string]interface{}{
						"type": "string",
					},
					"GuildID": map[string]interface{}{
						"type": "string",
					},
					"EmojiID": map[string]interface{}{
						"type": "string",
					},
					"EmojiName": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
	}

	elastic := cache.GetElastic()
	createIndex, err := elastic.CreateIndex(models.ElasticIndex).BodyJson(mapping).Do(context.Background())
	if err != nil {
		panic(err)
	}
	if !createIndex.Acknowledged {
		cache.GetLogger().WithField("module", "migrations").Error("ElasticSearch index not acknowledged")
	}
}
