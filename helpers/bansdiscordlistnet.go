package helpers

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/davecgh/go-spew/spew"
	redisCache "github.com/go-redis/cache"
	"github.com/pkg/errors"
)

func IsBannedOnBansdiscordlistNet(userID string) (isBanned bool, err error) {
	if userID == "" {
		return false, errors.New("invalid userID")
	}

	cacheCodec := cache.GetRedisCacheCodec()
	key := "robyul2-discord:bansdiscordlistnet:user:" + userID

	if err = cacheCodec.Get(key, &isBanned); err == nil {
		return isBanned, nil
	}

	token := GetConfig().Path("bansdiscordlistnet-token").Data().(string)

	if token == "" {
		return false, errors.New("no bans.discordlist.net token set")
	}
	apiUrl := "https://bans.discordlist.net/api"

	data := url.Values{}
	data.Set("token", token)
	data.Add("userid", userID)

	client := &http.Client{
		Timeout: time.Duration(10 * time.Second),
	}
	r, err := http.NewRequest("POST", apiUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return false, err
	}
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

	cache.GetLogger().WithField("module", "bansdiscordlistnet").Info("bans.discordlist.net API request: UserID: " + userID)

	resp, err := client.Do(r)
	if err != nil {
		if strings.Contains(err.Error(), "net/http: request canceled") {
			cache.GetLogger().WithField("module", "bansdiscordlistnet").Errorf(
				"failed to get status for #%s: %s",
				userID, spew.Sdump(err))
			return false, nil
		}
		return false, err
	}

	if resp.Body != nil {
		defer resp.Body.Close()
	}

	bytesResp, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	if string(bytesResp) == "True" {
		isBanned = true
	}

	err = cacheCodec.Set(&redisCache.Item{
		Key:        key,
		Object:     isBanned,
		Expiration: time.Minute * 30,
	})
	RelaxLog(err)

	return isBanned, nil
}
