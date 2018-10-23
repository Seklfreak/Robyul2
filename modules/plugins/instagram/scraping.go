package instagram

import (
	"encoding/json"
	"net/http"
	"time"

	"fmt"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/pkg/errors"
)

func (m *Handler) getInformationAndPosts(username string, proxy http.Transport) (information InstagramAuthorInformations, posts []InstagramShortPostInformation, err error) {
	pageUrl := "https://www.instagram.com/" + username + "/"
	pageContent, err := helpers.NetGetUAWithErrorAndTransport(pageUrl, helpers.DEFAULT_UA, proxy)
	if err != nil {
		return
	}

	var sharedDataText string
	sharedDataText, err = m.extractInstagramSharedData(string(pageContent))
	if err != nil {
		return
	}

	var graphQlFeedResult InstagramPublicProfileFeed // TODO: new struct
	err = json.Unmarshal([]byte(sharedDataText), &graphQlFeedResult)
	if err != nil {
		return
	}

	if graphQlFeedResult.EntryData.ProfilePage == nil || len(graphQlFeedResult.EntryData.ProfilePage) < 0 ||
		graphQlFeedResult.EntryData.ProfilePage[0].Graphql.User.ID == "" {
		return information, posts, fmt.Errorf("failed to find user information for %s", username)
	}

	feed := graphQlFeedResult.EntryData.ProfilePage[0].Graphql

	receivedPosts := feed.User.EdgeOwnerToTimelineMedia.Edges

	for i := len(receivedPosts)/2 - 1; i >= 0; i-- {
		opp := len(receivedPosts) - 1 - i
		receivedPosts[i], receivedPosts[opp] = receivedPosts[opp], receivedPosts[i]
	}

	for _, receivedPost := range feed.User.EdgeOwnerToTimelineMedia.Edges {
		posts = append(posts, InstagramShortPostInformation{
			ID:        receivedPost.Node.ID + "_" + feed.User.ID,
			Shortcode: receivedPost.Node.Shortcode,
			CreatedAt: time.Unix(int64(receivedPost.Node.TakenAtTimestamp), 0),
		})
	}

	information = InstagramAuthorInformations{
		ID:            feed.User.ID,
		ProfilePicUrl: feed.User.ProfilePicURLHd,
		Username:      feed.User.Username,
		FullName:      feed.User.FullName,
		Link:          feed.User.ExternalURL,
		IsPrivate:     feed.User.IsPrivate,
		IsVerified:    feed.User.IsVerified,
		Followings:    feed.User.EdgeFollow.Count,
		Followers:     feed.User.EdgeFollowedBy.Count,
		Posts:         feed.User.EdgeOwnerToTimelineMedia.Count,
		Biography:     feed.User.Biography,
	}

	return
}

func (m *Handler) getPostInformation(shortCode string, proxy http.Transport) (information InstagramPostInformation, err error) {
	targetLink := "https://www.instagram.com/p/" + shortCode + "/"
	postSite, err := helpers.NetGetUAWithErrorAndTransport(targetLink, helpers.DEFAULT_UA, proxy)
	if err != nil {
		return
	}

	var sharedDataText string
	sharedDataText, err = m.extractInstagramSharedData(string(postSite))
	if err != nil {
		return
	}

	var sharedData InstagramSharedData
	err = json.Unmarshal([]byte(sharedDataText), &sharedData)
	if err != nil {
		return
	}

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
