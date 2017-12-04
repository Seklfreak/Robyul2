package migrations

import (
	"context"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
)

func m43_create_elastic_vanityinvite_click_index() {
	if !cache.HasElastic() {
		return
	}

	exists, err := cache.GetElastic().TypeExists().Index(models.ElasticIndex).Type(models.ElasticTypeVanityInviteClick).Do(context.Background())
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
			"VanityInviteName": map[string]interface{}{
				"type": "text",
			},
			"GuildID": map[string]interface{}{
				"type": "text",
			},
			"Referer": map[string]interface{}{
				"type": "text",
				"fields": map[string]interface{}{
					"keyword": map[string]interface{}{
						"type": "keyword",
					},
				},
			},
		},
	}

	elastic := cache.GetElastic()
	createIndex, err := elastic.PutMapping().Index(models.ElasticIndex).Type(models.ElasticTypeVanityInviteClick).BodyJson(mapping).Do(context.Background())
	if err != nil {
		panic(err)
	}
	if !createIndex.Acknowledged {
		cache.GetLogger().WithField("module", "migrations").Error("ElasticSearch index not acknowledged")
	}
}
