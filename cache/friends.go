package cache

import (
	"sync"

	"github.com/bwmarrin/discordgo"
)

var (
	friends      []*discordgo.Session
	friendsMutex sync.RWMutex
)

func AddFriend(s *discordgo.Session) {
	friendsMutex.Lock()
	friends = append(friends, s)
	friendsMutex.Unlock()
}

func GetFriends() []*discordgo.Session {
	friendsMutex.RLock()
	defer friendsMutex.RUnlock()

	return friends
}

func GetFriend(guildID string) *discordgo.Session {
	friendsMutex.RLock()
	defer friendsMutex.RUnlock()

	for _, friend := range friends {
		for _, friendGuild := range friend.State.Guilds {
			if friendGuild.ID == guildID {
				return friend
			}
		}
	}

	return nil
}
