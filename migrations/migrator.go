package migrations

import (
	"reflect"
	"runtime"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
)

var migrations = []helpers.Callback{
	m28_create_elastic_indexes,
	m29_create_elastic_presence_update_index,
	m43_create_elastic_vanityinvite_click_index,
	m45_create_elastic_index_messages,
	m46_create_elastic_index_joins,
	m47_create_elastic_index_leaves,
	m48_create_elastic_index_reactions,
	m49_create_elastic_index_presence_updates,
	m50_create_elastic_vanity_invite_clicks,
	m51_reindex_elasticv5_to_v6,
	m52_create_elastic_index_voice_sessions,
	m55_create_elastic_index_eventlogs,
}

// Run executes all registered migrations
func Run() {
	log := cache.GetLogger()
	log.WithField("module", "migrator").Info("Running migrations...")
	for _, migration := range migrations {
		migrationName := runtime.FuncForPC(
			reflect.ValueOf(migration).Pointer(),
		).Name()

		log.WithField("module", "migrator").Info("Running " + migrationName)
		migration()
	}

	log.WithField("module", "migrator").Info("Migrations finished!")
}
