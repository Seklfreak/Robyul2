package migrations

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/cheggaaa/pb"
	"github.com/globalsign/mgo/bson"
	"github.com/gorethink/gorethink"
)

func m70_migration_table_weather_last_locations() {
	if !TableExists("weather_last_locations") {
		return
	}

	cache.GetLogger().WithField("module", "migrations").Info("moving weather_last_locations to mongodb")

	cursor, err := gorethink.Table("weather_last_locations").Count().Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var numberOfElements int
	cursor.One(&numberOfElements)

	cursor, err = gorethink.Table("weather_last_locations").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	var rethinkdbEntry struct {
		ID     string  `rethink:"id,omitempty"`
		UserID string  `rethink:"user_id"`
		Lat    float64 `rethink:"lat"`
		Lng    float64 `rethink:"lng"`
		Text   string  `rethink:"text"`
	}
	bar := pb.StartNew(numberOfElements)
	for cursor.Next(&rethinkdbEntry) {
		err = helpers.MDbUpsertWithoutLogging(
			models.WeatherLastLocationsTable,
			bson.M{"userid": rethinkdbEntry.UserID},
			models.WeatherLastLocationEntry{
				UserID: rethinkdbEntry.UserID,
				Lat:    rethinkdbEntry.Lat,
				Lng:    rethinkdbEntry.Lng,
				Text:   rethinkdbEntry.Text,
			},
		)
		if err != nil {
			panic(err)
		}

		bar.Increment()
	}

	if cursor.Err() != nil {
		panic(err)
	}
	bar.Finish()

	cache.GetLogger().WithField("module", "migrations").Info("dropping rethinkdb weather_last_locations")
	_, err = gorethink.TableDrop("weather_last_locations").Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
}
