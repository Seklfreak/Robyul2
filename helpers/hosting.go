package helpers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"time"

	"net/url"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/pkg/errors"
)

func UploadImage(imageData []byte) (hostedUrl string, err error) {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	parameters := url.Values{"image": {base64.StdEncoding.EncodeToString(imageData)}}

	req, err := http.NewRequest("POST", "https://api.imgur.com/3/image", strings.NewReader(parameters.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Client-ID "+GetConfig().Path("imgur.client_id").Data().(string))
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}

	if res != nil && res.Body != nil {
		defer res.Body.Close()
	}

	var imgurResponse struct {
		Data struct {
			Link  string `json:"link"`
			Error string `json:"error"`
		} `json:"data"`
		Status  int  `json:"status"`
		Success bool `json:"success"`
	}

	json.NewDecoder(res.Body).Decode(&imgurResponse)

	if imgurResponse.Success == false {
		return "", errors.New(fmt.Sprintf("Imgur API Error: %d (%s)", imgurResponse.Status, fmt.Sprintf("%#v", imgurResponse.Data.Error)))
	}

	cache.GetLogger().WithField("module", "levels").Info("uploaded a picture to imgur: " + imgurResponse.Data.Link)
	return imgurResponse.Data.Link, nil
}
