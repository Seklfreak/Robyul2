package migrations

import (
	"context"

	"github.com/Seklfreak/Robyul2/cache"
)

func m55_create_elastic_index_eventlogs() {
	if !cache.HasElastic() {
		return
	}

	elastic := cache.GetElastic()
	exists, err := elastic.IndexExists("robyul-eventlogs").Do(context.Background())
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
						"fields": map[string]interface{}{
							"keyword": map[string]interface{}{
								"type": "keyword",
							},
						},
					},
					"TargetID": map[string]interface{}{
						"type": "text",
						"fields": map[string]interface{}{
							"keyword": map[string]interface{}{
								"type": "keyword",
							},
						},
					},
					"TargetType": map[string]interface{}{
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
					"ActionType": map[string]interface{}{
						"type": "text",
						"fields": map[string]interface{}{
							"keyword": map[string]interface{}{
								"type": "keyword",
							},
						},
					},
					"Reason": map[string]interface{}{
						"type": "text",
					},
					"Changes": map[string]interface{}{
						"type": "nested",
						"properties": map[string]interface{}{
							"Key":      map[string]interface{}{"type": "text"},
							"OldValue": map[string]interface{}{"type": "text"},
							"NewValue": map[string]interface{}{"type": "text"},
						},
					},
					"Options": map[string]interface{}{
						"type": "nested",
						"properties": map[string]interface{}{
							"Key": map[string]interface{}{
								"type": "text",
							},
							"Value": map[string]interface{}{
								"type": "text",
							},
						},
					},
					"WaitingFor": map[string]interface{}{
						"properties": map[string]interface{}{
							"AuditLogBackfill": map[string]interface{}{
								"type":       "boolean",
								"null_value": false,
							},
						},
					},
					"EventlogMessages": map[string]interface{}{
						"type": "text",
					},
				},
			},
		},
	}

	index, err := elastic.CreateIndex("robyul-eventlogs").BodyJson(messageMapping).Do(context.Background())
	if err != nil {
		panic(err)
	}
	if !index.Acknowledged {
		cache.GetLogger().WithField("module", "migrations").Error("ElasticSearch index not acknowledged")
	}
}
