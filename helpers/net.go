package helpers

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/Jeffail/gabs"
	"github.com/Seklfreak/Robyul2/version"
)

var DEFAULT_UA = "Robyul2/" + version.BOT_VERSION + " (https://github.com/Seklfreak/Robyul2)"

// NetGet executes a GET request to url with the Karen/Discord-Bot user-agent
func NetGet(url string) []byte {
	return NetGetUA(url, DEFAULT_UA)
}

// NetGetUA performs a GET request with a custom user-agent
func NetGetUA(url string, useragent string) []byte {
	// Allocate client
	client := &http.Client{}

	// Prepare request
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}

	// Set custom UA
	request.Header.Set("User-Agent", useragent)

	// Do request
	response, err := client.Do(request)
	Relax(err)

	// Only continue if code was 200
	if response.StatusCode != 200 {
		panic(errors.New("Expected status 200; Got " + strconv.Itoa(response.StatusCode)))
	} else {
		// Read body
		defer response.Body.Close()

		buf := bytes.NewBuffer(nil)
		_, err := io.Copy(buf, response.Body)
		Relax(err)

		return buf.Bytes()
	}
}

func NetGetUAWithError(url string, useragent string) ([]byte, error) {
	// Allocate client
	client := &http.Client{}

	// Prepare request
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []byte{}, err
	}

	// Set custom UA
	request.Header.Set("User-Agent", useragent)

	// Do request
	response, err := client.Do(request)
	if err != nil {
		return []byte{}, err
	}

	// Only continue if code was 200
	if response.StatusCode != 200 {
		if err != nil {
			return []byte{}, errors.New("Expected status 200; Got " + strconv.Itoa(response.StatusCode))
		}
	} else {
		// Read body
		defer response.Body.Close()

		buf := bytes.NewBuffer(nil)
		_, err := io.Copy(buf, response.Body)
		if err != nil {
			return []byte{}, err
		}

		return buf.Bytes(), nil
	}
	return []byte{}, errors.New("Internal Error")
}

func NetPost(url string, data string) []byte {
	return NetPostUA(url, data, DEFAULT_UA)
}

func NetPostUA(url string, data string, useragent string) []byte {
	// Allocate client
	client := &http.Client{}

	// Prepare request
	request, err := http.NewRequest("POST", url, bytes.NewBufferString(data))
	if err != nil {
		panic(err)
	}

	request.Header.Set("User-Agent", useragent)
	request.Header.Set("Content-Type", "application/json")

	// Do request
	response, err := client.Do(request)
	Relax(err)

	// Only continue if code was 200
	if response.StatusCode != 200 {
		panic(errors.New("Expected status 200; Got " + strconv.Itoa(response.StatusCode)))
	} else {
		// Read body
		defer response.Body.Close()

		buf := bytes.NewBuffer(nil)
		_, err := io.Copy(buf, response.Body)
		Relax(err)

		return buf.Bytes()
	}
}

// GetJSON sends a GET request to $url, parses it and returns the JSON
func GetJSON(url string) *gabs.Container {
	// Parse json
	json, err := gabs.ParseJSON(NetGet(url))
	Relax(err)

	return json
}
