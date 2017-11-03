package migrations

import (
	"github.com/Seklfreak/Robyul2/helpers"
	rethink "github.com/gorethink/gorethink"
)

func m38_create_table_weather_last_locations() {
	CreateTableIfNotExists("weather_last_locations")

	rethink.Table("weather_last_locations").IndexCreate("user_id").Run(helpers.GetDB())
}
