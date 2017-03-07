package ratelimits

import (
    "errors"
    "sync"
    "time"
)

const (
    // How many keys a bucket may contain when created
    BUCKET_INITIAL_FILL = 16

    // The maximum amount of keys a user may possess
    BUCKET_UPPER_BOUND = 32

    // How often new keys drip into the buckets
    DROP_INTERVAL = 10 * time.Second

    // How many keys may drop at a time
    DROP_SIZE = 1
)

// Global pointer to a container instance
var Container = &BucketContainer{}

// Container struct to lock the bucket map
type BucketContainer struct {
    sync.RWMutex

    // Maps discord ids to key-counts
    buckets map[string]int8
}

// Allocates the map and starts routines
func (b *BucketContainer) Init() {
    b.Lock()
    b.buckets = make(map[string]int8)
    b.Unlock()

    go b.Refiller()
}

// Refills user buckets in a set interval
func (b *BucketContainer) Refiller() {
    for {
        b.Lock()
        for user, keys := range b.buckets {
            // Chill zone
            if keys == -1 {
                b.buckets[user]++
                continue
            }

            // Chill zone exit
            if keys == 0 {
                b.buckets[user] = BUCKET_INITIAL_FILL
                continue
            }

            // More free keys for nice users :3
            if keys < BUCKET_UPPER_BOUND {
                b.buckets[user] += DROP_SIZE
                continue
            }
        }
        b.Unlock()

        time.Sleep(DROP_INTERVAL)
    }
}

// Check if the user has a bucket. If not create one
func (b *BucketContainer) CreateBucketIfNotExists(user string) {
    b.RLock()
    _, e := b.buckets[user]
    b.RUnlock()

    if !e {
        b.Lock()
        b.buckets[user] = BUCKET_INITIAL_FILL
        b.Unlock()
    }
}

// Drains $amount from $user if he has enough keys left
func (b *BucketContainer) Drain(amount int8, user string) error {
    b.CreateBucketIfNotExists(user)

    // Check if there are enough keys left
    b.RLock()
    userAmount := b.buckets[user]
    b.RUnlock()

    if amount > userAmount {
        return errors.New("No keys left")
    }

    // Remove keys from bucket
    b.Lock()
    b.buckets[user] -= amount
    b.Unlock()

    return nil
}

// Check if the user still has keys
func (b *BucketContainer) HasKeys(user string) bool {
    b.CreateBucketIfNotExists(user)

    b.RLock()
    defer b.RUnlock()

    return b.buckets[user] > 0
}

func (b *BucketContainer) Get(user string) int8 {
    b.RLock()
    defer b.RUnlock()

    return b.buckets[user]
}

func (b *BucketContainer) Set(user string, value int8) {
    b.Lock()
    b.buckets[user] = value
    b.Unlock()
}
