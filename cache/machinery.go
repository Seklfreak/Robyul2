package cache

import (
	"errors"
	"sync"

	"github.com/RichardKnop/machinery/v1"
	"github.com/go-redis/redis"
)

var (
	machineryRedisClient *redis.Client
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

func SetMachineryRedisClient(s *redis.Client) {
	machineryServerMutex.Lock()
	defer machineryServerMutex.Unlock()

	machineryRedisClient = s
}

func GetMachineryRedisClient() *redis.Client {
	machineryServerMutex.RLock()
	defer machineryServerMutex.RUnlock()

	if machineryRedisClient == nil {
		panic(errors.New("Tried to get machinery redis client before cache#SetMachineryRedisClient() was called"))
	}

	return machineryRedisClient
}
