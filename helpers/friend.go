package helpers

import (
	"net/http"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
)

const (
	ServerLimitPerFriend = 95
)

func InviteFriend() (*discordgo.Session, error) {
	friends := cache.GetFriends()

	var chosenFriend *discordgo.Session
	for _, friend := range friends {
		if len(friend.State.Guilds) < ServerLimitPerFriend {
			chosenFriend = friend
			break
		}
	}

	if chosenFriend == nil {
		return nil, errors.New("No friend with free slots available, please add more friends!")
	}

	return chosenFriend, nil
}

func GuildFriendRequest(guildID string, method string, endpoint string) (response *http.Response, err error) {
	friend := cache.GetFriend(guildID)
	if friend == nil {
		return nil, errors.New("No friend on this server!")
	}

	return FriendRequest(friend, method, endpoint)
}

func FriendRequest(friend *discordgo.Session, method string, endpoint string) (response *http.Response, err error) {
	client := &http.Client{}

	request, err := http.NewRequest(method, discordgo.EndpointAPI+endpoint, nil)
	if err != nil {
		panic(err)
	}

	request.Header.Set("Authorization", friend.Token)

	response, err = client.Do(request)
	return response, err
}
