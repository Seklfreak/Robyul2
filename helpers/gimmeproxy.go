package helpers

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/Seklfreak/Robyul2/cache"
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

func GimmeProxy() (proxy http.Transport, err error) {
	gimmeProxyUrl := "https://gimmeproxy.com/api/getProxy?supportsHttps=true&protocol=http&minSpeed=50"
	result, err := NetGetUAWithError(gimmeProxyUrl, DEFAULT_UA)
	if err != nil {
		return proxy, err
	}

	var receivedProxy gimmeProxyResult
	err = json.Unmarshal(result, &receivedProxy)
	if err != nil {
		return proxy, err
	}

	proxyUrl, err := url.Parse(receivedProxy.Curl)
	if err != nil {
		return proxy, err
	}

	transport := http.Transport{Proxy: http.ProxyURL(proxyUrl)}

	cache.GetLogger().WithField("module", "gimmeproxy").Info("got proxy: ", proxyUrl)
	return transport, nil
}
