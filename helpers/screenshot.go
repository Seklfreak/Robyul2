package helpers

import (
	"encoding/json"
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

	data, err = NetPostUAWithError(sushiiImageServerBase+"/html", string(marshalledRequest), DEFAULT_UA)
	if err != nil {
		return nil, err
	}

	return data, nil
}

type SushiRequest struct {
	Html   string `json:"html"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
