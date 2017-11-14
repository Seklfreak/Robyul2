package google

import (
	"strings"
	"testing"
)

func TestGetSearchQueries(t *testing.T) {
	queryResult := getSearchQueries("foo bar äöü", true, true)
	if queryResult != "q=foo+bar+%C3%A4%C3%B6%C3%BC&safe=off" {
		t.Fatal("google.getSearchQueries() created an invalid query string")
	}

	queryResult = getSearchQueries("foo bar äöü", true, false)
	if queryResult != "hl=en&lr=lang_en&q=foo+bar+%C3%A4%C3%B6%C3%BC&safe=off" {
		t.Fatal("google.getSearchQueries() created an invalid query string")
	}

	queryResult = getSearchQueries("foo bar äöü", false, true)
	if queryResult != "q=foo+bar+%C3%A4%C3%B6%C3%BC&safe=on" {
		t.Fatal("google.getSearchQueries() created an invalid query string")
	}

	queryResult = getSearchQueries("foo bar äöü", false, false)
	if queryResult != "hl=en&lr=lang_en&q=foo+bar+%C3%A4%C3%B6%C3%BC&safe=on" {
		t.Fatal("google.getSearchQueries() created an invalid query string")
	}
}

func TestImageSearchQueries(t *testing.T) {
	queryResult := getImageSearchQuries("foo bar äöü", true, true)
	if queryResult != "gbv=1&gs_l=img&ie=ISO-8859-1&q=foo+bar+%C3%A4%C3%B6%C3%BC&safe=disabled&source=hp&tbm=isch" {
		t.Fatal("google.getImageSearchQuries() created an invalid query string")
	}

	queryResult = getImageSearchQuries("foo bar äöü", true, false)
	if queryResult != "gbv=1&gs_l=img&hl=en&ie=ISO-8859-1&q=foo+bar+%C3%A4%C3%B6%C3%BC&safe=disabled&source=hp&tbm=isch" {
		t.Fatal("google.getImageSearchQuries() created an invalid query string")
	}

	queryResult = getImageSearchQuries("foo bar äöü", false, true)
	if queryResult != "gbv=1&gs_l=img&ie=ISO-8859-1&q=foo+bar+%C3%A4%C3%B6%C3%BC&safe=active&source=hp&tbm=isch" {
		t.Fatal("google.getImageSearchQuries() created an invalid query string")
	}

	queryResult = getImageSearchQuries("foo bar äöü", false, false)
	if queryResult != "gbv=1&gs_l=img&hl=en&ie=ISO-8859-1&q=foo+bar+%C3%A4%C3%B6%C3%BC&safe=active&source=hp&tbm=isch" {
		t.Fatal("google.getImageSearchQuries() created an invalid query string")
	}
}

func TestSearch(t *testing.T) {
	results, err := search("google", false)
	if err != nil {
		t.Fatalf("google.search() returned an error: %s", err.Error())
	}

	if len(results) <= 0 {
		t.Fatal("google.search() returned less than one result")
	}

	if results[0].Title != "Google" {
		t.Fatalf("google.search() first result's Title is not \"Google\" but \"%s\"", results[0].Title)
	}
	if results[0].Link != "https://www.google.com/" {
		t.Fatalf("google.search() first result's Link is not \"https://www.google.com/\" but \"%s\"", results[0].Link)
	}
	if !strings.Contains(results[0].Text, "Search") {
		t.Fatalf("google.search() first result's Text does not contain \"Search\", it is \"%s\"", results[0].Text)
	}

	sfwResult, err := search("porn", false)
	if err != nil {
		t.Fatalf("google.search() returned an error: %s", err.Error())
	}
	if len(sfwResult) <= 0 {
		t.Fatal("google.search() returned less than one result")
	}
	nsfwResult, err := search("porn", true)
	if err != nil {
		t.Fatalf("google.search() returned an error: %s", err.Error())
	}
	if len(nsfwResult) <= 0 {
		t.Fatal("google.search() returned less than one result")
	}

	if sfwResult[0].Title == nsfwResult[0].Title {
		t.Fatalf("google.search() returned the same Title for a nsfw and sfw search, sfw: %s, nsfw: %s", sfwResult[0].Title, nsfwResult[0].Title)
	}
	if sfwResult[0].Link == nsfwResult[0].Link {
		t.Fatalf("google.search() returned the same Link for a nsfw and sfw search, sfw: %s, nsfw: %s", sfwResult[0].Link, nsfwResult[0].Link)
	}
	if sfwResult[0].Text == nsfwResult[0].Text {
		t.Fatalf("google.search() returned the same Text for a nsfw and sfw search, sfw: %s, nsfw: %s", sfwResult[0].Text, nsfwResult[0].Text)
	}
}

func TestImageSearch(t *testing.T) {
	results, err := imageSearch("google", false)
	if err != nil {
		t.Fatalf("google.imageSearch() returned an error: %s", err.Error())
	}

	if len(results) <= 0 {
		t.Fatal("google.imageSearch() returned less than one result")
	}

	if results[0].Title != "Image result for google" {
		t.Fatalf("google.imageSearch() first result's Title is not \"Image result for google\" but \"%s\"", results[0].Title)
	}
	if !strings.Contains(results[0].Link, "https://www.google.com/url?q=http") {
		t.Fatalf("google.imageSearch() first result's Link does not contain \"https://www.google.com/url?q=http\", it is \"%s\"", results[0].Link)
	}
	if !strings.Contains(results[0].URL, "https://encrypted-tbn0.gstatic.com/images?q=tbn:") {
		t.Fatalf("google.imageSearch() first result's URL does not contain \"https://encrypted-tbn0.gstatic.com/images?q=tbn:\", it is \"%s\"", results[0].URL)
	}

	sfwResult, err := imageSearch("porn", false)
	if err != nil {
		t.Fatalf("google.imageSearch() returned an error: %s", err.Error())
	}
	if len(sfwResult) <= 0 {
		t.Fatal("google.imageSearch() returned less than one result")
	}
	nsfwResult, err := imageSearch("porn", true)
	if err != nil {
		t.Fatalf("google.imageSearch() returned an error: %s", err.Error())
	}
	if len(nsfwResult) <= 0 {
		t.Fatal("google.imageSearch() returned less than one result")
	}

	if sfwResult[0].Title != nsfwResult[0].Title {
		t.Fatalf("google.imageSearch() returned a different Title for a nsfw and sfw search, sfw: %s, nsfw: %s", sfwResult[0].Title, nsfwResult[0].Title)
	}
	if sfwResult[0].Link == nsfwResult[0].Link {
		t.Fatalf("google.imageSearch() returned the same Link for a nsfw and sfw search, sfw: %s, nsfw: %s", sfwResult[0].Link, nsfwResult[0].Link)
	}
}
