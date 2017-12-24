package migrations

import (
	"context"

	"github.com/Seklfreak/Robyul2/cache"
)

func m49_create_elastic_index_presence_updates() {
	if !cache.HasElastic() {
		return
	}

	elastic := cache.GetElastic()
	exists, err := elastic.IndexExists("robyul-presence_updates").Do(context.Background())
	if err != nil {
		panic(err)
	}
	if exists {
		return
	}

	presenceUpdateMapping := map[string]interface{}{
		"mappings": map[string]interface{}{
			"doc": map[string]interface{}{
				"properties": map[string]interface{}{
					"CreatedAt": map[string]interface{}{
						"type": "date",
					},
					"UserID": map[string]interface{}{
						"type": "text",
					},
					"GameType": map[string]interface{}{
						"type": "long",
					},
					"GameTypeV2": map[string]interface{}{
						"type": "text",
					},
					"GameName": map[string]interface{}{
						"type": "text",
					},
					"GameURL": map[string]interface{}{
						"type": "text",
					},
					"Status": map[string]interface{}{
						"type": "text",
					},
				},
			},
		},
	}

	index, err := elastic.CreateIndex("robyul-presence_updates").BodyJson(presenceUpdateMapping).Do(context.Background())
	if err != nil {
		panic(err)
	}
	if !index.Acknowledged {
		cache.GetLogger().WithField("module", "migrations").Error("ElasticSearch index not acknowledged")
	}
}
