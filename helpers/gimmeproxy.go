package helpers

import (
	"encoding/json"
	"net/http"
	"net/url"

	"time"

	"strings"

	"github.com/Seklfreak/Robyul2/cache"
)

const (
	PROXIES_KEY       = "robyul-discord:gimmeproxy:proxies"
	NUMBER_OF_PROXIES = 150
)

var (
	PROXY_CHECK_URLS = []string{"https://instagram.com"}
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
			if !strings.Contains(err.Error(), "expected status 200; got 429") {
				RelaxLog(err)
			}
		} else {
			_, err = redis.SAdd(PROXIES_KEY, proxyUrlString).Result()
			RelaxLog(err)
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

	//cache.GetLogger().WithField("module", "gimmeproxy").Info("got proxy from cache: ", randomProxyUrl)

	transport := http.Transport{Proxy: http.ProxyURL(randomProxyUrl)}
	return transport, nil
}

func CachedProxiesHealthcheckLoop() {
	defer Recover()
	defer func() {
		go func() {
			cache.GetLogger().WithField("module", "gimmeproxy").Error(
				"The CachedProxiesHealthcheckLoop died. Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			CachedProxiesHealthcheckLoop()
		}()
	}()

	for {
		CachedProxiesHealthcheck()

		time.Sleep(1 * time.Hour)
	}
}

func CachedProxiesHealthcheck() {
	defer Recover()

	redis := cache.GetRedisClient()
	proxieUrlStrings, err := redis.SMembers(PROXIES_KEY).Result()
	RelaxLog(err)

	proxiesToDelete := make([]string, 0)

	for _, proxyUrlString := range proxieUrlStrings {
		randomProxyUrl, err := url.Parse(proxyUrlString)
		if err != nil {
			cache.GetLogger().WithField("module", "gimmeproxy").Infof(
				"removing proxy %s because error: %s", proxyUrlString, err.Error(),
			)
			proxiesToDelete = append(proxiesToDelete, proxyUrlString)
			continue
		}

		for _, proxyCheckUrl := range PROXY_CHECK_URLS {
			_, err := NetGetUAWithErrorAndTransport(proxyCheckUrl, DEFAULT_UA, http.Transport{Proxy: http.ProxyURL(randomProxyUrl)})
			if err != nil {
				cache.GetLogger().WithField("module", "gimmeproxy").Infof(
					"removing proxy %s because error: %s checking %s",
					proxyUrlString, err.Error(), proxyCheckUrl,
				)
				proxiesToDelete = append(proxiesToDelete, proxyUrlString)
				continue
			}
		}
	}

	cache.GetLogger().WithField("module", "gimmeproxy").Infof(
		"deleting %d proxies from cache", len(proxiesToDelete),
	)

	for _, proxyToDelete := range proxiesToDelete {
		_, err = redis.SRem(PROXIES_KEY, proxyToDelete).Result()
		RelaxLog(err)
	}

	FillProxies()
}

func FillProxies() {
	redis := cache.GetRedisClient()

	for {
		length, err := redis.SCard(PROXIES_KEY).Result()
		Relax(err)

		if length < NUMBER_OF_PROXIES {
			proxyUrlString, err := GimmeProxy()
			if err != nil {
				cache.GetLogger().WithField("module", "gimmeproxy").Warnf(
					"found %d cached proxies, which is less than %d, adding one failed: %s",
					length, NUMBER_OF_PROXIES, err.Error(),
				)
				if strings.Contains(err.Error(), "expected status 200; got 429") {
					return
				} else {
					time.Sleep(10 * time.Second)
					continue
				}
			} else {
				cache.GetLogger().WithField("module", "gimmeproxy").Infof(
					"found %d cached proxies, which is less than %d, adding one", length, NUMBER_OF_PROXIES,
				)
				_, err = redis.SAdd(PROXIES_KEY, proxyUrlString).Result()
				Relax(err)
			}
		}
	}
}
