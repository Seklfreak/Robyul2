package helpers

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/Seklfreak/Robyul2/cache"
)

const (
	PROXIES_KEY       = "robyul-discord:gimmeproxy:proxies"
	NUMBER_OF_PROXIES = 10
)

type gimmeProxyResult struct {
	SupportsHTTPS  bool   `json:"supportsHttps"`
	Protocol       string `json:"protocol"`
	IP             string `json:"ip"`
	Port           string `json:"port"`
	Get            bool   `json:"get"`
	Post           bool   `json:"post"`
	Cookies        bool   `json:"cookies"`
	Referer        bool   `json:"referer"`
	UserAgent      bool   `json:"user-agent"`
	AnonymityLevel int    `json:"anonymityLevel"`
	Websites       struct {
		Example bool `json:"example"`
		Google  bool `json:"google"`
		Amazon  bool `json:"amazon"`
	} `json:"websites"`
	Country        string  `json:"country"`
	TsChecked      int     `json:"tsChecked"`
	Curl           string  `json:"curl"`
	IPPort         string  `json:"ipPort"`
	Type           string  `json:"type"`
	Speed          float64 `json:"speed"`
	OtherProtocols struct {
	} `json:"otherProtocols"`
}

func GimmeProxy() (proxyUrl string, err error) {
	gimmeProxyUrl := "https://gimmeproxy.com/api/getProxy?supportsHttps=true&protocol=http&minSpeed=50"
	result, err := NetGetUAWithError(gimmeProxyUrl, DEFAULT_UA)
	if err != nil {
		return proxyUrl, err
	}

	var receivedProxy gimmeProxyResult
	err = json.Unmarshal(result, &receivedProxy)
	if err != nil {
		return proxyUrl, err
	}

	cache.GetLogger().WithField("module", "gimmeproxy").Info("received new proxy: ", receivedProxy.Curl)
	return receivedProxy.Curl, nil
}

func GetRandomProxy() (proxy http.Transport, err error) {
	redis := cache.GetRedisClient()
	length, err := redis.SCard(PROXIES_KEY).Result()
	if err != nil {
		return proxy, err
	}

	if length < NUMBER_OF_PROXIES {
		cache.GetLogger().WithField("module", "gimmeproxy").Infof(
			"found %d cached proxies, which is less than %d, adding one", length, NUMBER_OF_PROXIES,
		)
		proxyUrlString, err := GimmeProxy()
		if err != nil {
			return proxy, err
		}
		_, err = redis.SAdd(PROXIES_KEY, proxyUrlString).Result()
		if err != nil {
			return proxy, err
		}
	}

	randomProxyUrlString, err := redis.SRandMember(PROXIES_KEY).Result()
	if err != nil {
		return proxy, err
	}

	randomProxyUrl, err := url.Parse(randomProxyUrlString)
	if err != nil {
		return proxy, err
	}

	cache.GetLogger().WithField("module", "gimmeproxy").Info("got proxy from cache: ", randomProxyUrl)

	transport := http.Transport{Proxy: http.ProxyURL(randomProxyUrl)}
	return transport, nil
}

// TODO: test proxies, remove dead ones
