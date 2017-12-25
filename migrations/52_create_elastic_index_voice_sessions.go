package migrations

import (
	"context"

	"github.com/Seklfreak/Robyul2/cache"
)

func m52_create_elastic_index_voice_sessions() {
	if !cache.HasElastic() {
		return
	}

	elastic := cache.GetElastic()
	exists, err := elastic.IndexExists("robyul-voice_session").Do(context.Background())
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
					"GuildID": map[string]interface{}{
						"type": "text",
					},
					"ChannelID": map[string]interface{}{
						"type": "text",
						"fields": map[string]interface{}{
							"keyword": map[string]interface{}{
								"type": "keyword",
							},
						},
					},
					"UserID": map[string]interface{}{
						"type": "text",
						"fields": map[string]interface{}{
							"keyword": map[string]interface{}{
								"type": "keyword",
							},
						},
					},
					"JoinTime": map[string]interface{}{
						"type": "date",
					},
					"LeaveTime": map[string]interface{}{
						"type": "date",
					},
					"DurationSeconds": map[string]interface{}{
						"type": "long",
					},
				},
			},
		},
	}

	index, err := elastic.CreateIndex("robyul-voice_session").BodyJson(messageMapping).Do(context.Background())
	if err != nil {
		panic(err)
	}
	if !index.Acknowledged {
		cache.GetLogger().WithField("module", "migrations").Error("ElasticSearch index not acknowledged")
	}
}
