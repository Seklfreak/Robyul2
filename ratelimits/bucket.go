package ratelimits

import (
    "time"
    "sync"
    "errors"
)

const (
    // How many keys a bucket may contain when created
    BUCKET_INITIAL_FILL = 5

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
    sync.Mutex

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
        for user, keys := range b.buckets {
            // Chill zone
            if keys == -1 {
                b.Lock()
                b.buckets[user]++
                b.Unlock()

                continue
            }

            // Chill zone exit
            if keys == 0 {
                b.Lock()
                b.buckets[user] = BUCKET_INITIAL_FILL
                b.Unlock()

                continue
            }

            // More free keys for nice users :3
            if keys < BUCKET_UPPER_BOUND {
                b.Lock()
                b.buckets[user] += DROP_SIZE
                b.Unlock()

                continue
            }
        }

        time.Sleep(DROP_INTERVAL)
    }
}

// Check if the user has a bucket. If not create one
func (b *BucketContainer) CreateBucketIfNotExists(user string) {
    if _, e := b.buckets[user]; !e {
        b.Lock()
        b.buckets[user] = BUCKET_INITIAL_FILL
        b.Unlock()
    }
}

// Drains $amount from $user if he has enough keys left
func (b *BucketContainer) Drain(amount int8, user string) error {
    b.CreateBucketIfNotExists(user)

    // Check if there are enough keys left
    if amount > b.buckets[user] {
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

    return b.buckets[user] > 0
}

func (b *BucketContainer) Get(user string) int8 {
    return b.buckets[user]
}

func (b *BucketContainer) Set(user string, value int8) {
    b.Lock()
    b.buckets[user] = value
    b.Unlock()
}
