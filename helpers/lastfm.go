package helpers

import (
	"sync"

	"strings"

	"github.com/Seklfreak/Robyul2/models"
	"github.com/Seklfreak/lastfm-go/lastfm"
	"github.com/globalsign/mgo/bson"
)

var (
	lastFmClient      *lastfm.Api
	lastFmClientMutex sync.Mutex
)

// Gets the LastFM client from cache. If there is no LastFM client in cache yet it will create a new instance
func GetLastFmClient() (client *lastfm.Api) {
	lastFmClientMutex.Lock()
	defer lastFmClientMutex.Unlock()

	if lastFmClient != nil {
		return lastFmClient
	}

	lastFmClient = lastfm.New(
		GetConfig().Path("lastfm.api_key").Data().(string),
		GetConfig().Path("lastfm.api_secret").Data().(string),
	)

	return lastFmClient
}

// Gets the LastFM Username for a Discord User, returns an empty string if no username has been set
// userID	: the ID of the discord user
func GetLastFmUsername(userID string) (username string) {
	var entryBucket models.LastFmEntry
	var err error
	err = MdbOne(
		MdbCollection(models.LastFmTable).Find(bson.M{"userid": userID}),
		&entryBucket,
	)

	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			RelaxLog(err)
		}
		return ""
	}

	return entryBucket.LastFmUsername
}
