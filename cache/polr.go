package cache

import (
	"sync"

	"github.com/Seklfreak/polr-go"
)

var (
	polrClient      *polr.Polr
	polrClientMutex sync.RWMutex
)

func SetPolr(s *polr.Polr) {
	polrClientMutex.RLock()
	defer polrClientMutex.RUnlock()

	polrClient = s
}

func GetPolr() *polr.Polr {
	polrClientMutex.RLock()
	defer polrClientMutex.RUnlock()

	return polrClient
}
