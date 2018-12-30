package migrations

import (
	"context"

	"github.com/Seklfreak/Robyul2/cache"
)

func m45_create_elastic_index_messages() {
	if !cache.HasElastic() {
		return
	}

	elastic := cache.GetElastic()
	exists, err := elastic.IndexExists("robyul-messages-v2").Do(context.Background())
	if err != nil {
		panic(err)
	}
	if exists {
		return
	}

	messageMapping := map[string]interface{}{
		"mappings": map[string]interface{}{
			"doc": map[string]interface{}{
				"properties": map[string]interface{}{
					"CreatedAt": map[string]interface{}{
						"type": "date",
					},
					"MessageID": map[string]interface{}{
						"type": "text",
						"fields": map[string]interface{}{
							"keyword": map[string]interface{}{
								"type": "keyword",
							},
						},
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
						"fields": map[string]interface{}{
							"keyword": map[string]interface{}{
								"type": "keyword",
							},
						},
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
			},
		},
	}

	index, err := elastic.CreateIndex("robyul-messages-v2").BodyJson(messageMapping).Do(context.Background())
	if err != nil {
		panic(err)
	}
	if !index.Acknowledged {
		cache.GetLogger().WithField("module", "migrations").Error("ElasticSearch index not acknowledged")
	}
}
