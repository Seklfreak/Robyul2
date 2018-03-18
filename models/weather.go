package models

import "github.com/globalsign/mgo/bson"

const (
	WeatherLastLocationsTable MongoDbCollection = "weather_last_locations"
)

type WeatherLastLocationEntry struct {
	ID     bson.ObjectId `bson:"_id,omitempty"`
	UserID string
	Lat    float64
	Lng    float64
	Text   string
}
