package migrations

import (
	"context"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
)

func m29_create_elastic_presence_update_index() {
	if !cache.HasElastic() {
		return
	}

	exists, err := cache.GetElastic().TypeExists().Index(models.ElasticIndex).Type(models.ElasticTypePresenceUpdate).Do(context.Background())
	if err != nil {
		panic(err)
	}
	if exists {
		return
	}

	mapping := map[string]interface{}{
		"properties": map[string]interface{}{
			"CreatedAt": map[string]interface{}{
				"type": "date",
			},
			"UserID": map[string]interface{}{
				"type": "string",
			},
			"GameType": map[string]interface{}{
				"type": "long",
			},
			"GameTypeV2": map[string]interface{}{
				"type": "string",
			},
			"GameName": map[string]interface{}{
				"type": "string",
			},
			"GameURL": map[string]interface{}{
				"type": "string",
			},
			"Status": map[string]interface{}{
				"type": "string",
			},
		},
	}

	elastic := cache.GetElastic()
	createIndex, err := elastic.PutMapping().Index(models.ElasticIndex).Type(models.ElasticTypePresenceUpdate).BodyJson(mapping).Do(context.Background())
	if err != nil {
		panic(err)
	}
	if !createIndex.Acknowledged {
		cache.GetLogger().WithField("module", "migrations").Error("ElasticSearch index not acknowledged")
	}
}
