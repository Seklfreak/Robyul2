package instagram

import (
	"encoding/json"
	"net/http"
	"time"

	"fmt"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
)

func (m *Handler) getPosts(accountID string, proxy http.Transport) (posts []InstagramShortPostInformation, err error) {
	graphQlUrl := m.graphQlMediaUrl(accountID)
	result, err := helpers.NetGetUAWithErrorAndTransport(graphQlUrl, helpers.DEFAULT_UA, proxy)
	if err != nil {
		return
	}

	var graphQlFeedResult Instagram_GraphQl_User_Feed
	err = json.Unmarshal(result, &graphQlFeedResult)
	helpers.Relax(err)

	receivedPosts := graphQlFeedResult.Data.User.EdgeOwnerToTimelineMedia.Edges

	for i := len(receivedPosts)/2 - 1; i >= 0; i-- {
		opp := len(receivedPosts) - 1 - i
		receivedPosts[i], receivedPosts[opp] = receivedPosts[opp], receivedPosts[i]
	}

	for _, receivedPost := range graphQlFeedResult.Data.User.EdgeOwnerToTimelineMedia.Edges {
		posts = append(posts, InstagramShortPostInformation{
			ID:        receivedPost.Node.ID + "_" + accountID,
			Shortcode: receivedPost.Node.Shortcode,
			CreatedAt: time.Unix(int64(receivedPost.Node.TakenAtTimestamp), 0),
		})
	}

	return
}

func (m *Handler) getStory(accountID string, proxy http.Transport) (err error) {
	graphQlUrl := m.graphQlStoryUrl(accountID)
	fmt.Println(graphQlUrl)
	result, err := helpers.NetGetUAWithErrorAndTransport(graphQlUrl, helpers.DEFAULT_UA, proxy)
	if err != nil {
		return
	}

	spew.Dump(string(result))

	return nil
}

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

func (m *Handler) getUserInformation(username string, proxy http.Transport) (information InstagramAuthorInformations, err error) {
	targetLink := "https://www.instagram.com/" + username + "/?__a=1"
	profileSite, err := helpers.NetGetUAWithErrorAndTransport(targetLink, helpers.DEFAULT_UA, proxy)
	if err != nil {
		return
	}

	var profileGraphQl Instagram_GraphQl_User_Profile
	err = json.Unmarshal(profileSite, &profileGraphQl)
	if err != nil {
		return
	}

	information = InstagramAuthorInformations{
		ID:            profileGraphQl.Graphql.User.ID,
		ProfilePicUrl: profileGraphQl.Graphql.User.ProfilePicURLHd,
		Username:      profileGraphQl.Graphql.User.Username,
		FullName:      profileGraphQl.Graphql.User.FullName,
		Link:          profileGraphQl.Graphql.User.ExternalURL,
		IsPrivate:     profileGraphQl.Graphql.User.IsPrivate,
		IsVerified:    profileGraphQl.Graphql.User.IsVerified,
		Followings:    profileGraphQl.Graphql.User.EdgeFollow.Count,
		Followers:     profileGraphQl.Graphql.User.EdgeFollowedBy.Count,
		Posts:         profileGraphQl.Graphql.User.EdgeOwnerToTimelineMedia.Count,
		Biography:     profileGraphQl.Graphql.User.Biography,
	}

	if information.ID == "" {
		return information, errors.New("failed to find user information")
	}

	return
}
