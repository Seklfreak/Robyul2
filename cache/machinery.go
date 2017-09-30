package cache

import (
	"errors"
	"sync"

	"github.com/RichardKnop/machinery/v1"
)

var (
	machineryServer      *machinery.Server
	machineryServerMutex sync.RWMutex
)

func SetMachineryServer(s *machinery.Server) {
	machineryServerMutex.Lock()
	machineryServer = s
	machineryServerMutex.Unlock()
}

func GetMachineryServer() *machinery.Server {
	machineryServerMutex.RLock()
	defer machineryServerMutex.RUnlock()

	if machineryServer == nil {
		panic(errors.New("Tried to get machinery server before cache#SetMachineryServer() was called"))
	}

	return machineryServer
}
