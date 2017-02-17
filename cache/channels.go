package cache

import (
    "github.com/bwmarrin/discordgo"
    "time"
    "sync"
)

// How long a cached channel pointer is valid (seconds)
var channelTimeout int64 = 15

// A mutex to prevent concurrent modifications
var mutex = sync.Mutex{}

// Maps channel-id's to channel pointers
var channels = make(map[string]*discordgo.Channel)

// Maps channel-id's to unix timestamps
var channelMeta = make(map[string]int64)

// Requests a channel update and stores the pointer
func updateChannel(id string) error {
    channel, err := GetSession().Channel(id)
    if err != nil {
        return err
    }

    mutex.Lock()
    channels[id] = channel
    channelMeta[id] = time.Now().Unix()
    mutex.Unlock()

    return nil
}

// GetChannel tries to return a cached channel pointer
// If there is no cache a request is sent
func Channel(id string) (ch *discordgo.Channel, e error) {
    // Check if that channel wasn't cached yet
    if channels[id] == nil {
        e = updateChannel(id)
    }

    // Check if the channel timed out
    if time.Now().Unix() - channelMeta[id] > channelTimeout {
        e = updateChannel(id)
    }

    // Return channel
    ch = channels[id]
    return
}
