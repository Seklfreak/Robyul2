package service

import (
	"sync"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	redisCache "github.com/go-redis/cache"
	rethink "github.com/gorethink/gorethink"
)

type quota struct {
	data     models.YoutubeQuota
	count    int64
	interval int64

	sync.Mutex
}

const (
	dailyQuotaLimit   int64 = 1000000
	activityQuotaCost int64 = 5
	searchQuotaCost   int64 = 100
	videosQuotaCost   int64 = 7
	channelsQuotaCost int64 = 7
)

func (q *quota) Init() (err error) {
	q.Lock()
	defer q.Unlock()

	if q.count, err = q.readEntryCount(); err != nil {
		return
	}

	q.data.Daily = dailyQuotaLimit
	q.data.Left = dailyQuotaLimit
	q.data.ResetTime = q.calcResetTime().Unix()

	oldQuota, err := q.get()
	if err != nil {
		return err
	}

	if q.data.ResetTime <= oldQuota.ResetTime {
		q.data.Left = oldQuota.Left
	}

	return q.set()
}

func (q *quota) GetInterval() int64 {
	q.Lock()
	defer q.Unlock()

	return q.interval
}

func (q *quota) GetCount() int64 {
	q.Lock()
	defer q.Unlock()

	return q.count
}

func (q *quota) GetQuota() models.YoutubeQuota {
	q.Lock()
	defer q.Unlock()

	return q.data
}

func (q *quota) Sub(i int64) int64 {
	q.Lock()
	defer q.Unlock()

	if q.data.Left < i {
		return -1
	}

	q.data.Left -= i
	return q.data.Left
}

func (q *quota) IncEntryCount() {
	q.Lock()
	defer q.Unlock()

	q.count++
}

func (q *quota) DecEntryCount() {
	q.Lock()
	defer q.Unlock()

	if q.count > 0 {
		q.count--
	}
}

func (q *quota) UpdateCheckingInterval() error {
	q.Lock()
	defer q.Unlock()

	q.interval = q.calcCheckingTimeInterval()
	return q.set()
}

func (q *quota) DailyLimitExceeded() {
	q.Lock()
	defer q.Unlock()

	q.data.Left = 0
}

// Set entries count which will use in quota calculation.
func (q *quota) readEntryCount() (int64, error) {
	query := rethink.Table(models.YoutubeChannelTable).Count()

	cursor, err := query.Run(helpers.GetDB())
	if err != nil {
		return 0, err
	}
	defer cursor.Close()

	cnt := int64(0)
	cursor.One(&cnt)

	return cnt, nil
}

// readOldQuota reads previous quota information from database.
// If failed, return zero filled quota.
func (q *quota) get() (models.YoutubeQuota, error) {
	codec := cache.GetRedisCacheCodec()

	var savedQuota models.YoutubeQuota
	if err := codec.Get(models.YoutubeQuotaRedisKey, &savedQuota); err != nil {
		return models.YoutubeQuota{
			Daily:     0,
			Left:      0,
			ResetTime: 0,
		}, nil
	}

	return savedQuota, nil
}

func (q *quota) set() error {
	return cache.GetRedisCacheCodec().Set(&redisCache.Item{
		Key:        models.YoutubeQuotaRedisKey,
		Object:     q.data,
		Expiration: time.Hour * 24,
	})
}

func (q *quota) calcResetTime() time.Time {
	now := time.Now()
	localZone := now.Location()

	// Youtube quota is reset when every midnight in pacific time
	pacific, err := time.LoadLocation("America/Los_Angeles")
	if err == nil {
		now = now.In(pacific)
	} else {
		cache.GetLogger().Error(err)
	}

	y, m, d := now.Date()
	resetTime := time.Date(y, m, d+1, 0, 0, 0, 0, now.Location())

	return resetTime.In(localZone)
}

func (q *quota) calcCheckingTimeInterval() int64 {
	defaultTimeInterval := int64(5)

	now := time.Now().Unix()

	if now > q.data.ResetTime {
		q.data.ResetTime = q.calcResetTime().Unix()
		q.data.Left = dailyQuotaLimit
	}

	delta := q.data.ResetTime - now
	if delta < 1 {
		return defaultTimeInterval
	}

	quotaPerSec := q.data.Left / delta
	if quotaPerSec < 1 {
		// Adds 300 seconds for reducing "API limit exceed" error message.
		//
		// Just after reset time, youtube API server's quota isn't synchronized correctly for few minutes.
		// It occurs some responses are OK, others are ERR(API limit exceed).
		// Hence schedule to reset time + 300 seconds.
		return delta + 300
	}

	calcTimeInterval := (channelsQuotaCost * q.count / quotaPerSec)
	if calcTimeInterval < defaultTimeInterval {
		return defaultTimeInterval
	}

	return calcTimeInterval
}
