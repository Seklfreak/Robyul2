package models

const (
	WeatherLastLocationsTable = "weather_last_locations"
)

type WeatherLastLocation struct {
	ID     string  `rethink:"id,omitempty"`
	UserID string  `rethink:"user_id"`
	Lat    float64 `rethink:"lat"`
	Lng    float64 `rethink:"lng"`
	Text   string  `rethink:"text"`
}
