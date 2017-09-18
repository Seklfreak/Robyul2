package helpers

import (
	"net/http"

	"fmt"

	"time"

	"io/ioutil"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/bwmarrin/discordgo"
	"github.com/getsentry/raven-go"
	redisCache "github.com/go-redis/cache"
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

func GuildFriendRequest(guildID string, method string, endpoint string) (result []byte, err error) {
	friend := cache.GetFriend(guildID)
	if friend == nil {
		return []byte{}, errors.New("No friend on this server!")
	}

	return FriendRequest(friend, method, endpoint)
}

func FriendRequest(friend *discordgo.Session, method string, endpoint string) (result []byte, err error) {
	cacheCodec := cache.GetRedisCacheCodec()
	key := fmt.Sprintf("robyul2-discord:helper-api:request:%s", GetMD5Hash(friend.State.User.ID+"-"+method+"-"+endpoint))

	if err = cacheCodec.Get(key, &result); err == nil {
		return result, err
	}

	client := &http.Client{}

	request, err := http.NewRequest(method, discordgo.EndpointAPI+endpoint, nil)
	if err != nil {
		panic(err)
	}

	request.Header.Set("Authorization", friend.Token)

	cache.GetLogger().WithField("module", "friend").WithField("method", "FriendRequest").Debug(
		fmt.Sprintf("friend api request: %s: %s %s", friend.State.User.Username, method, endpoint))

	response, err := client.Do(request)
	if err != nil {
		return []byte{}, err
	}
	if response.StatusCode != 200 {
		return []byte{}, errors.New(fmt.Sprintf("unexpected status code: %d", response.StatusCode))
	}
	result, err = ioutil.ReadAll(response.Body)
	if err != nil {
		return []byte{}, err
	}

	err = cacheCodec.Set(&redisCache.Item{
		Key:        key,
		Object:     result,
		Expiration: time.Minute * 60,
	})
	if err != nil {
		raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
	}
	return result, nil
}
