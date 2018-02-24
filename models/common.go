package models

type MongoDbCollection string

func (c MongoDbCollection) String() string {
	return string(c)
}
