package google

import (
	"errors"
	"net/http"
	"net/url"

	"fmt"

	"strings"

	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/Seklfreak/Robyul2/helpers"
)

// based on https://github.com/devsnek/googlebot/blob/rewrite/src/util/google.js
const (
	UserAgent = "Mozilla/5.0 (Windows NT 6.3; Win64; x64) Gecko/20100101 Firefox/53."
	BaseUrl   = "https://www.google.com"
	SearchUrl = BaseUrl + "/search"
)

type linkResult struct {
	Link  string
	Title string
	Text  string
}

type imageResult struct {
	Title string
	URL   string
	Link  string
}

func getSearchQueries(queryText string, nsfw, friendly bool) (query string) {
	safeText := "on"
	if nsfw {
		safeText = "off"
	}

	parsedUrl, err := url.Parse("https://google.com")
	if err != nil {
		panic(err)
	}
	queryBuilder := parsedUrl.Query()
	queryBuilder.Add("q", queryText)
	queryBuilder.Add("safe", safeText)

	if !friendly {
		queryBuilder.Add("lr", "lang_en")
		queryBuilder.Add("hl", "en")
	}

	return queryBuilder.Encode()
}

func getImageSearchQuries(queryText string, nsfw, friendly bool) (query string) {
	safeText := "active"
	if nsfw {
		safeText = "disabled"
	}

	parsedUrl, err := url.Parse("https://google.com")
	if err != nil {
		panic(err)
	}
	queryBuilder := parsedUrl.Query()
	queryBuilder.Add("q", queryText)
	queryBuilder.Add("safe", safeText)
	queryBuilder.Add("ie", "ISO-8859-1")
	queryBuilder.Add("source", "hp")
	queryBuilder.Add("tbm", "isch")
	queryBuilder.Add("gbv", "1")
	queryBuilder.Add("gs_l", "img")

	if !friendly {
		queryBuilder.Add("hl", "en")
	}

	return queryBuilder.Encode()
}

func search(query string, nsfw bool, transport *http.Transport) (results []linkResult, err error) {
	client := &http.Client{
		Timeout: time.Duration(10 * time.Second),
	}

	if transport != nil {
		client.Transport = transport
	}

	request, err := http.NewRequest("GET", SearchUrl+"?"+getSearchQueries(query, nsfw, false), nil)
	if err != nil {
		return results, err
	}

	request.Header.Set("User-Agent", UserAgent)

	response, err := client.Do(request)
	if err != nil {
		if strings.Contains(err.Error(), "http.httpError") ||
			strings.Contains(err.Error(), "url.Error") ||
			strings.Contains(err.Error(), "Timeout") {
			// try with proxy
			proxy, err := helpers.GetRandomProxy()
			if err != nil {
				return results, err
			}
			return search(query, nsfw, &proxy)
		}
		return results, err
	}

	if response.StatusCode != 200 {
		if response.StatusCode == 503 { // too many requests
			// try with proxy
			proxy, err := helpers.GetRandomProxy()
			if err != nil {
				return results, err
			}
			return search(query, nsfw, &proxy)
		}
		return results, errors.New(fmt.Sprintf("unexpected status code: %d", response.StatusCode))
	}

	doc, err := goquery.NewDocumentFromResponse(response)
	if err != nil {
		return results, err
	}

	doc.Find("div[class=rc]").Each(func(_ int, selection *goquery.Selection) {
		link := selection.Find("h3[class=r]")
		text := selection.Find("span[class=st]")
		result := linkResult{
			Link:  link.Children().First().AttrOr("href", ""),
			Title: link.Children().First().Text(),
			Text:  text.Text(),
		}
		if result.Link != "" && result.Title != "" {
			results = append(results, result)
		}
	})

	if len(results) <= 0 {
		return results, errors.New("no search results")
	}

	return results, nil
}

func imageSearch(query string, nsfw bool, transport *http.Transport) (results []imageResult, err error) {
	client := &http.Client{
		Timeout: time.Duration(10 * time.Second),
	}

	if transport != nil {
		client.Transport = transport
	}

	request, err := http.NewRequest("GET", SearchUrl+"?"+getImageSearchQuries(query, nsfw, false), nil)
	if err != nil {
		return results, err
	}

	request.Header.Set("User-Agent", UserAgent)

	response, err := client.Do(request)
	if err != nil {
		if strings.Contains(err.Error(), "http.httpError") || strings.Contains(err.Error(), "url.Error") {
			// try with proxy
			proxy, err := helpers.GetRandomProxy()
			if err != nil {
				return results, err
			}
			return imageSearch(query, nsfw, &proxy)
		}
		return results, err
	}

	if response.StatusCode != 200 {
		if response.StatusCode == 503 { // too many requests
			// try with proxy
			proxy, err := helpers.GetRandomProxy()
			if err != nil {
				return results, err
			}
			return imageSearch(query, nsfw, &proxy)
		}
		return results, errors.New(fmt.Sprintf("unexpected status code: %d", response.StatusCode))
	}

	doc, err := goquery.NewDocumentFromResponse(response)
	if err != nil {
		return results, err
	}

	doc.Find("td a").Each(func(_ int, selection *goquery.Selection) {
		image := selection.Find("img")
		result := imageResult{
			URL:   image.AttrOr("src", ""),
			Link:  selection.AttrOr("href", ""),
			Title: image.AttrOr("alt", "Image result"),
		}
		if strings.HasPrefix(result.Link, "/url?") {
			result.Link = BaseUrl + result.Link
		}

		if result.URL != "" && result.Link != "" {
			results = append(results, result)
		}
	})

	if len(results) <= 0 {
		return results, errors.New("no search results")
	}

	return results, nil
}
