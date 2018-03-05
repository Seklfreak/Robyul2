package youtube

import (
	"errors"
	"sync"
)

var (
	youtubeServiceClient      *Service
	youtubeServiceClientMutex sync.RWMutex
)

func SetYouTubeService(s *Service) {
	youtubeServiceClientMutex.Lock()
	youtubeServiceClient = s
	youtubeServiceClientMutex.Unlock()
}

func HasYouTubeService() bool {
	if youtubeServiceClient == nil {
		return false
	}
	return true
}

func GetYouTubeService() *Service {
	youtubeServiceClientMutex.RLock()
	defer youtubeServiceClientMutex.RUnlock()

	if youtubeServiceClient == nil {
		panic(errors.New("tried to get youtube service before cache#SetYouTubeService() was called"))
	}

	return youtubeServiceClient
}
