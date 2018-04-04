package instagram

import (
	"net/url"
	"strings"

	goinstaResponse "github.com/ahmdrz/goinsta/response"
)

func stripInstagramDirectLink(link string) (result string) {
	url, err := url.Parse(link)
	if err != nil {
		return link
	}
	queries := strings.Split(url.RawQuery, "&")
	var newQueryString string
	for _, query := range queries {
		if strings.HasPrefix(query, "ig_cache_key") { // strip ig_cache_key
			continue
		}
		newQueryString += query + "&"
	}
	newQueryString = strings.TrimSuffix(newQueryString, "&")
	url.RawQuery = newQueryString
	return url.String()
}

func getBestCandidateURL(imageCandidates []goinstaResponse.ImageCandidate) string {
	var lastBestCandidate goinstaResponse.ImageCandidate
	for _, candidate := range imageCandidates {
		if lastBestCandidate.URL == "" {
			lastBestCandidate = candidate
		} else {
			if candidate.Height > lastBestCandidate.Height || candidate.Width > lastBestCandidate.Width {
				lastBestCandidate = candidate
			}
		}
	}

	return lastBestCandidate.URL
}

/*
func getBestStoryVideoVersionURL(story goinstaResponse.StoryResponse, number int) string {
	item := story.Reel.Items[number]

	var lastBestCandidateURL string
	var lastBestCandidateWidth, lastBestCandidataHeight int
	for _, version := range item.VideoVersions {
		if lastBestCandidateURL == "" {
			lastBestCandidateURL = version.URL
			lastBestCandidataHeight = version.Height
			lastBestCandidateWidth = version.Width
		} else {
			if version.Height > lastBestCandidataHeight || version.Width > lastBestCandidateWidth {
				lastBestCandidateURL = version.URL
				lastBestCandidataHeight = version.Height
				lastBestCandidateWidth = version.Width
			}
		}
	}

	return lastBestCandidateURL
}
*/

func (m *Handler) getBestDisplayResource(imageCandidates []InstagramDisplayResource) string {
	var lastBestCandidate InstagramDisplayResource
	if imageCandidates != nil && len(imageCandidates) > 0 {
		for _, candidate := range imageCandidates {
			if lastBestCandidate.Src == "" {
				lastBestCandidate = candidate
			} else {
				if candidate.ConfigHeight > lastBestCandidate.ConfigHeight || candidate.ConfigWidth > lastBestCandidate.ConfigWidth {
					lastBestCandidate = candidate
				}
			}
		}
	}

	return lastBestCandidate.Src
}
