package cache

import (
	"errors"
	"sync"

	"github.com/go-redis/cache"
	"github.com/go-redis/redis"
	"github.com/vmihailenco/msgpack"
)

var (
	redisClient *redis.Client
	redisMutext sync.RWMutex
	CacheCodec  *cache.Codec
)

func SetRedisClient(s *redis.Client) {
	redisMutext.Lock()
	redisClient = s

	CacheCodec = &cache.Codec{
		Redis: redisClient,
		Marshal: func(v interface{}) ([]byte, error) {
			return msgpack.Marshal(v)
		},
		Unmarshal: func(b []byte, v interface{}) error {
			return msgpack.Unmarshal(b, v)
		},
	}

	redisMutext.Unlock()
}

func GetRedisClient() *redis.Client {
	redisMutext.RLock()
	defer redisMutext.RUnlock()

	if redisClient == nil {
		panic(errors.New("Tried to get redis client before redis#setRedis() was called"))
	}

	return redisClient
}

func GetRedisCacheCodec() *cache.Codec {
	redisMutext.RLock()
	defer redisMutext.RUnlock()

	if CacheCodec.Redis == nil {
		panic(errors.New("Tried to get redis cache codec before redis#setRedis() was called"))
	}

	return CacheCodec
}
