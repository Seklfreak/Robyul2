package cache

import (
	"errors"
	"sync"

	"github.com/Seklfreak/Robyul2/shardmanager"
)

var (
	session      *shardmanager.Manager
	sessionMutex sync.RWMutex
)

func SetSession(s *shardmanager.Manager) {
	sessionMutex.Lock()
	session = s
	sessionMutex.Unlock()
}

func GetSession() *shardmanager.Manager {
	sessionMutex.RLock()
	defer sessionMutex.RUnlock()

	if session == nil {
		panic(errors.New("Tried to get discord session before cache#SetSession() was called"))
	}

	return session
}
