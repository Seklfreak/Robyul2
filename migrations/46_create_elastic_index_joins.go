package migrations

import (
	"context"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
)

func m46_create_elastic_index_joins() {
	if !cache.HasElastic() {
		return
	}

	elastic := cache.GetElastic()
	exists, err := elastic.IndexExists(models.ElasticIndexJoins).Do(context.Background())
	if err != nil {
		panic(err)
	}
	if exists {
		return
	}

	joinMapping := map[string]interface{}{
		"mappings": map[string]interface{}{
			"doc": map[string]interface{}{
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
					"UsedInviteCode": map[string]interface{}{
						"type": "text",
					},
					"VanityInvite": map[string]interface{}{
						"type": "text",
					},
				},
			},
		},
	}

	index, err := elastic.CreateIndex(models.ElasticIndexJoins).BodyJson(joinMapping).Do(context.Background())
	if err != nil {
		panic(err)
	}
	if !index.Acknowledged {
		cache.GetLogger().WithField("module", "migrations").Error("ElasticSearch index not acknowledged")
	}
}
