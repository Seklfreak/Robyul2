package cache

import (
	"errors"
	"sync"

	"gopkg.in/olivere/elastic.v5"
)

var (
	elasticClient      *elastic.Client
	elasticClientMutex sync.RWMutex
)

func SetElastic(s *elastic.Client) {
	elasticClientMutex.Lock()
	elasticClient = s
	elasticClientMutex.Unlock()
}

func HasElastic() bool {
	if elasticClient == nil {
		return false
	}
	return true
}

func GetElastic() *elastic.Client {
	elasticClientMutex.RLock()
	defer elasticClientMutex.RUnlock()

	if elasticClient == nil {
		panic(errors.New("Tried to get elastic client before cache#SetElastic() was called"))
	}

	return elasticClient
}
