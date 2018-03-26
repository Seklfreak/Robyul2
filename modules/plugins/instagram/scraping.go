package instagram

import (
	"net/http"
	"time"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/pkg/errors"
)

func (m *Handler) getPostInformation(shortCode string, proxy http.Transport) (information InstagramPostInformation, err error) {
	targetLink := "https://www.instagram.com/p/" + shortCode + "/"
	postSite, err := helpers.NetGetUAWithErrorAndTransport(targetLink, helpers.DEFAULT_UA, proxy)
	if err != nil {
		return
	}
	sharedData, err := m.getInstagramSharedData(string(postSite))
	helpers.Relax(err)

	for _, postData := range sharedData.EntryData.PostPage {
		if postData.Graphql.ShortcodeMedia.Shortcode == shortCode {
			information = InstagramPostInformation{
				ID:        postData.Graphql.ShortcodeMedia.ID + "_" + postData.Graphql.ShortcodeMedia.Owner.ID,
				Shortcode: postData.Graphql.ShortcodeMedia.Shortcode,
				Author: InstagramAuthorInformations{
					ID:            postData.Graphql.ShortcodeMedia.Owner.ID,
					ProfilePicUrl: postData.Graphql.ShortcodeMedia.Owner.ProfilePicURL,
					Username:      postData.Graphql.ShortcodeMedia.Owner.Username,
					FullName:      postData.Graphql.ShortcodeMedia.Owner.FullName,
					IsPrivate:     postData.Graphql.ShortcodeMedia.Owner.IsPrivate,
					IsVerified:    postData.Graphql.ShortcodeMedia.Owner.IsVerified,
				},
				MediaUrls: []string{m.getBestDisplayResource(postData.Graphql.ShortcodeMedia.DisplayResources)},
				Caption:   "",
				TakentAt:  time.Unix(int64(postData.Graphql.ShortcodeMedia.TakenAtTimestamp), 0),
				IsVideo:   postData.Graphql.ShortcodeMedia.IsVideo,
			}
			if postData.Graphql.ShortcodeMedia.VideoURL != "" {
				information.MediaUrls = []string{postData.Graphql.ShortcodeMedia.VideoURL}
			}
			if postData.Graphql.ShortcodeMedia.EdgeMediaToCaption.Edges != nil &&
				len(postData.Graphql.ShortcodeMedia.EdgeMediaToCaption.Edges) > 0 {
				information.Caption = postData.Graphql.ShortcodeMedia.EdgeMediaToCaption.Edges[0].Node.Text
			}
			if postData.Graphql.ShortcodeMedia.EdgeSidecarToChildren.Edges != nil &&
				len(postData.Graphql.ShortcodeMedia.EdgeSidecarToChildren.Edges) > 0 {
				information.MediaUrls = make([]string, 0)
				for _, sidecar := range postData.Graphql.ShortcodeMedia.EdgeSidecarToChildren.Edges {
					sidecarUrl := m.getBestDisplayResource(sidecar.Node.DisplayResources)
					if sidecar.Node.VideoURL != "" {
						sidecarUrl = sidecar.Node.VideoURL
					}
					information.MediaUrls = append(information.MediaUrls, sidecarUrl)
				}
			}
		}
	}

	if information.ID == "" {
		return information, errors.New("failed to find post information")
	}

	return
}
