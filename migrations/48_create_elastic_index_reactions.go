package migrations

import (
	"context"

	"github.com/Seklfreak/Robyul2/cache"
)

func m48_create_elastic_index_reactions() {
	if !cache.HasElastic() {
		return
	}

	elastic := cache.GetElastic()
	exists, err := elastic.IndexExists("robyul-reactions").Do(context.Background())
	if err != nil {
		panic(err)
	}
	if exists {
		return
	}

	reactionMapping := map[string]interface{}{
		"mappings": map[string]interface{}{
			"doc": map[string]interface{}{
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
			},
		},
	}

	index, err := elastic.CreateIndex("robyul-reactions").BodyJson(reactionMapping).Do(context.Background())
	if err != nil {
		panic(err)
	}
	if !index.Acknowledged {
		cache.GetLogger().WithField("module", "migrations").Error("ElasticSearch index not acknowledged")
	}
}
