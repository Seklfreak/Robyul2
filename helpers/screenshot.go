package helpers

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"
)

var (
	sushiiImageServerBase string
)

func TakeHTMLScreenshot(html string, width, height int) (data []byte, err error) {
	if sushiiImageServerBase == "" {
		sushiiImageServerBase = GetConfig().Path("sushii-image-server.base").Data().(string)
	}

	marshalledRequest, err := json.Marshal(&SushiRequest{
		Html:   html,
		Width:  width,
		Height: height,
	})
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: time.Duration(30 * time.Second),
	}

	request, err := http.NewRequest("POST", sushiiImageServerBase+"/html", bytes.NewBuffer(marshalledRequest))
	if err != nil {
		return nil, err
	}

	request.Header.Set("User-Agent", DEFAULT_UA)
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	return ioutil.ReadAll(response.Body)
}

type SushiRequest struct {
	Html   string `json:"html"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
