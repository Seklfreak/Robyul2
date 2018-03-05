package youtube

import (
	"sync"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	redisCache "github.com/go-redis/cache"
	rethink "github.com/gorethink/gorethink"
)

// quota contains information for calculating
// checking time of feeds as fast as possible.
type quota struct {
	// Contains daily limit, left quota, reset time.
	data models.YoutubeQuota

	entriesCount  int64 // Number of feed entries count.
	checkInterval int64 // Calculated checking time interval.

	sync.Mutex // Embedded lock for changing quota fields.
}

const (
	dailyQuotaLimit   int64 = 1000000
	activityQuotaCost int64 = 5
	searchQuotaCost   int64 = 100
	videosQuotaCost   int64 = 7
	channelsQuotaCost int64 = 7
)

// Init fills quota information with default setting value and
// previous saved information from redis.
func (q *quota) Init() (err error) {
	q.Lock()
	defer q.Unlock()

	// Read the count of youtube feed entries.
	if q.entriesCount, err = q.readEntryCount(); err != nil {
		return
	}

	// Set default information.
	q.data.Daily = dailyQuotaLimit
	q.data.Left = dailyQuotaLimit
	q.data.ResetTime = q.calcResetTime().Unix()

	// Get saved quota information from redis.
	s, err := q.get()
	if err != nil {
		return err
	}

	// If reset time was over, then refresh the quota left.
	if q.data.ResetTime <= s.ResetTime {
		q.data.Left = s.Left
	}

	// Set new quota into redis.
	return q.set()
}

func (q *quota) GetInterval() int64 {
	q.Lock()
	defer q.Unlock()

	return q.checkInterval
}

func (q *quota) GetCount() int64 {
	q.Lock()
	defer q.Unlock()

	return q.entriesCount
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

	q.entriesCount++
}

func (q *quota) DecEntryCount() {
	q.Lock()
	defer q.Unlock()

	if q.entriesCount > 0 {
		q.entriesCount--
	}
}

func (q *quota) UpdateCheckingInterval() error {
	q.Lock()
	defer q.Unlock()

	q.checkInterval = q.calcCheckingTimeInterval()
	return q.set()
}

func (q *quota) DailyLimitExceeded() {
	q.Lock()
	defer q.Unlock()

	q.data.Left = 0
}

// readEntryCount reads how many entries in database
// and this will use in check time calculation.
func (q *quota) readEntryCount() (int64, error) {
	query := rethink.Table(models.YoutubeChannelTable).Count()

	c, err := query.Run(helpers.GetDB())
	if err != nil {
		return 0, err
	}
	defer c.Close()

	cnt := int64(0)
	c.One(&cnt)

	return cnt, nil
}

// get previous quota information from redis.
// If failed, return zero filled quota.
func (q *quota) get() (models.YoutubeQuota, error) {
	c := cache.GetRedisCacheCodec()

	var s models.YoutubeQuota
	if err := c.Get(models.YoutubeQuotaRedisKey, &s); err != nil {
		return models.YoutubeQuota{
			Daily:     0,
			Left:      0,
			ResetTime: 0,
		}, nil
	}

	return s, nil
}

func (q *quota) set() error {
	return cache.GetRedisCacheCodec().Set(&redisCache.Item{
		Key:        models.YoutubeQuotaRedisKey,
		Object:     q.data,
		Expiration: time.Hour * 24,
	})
}

// calcResetTime calculates the upcoming youtube quota reset time.
func (q *quota) calcResetTime() time.Time {
	now := time.Now()
	localZone := now.Location()

	// Youtube quota is reset when every midnight in pacific time.
	pacific, err := time.LoadLocation("America/Los_Angeles")
	if err == nil {
		now = now.In(pacific)
	} else {
		cache.GetLogger().Error(err)
	}

	y, m, d := now.Date()
	resetTime := time.Date(y, m, d+1, 0, 0, 0, 0, now.Location())

	// Returns in local zone format for convenient comparing.
	return resetTime.In(localZone)
}

// calcCheckingTimeInterval calculates checking time interval with
// current quota information.
func (q *quota) calcCheckingTimeInterval() int64 {
	defaultTimeInterval := int64(5)

	now := time.Now().Unix()

	// If now is later than reset time, then update reset time
	// and left quota information.
	if now > q.data.ResetTime {
		q.data.ResetTime = q.calcResetTime().Unix()
		q.data.Left = dailyQuotaLimit
	}

	// How much times do we have until quota will be reset.
	delta := q.data.ResetTime - now
	if delta < 1 {
		return defaultTimeInterval
	}

	// quotaPerSec means how many quota we can use in every seconds.
	quotaPerSec := q.data.Left / delta
	if quotaPerSec < 1 {
		// Adds 300 seconds for reducing "API limit exceed" error message.
		//
		// Just after reset time, youtube API server's quota isn't synchronized correctly for few minutes.
		// It occurs some responses are OK, others are ERR(API limit exceed).
		// Hence schedule to reset time + 300 seconds.
		return delta + 300
	}

	calcTimeInterval := (channelsQuotaCost * q.entriesCount / quotaPerSec)
	if calcTimeInterval < defaultTimeInterval {
		return defaultTimeInterval
	}

	return calcTimeInterval
}
