package migrations

import (
	"context"

	"github.com/Seklfreak/Robyul2/cache"
)

func m50_create_elastic_vanity_invite_clicks() {
	if !cache.HasElastic() {
		return
	}

	elastic := cache.GetElastic()
	exists, err := elastic.IndexExists("robyul-vanity_invite_clicks").Do(context.Background())
	if err != nil {
		panic(err)
	}
	if exists {
		return
	}

	vanityInviteClickMapping := map[string]interface{}{
		"mappings": map[string]interface{}{
			"doc": map[string]interface{}{
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
			},
		},
	}

	index, err := elastic.CreateIndex("robyul-vanity_invite_clicks").BodyJson(vanityInviteClickMapping).Do(context.Background())
	if err != nil {
		panic(err)
	}
	if !index.Acknowledged {
		cache.GetLogger().WithField("module", "migrations").Error("ElasticSearch index not acknowledged")
	}
}
