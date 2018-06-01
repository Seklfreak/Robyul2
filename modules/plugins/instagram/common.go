package instagram

import (
	"net/url"
	"strings"
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
