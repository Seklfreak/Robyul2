package google

import (
	"testing"

	"github.com/Seklfreak/Robyul2/helpers"
)

func TestLinkResultEmbed(t *testing.T) {
	helpers.LoadTranslations()

	searchResult := linkResult{
		Link:  "https://www.google.com/",
		Title: "Google",
		Text:  "Search the world's information, including webpages, images, videos and more. Google has many special features to help you find exactly what you're looking ...",
	}

	resultEmbed := linkResultEmbed(searchResult)

	if resultEmbed == nil {
		t.Fatal("google.linkResultEmbed() failed to create an embed")
	}

	if resultEmbed.URL != searchResult.Link ||
		resultEmbed.Title != searchResult.Title ||
		resultEmbed.Description != searchResult.Text {
		t.Fatal("google.linkResultEmbed() created an embed with invalid data")
	}
}

func TestImageResultEmbed(t *testing.T) {
	helpers.LoadTranslations()

	searchResult := imageResult{
		Title: "Image result for google",
		URL:   "https://encrypted-tbn0.gstatic.com/images?q=tbn:ANd9GcRgRj3AnHdw708pjQqsf_7hp-_yWvwBp9Y7Yw3yWTJ9eXeGvY4KSWzJQow",
		Link:  "https://www.google.com/url?q=https://twitter.com/google&sa=U&ved=0ahUKEwigtKPGxrzXAhWECOwKHYxgAbcQwW4IFjAA&usg=AOvVaw226LF1YGmMlOfgJAb5kJM1",
	}

	resultEmbed := imageResultEmbed(searchResult)

	if resultEmbed == nil {
		t.Fatal("google.imageResultEmbed() failed to create an embed")
	}

	if resultEmbed.URL != searchResult.Link ||
		resultEmbed.Title != searchResult.Title ||
		resultEmbed.Image.URL != searchResult.URL {
		t.Fatal("google.imageResultEmbed() created an embed with invalid data")
	}
}
