package instagram

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
