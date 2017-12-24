package migrations

import (
	"context"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/olivere/elastic"
)

func m51_reindex_elasticv5_to_v6() {
	if !cache.HasElastic() {
		return
	}

	elasticClient := cache.GetElastic()
	exists, err := elasticClient.IndexExists("robyul").Do(context.Background())
	if err != nil {
		panic(err)
	}
	if !exists {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("reindexing ElasticSearch indexes")

	src := elastic.NewReindexSource().Index("robyul").Type("message")
	dst := elastic.NewReindexDestination().Index("robyul-messages").Type("doc")
	res, err := elasticClient.Reindex().Source(src).Destination(dst).Refresh("true").Do(context.Background())
	if err != nil {
		panic(err)
	}
	cache.GetLogger().WithField("module", "migrations").Infof(
		"Reindexed a total of %d message documents", res.Total)

	src = elastic.NewReindexSource().Index("robyul").Type("join")
	dst = elastic.NewReindexDestination().Index("robyul-joins").Type("doc")
	res, err = elasticClient.Reindex().Source(src).Destination(dst).Refresh("true").Do(context.Background())
	if err != nil {
		panic(err)
	}
	cache.GetLogger().WithField("module", "migrations").Infof(
		"Reindexed a total of %d join documents", res.Total)

	src = elastic.NewReindexSource().Index("robyul").Type("leave")
	dst = elastic.NewReindexDestination().Index("robyul-leaves").Type("doc")
	res, err = elasticClient.Reindex().Source(src).Destination(dst).Refresh("true").Do(context.Background())
	if err != nil {
		panic(err)
	}
	cache.GetLogger().WithField("module", "migrations").Infof(
		"Reindexed a total of %d leave documents", res.Total)

	src = elastic.NewReindexSource().Index("robyul").Type("reaction")
	dst = elastic.NewReindexDestination().Index("robyul-reactions").Type("doc")
	res, err = elasticClient.Reindex().Source(src).Destination(dst).Refresh("true").Do(context.Background())
	if err != nil {
		panic(err)
	}
	cache.GetLogger().WithField("module", "migrations").Infof(
		"Reindexed a total of %d reaction documents", res.Total)

	src = elastic.NewReindexSource().Index("robyul").Type("presence_update")
	dst = elastic.NewReindexDestination().Index("robyul-presence_updates").Type("doc")
	res, err = elasticClient.Reindex().Source(src).Destination(dst).Refresh("true").Do(context.Background())
	if err != nil {
		panic(err)
	}
	cache.GetLogger().WithField("module", "migrations").Infof(
		"Reindexed a total of %d presence update documents", res.Total)

	src = elastic.NewReindexSource().Index("robyul").Type("vanity_invite_click")
	dst = elastic.NewReindexDestination().Index("robyul-vanity_invite_clicks").Type("doc")
	res, err = elasticClient.Reindex().Source(src).Destination(dst).Refresh("true").Do(context.Background())
	if err != nil {
		panic(err)
	}
	cache.GetLogger().WithField("module", "migrations").Infof(
		"Reindexed a total of %d vanity invite click documents", res.Total)

	index, err := elasticClient.DeleteIndex("robyul").Do(context.Background())
	if err != nil {
		panic(err)
	}
	if !index.Acknowledged {
		cache.GetLogger().WithField("module", "migrations").Error("ElasticSearch index not acknowledged")
	}
}
