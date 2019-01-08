package cache

import (
	"errors"
	"sync"

	keen "github.com/inconshreveable/go-keen"
)

var (
	keenClient      *keen.Client
	keenClientMutex sync.RWMutex
)

func SetKeen(k *keen.Client) {
	keenClientMutex.Lock()
	keenClient = k
	keenClientMutex.Unlock()
}

func GetKeen() *keen.Client {
	keenClientMutex.RLock()
	defer keenClientMutex.RUnlock()

	if keenClient == nil {
		panic(errors.New("Tried to get keen client before cache#SetKeen() was called"))
	}

	return keenClient
}

func HasKeen() bool {
	if keenClient == nil {
		return false
	}
	return true
}
