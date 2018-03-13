package helpers

import (
	"sync"

	"github.com/shkh/lastfm-go/lastfm"
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
