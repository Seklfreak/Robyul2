package instagram

import (
	"math/rand"
	"strings"
	"time"

	"encoding/json"

	"net/url"

	"strconv"

	"net/http"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	goinstaResponse "github.com/ahmdrz/goinsta/response"
)

func (m *Handler) checkInstagramGraphQlFeedLoop() {
	if !useGraphQlQuery {
		return
	}

	log := cache.GetLogger()

	defer helpers.Recover()
	defer func() {
		go func() {
			log.WithField("module", "instagram").Error("The checkInstagramGraphQlFeedLoop died." +
				"Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			m.checkInstagramGraphQlFeedLoop()
		}()
	}()

	proxyList := make([]http.Transport, 0)

	for {
		proxy, err := helpers.GimmeProxy()
		helpers.Relax(err)
		proxyList = append(proxyList, proxy)
		if len(proxyList) >= 4 {
			break
		}
	}

	s := rand.NewSource(time.Now().Unix())
	r := rand.New(s)

	currentProxy := proxyList[r.Intn(len(proxyList))]
	cache.GetLogger().WithField("module", "instagram").Infof("switched to random proxy")

	var graphQlFeedResult Instagram_GraphQl_User_Feed

	for {
		bundledEntries, entriesCount, err := m.getBundledEntries()
		helpers.Relax(err)

		cache.GetLogger().WithField("module", "instagram").Infof(
			"checking graphql feed on %d accounts for %d feeds", len(bundledEntries), entriesCount)
		start := time.Now()

		for instagramAccountID, entries := range bundledEntries {
			// log.WithField("module", "instagram").Debug(fmt.Sprintf("checking Instagram Account @%s", instagramUsername))

			jsonData, err := json.Marshal(struct {
				ID    string `json:"id"`
				First string `json:"first"`
			}{ID: strconv.Itoa(int(instagramAccountID)), First: "10"})
			helpers.Relax(err)

		RetryGraphQl:
			graphQlUrl := "https://www.instagram.com/graphql/query/" +
				"?query_id=17888483320059182" +
				"&variables=" + url.QueryEscape(string(jsonData))
			result, err := helpers.NetGetUAWithErrorAndTransport(graphQlUrl, helpers.DEFAULT_UA, currentProxy)
			if err != nil {
				if strings.Contains(err.Error(), "expected status 200; got 429") {
					cache.GetLogger().WithField("module", "instagram").Infof(
						"hit rate limit checking Instagram Account %d (GraphQL), "+
							"sleeping for 5 seconds, switching proxy and then trying again", instagramAccountID)
					time.Sleep(5 * time.Second)
					currentProxy = proxyList[r.Intn(len(proxyList))]
					cache.GetLogger().WithField("module", "instagram").Infof("switched to random proxy")
					goto RetryGraphQl
				}
				cache.GetLogger().WithField("module", "instagram").Warnf(
					"getting graphql %s failed", graphQlUrl)
				helpers.Relax(err)
			}

			err = json.Unmarshal(result, &graphQlFeedResult)
			helpers.Relax(err)

			receivedPosts := graphQlFeedResult.Data.User.EdgeOwnerToTimelineMedia.Edges

			// https://github.com/golang/go/wiki/SliceTricks#reversing
			for i := len(receivedPosts)/2 - 1; i >= 0; i-- {
				opp := len(receivedPosts) - 1 - i
				receivedPosts[i], receivedPosts[opp] = receivedPosts[opp], receivedPosts[i]
			}

			for _, receivedPost := range receivedPosts {
				fullPostID := receivedPost.Node.ID + "_" + strconv.Itoa(int(instagramAccountID))

				postHasBeenPostedEverywhere := true
				for _, entry := range entries {
					postAlreadyPosted := false
					for _, postedPosts := range entry.PostedPosts {
						if postedPosts.ID == fullPostID {
							postAlreadyPosted = true
						}
					}
					if !postAlreadyPosted {
						postHasBeenPostedEverywhere = false
					}
				}

				if postHasBeenPostedEverywhere {
					continue
				}

			RetryPost:
				postsData, err := instagramClient.MediaInfo(receivedPost.Node.ID)
				if err != nil || postsData.Status != "ok" {
					if err != nil &&
						strings.Contains(err.Error(), "Please wait a few minutes before you try again.") {
						cache.GetLogger().WithField("module", "instagram").Infof(
							"hit rate limit checking Instagram Account %d,"+
								"sleeping for 20 seconds and then trying again", instagramAccountID)
						time.Sleep(20 * time.Second)
						goto RetryPost
					}
					log.WithField("module", "instagram").Warnf(
						"failed to get post information for %s account %d failed: %s",
						receivedPost.Node.ID, instagramAccountID, err)
					continue
				}

				var post goinstaResponse.Item
				for _, postData := range postsData.Items {
					if postData.ID == fullPostID {
						post = postData
					}
				}

				if post.ID == "" {
					log.WithField("module", "instagram").Warnf(
						"failed to find post information in returned post information for %s account %d failed: %s",
						receivedPost.Node.ID, instagramAccountID, err)
					continue
				}

				for _, entry := range entries {
					changes := false
					postAlreadyPosted := false
					for _, postedPosts := range entry.PostedPosts {
						if postedPosts.ID == fullPostID {
							postAlreadyPosted = true
						}
					}

					if postAlreadyPosted == false {
						log.WithField("module", "instagram").Infof("Posting Post: #%s", post.ID)
						entry.PostedPosts = append(entry.PostedPosts,
							DB_Instagram_Post{ID: post.ID, CreatedAt: post.Caption.CreatedAt})
						changes = true
						go m.postPostToChannel(entry.ChannelID, post, entry.PostDirectLinks)
					}

					if changes {
						lockPostedPosts.Lock()
						m.setEntry(entry)
						lockPostedPosts.Unlock()
					}
				}
			}

			time.Sleep(2 * time.Second)
		}

		elapsed := time.Since(start)
		cache.GetLogger().WithField("module", "instagram").Infof(
			"checked graphql on %d accounts for %d feeds, took %s",
			len(bundledEntries), entriesCount, elapsed)
		metrics.InstagramGraphQlFeedRefreshTime.Set(elapsed.Seconds())

		if entriesCount <= 10 {
			time.Sleep(30 * time.Second)
		}
	}
}

func (m *Handler) checkInstagramFeedsAndStoryLoop() {
	log := cache.GetLogger()

	defer helpers.Recover()
	defer func() {
		go func() {
			log.WithField("module", "instagram").Error("The checkInstagramFeedsAndStoryLoop died." +
				"Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			m.checkInstagramFeedsAndStoryLoop()
		}()
	}()

	for {
		bundledEntries, entriesCount, err := m.getBundledEntries()
		helpers.Relax(err)

		cache.GetLogger().WithField("module", "instagram").Infof(
			"checking feed and story on %d accounts for %d feeds", len(bundledEntries), entriesCount)
		start := time.Now()

		for instagramAccountID, entries := range bundledEntries {
		RetryAccount:
			// log.WithField("module", "instagram").Debug(fmt.Sprintf("checking Instagram Account @%s", instagramUsername))

			var posts goinstaResponse.UserFeedResponse
			if !useGraphQlQuery {
				posts, err := instagramClient.LatestUserFeed(instagramAccountID)
				if err != nil || posts.Status != "ok" {
					if err != nil &&
						strings.Contains(err.Error(), "Please wait a few minutes before you try again.") {
						cache.GetLogger().WithField("module", "instagram").Infof(
							"hit rate limit checking Instagram Account %d,"+
								"sleeping for 20 seconds and then trying again", instagramAccountID)
						time.Sleep(20 * time.Second)
						goto RetryAccount
					}
					log.WithField("module", "instagram").Warnf(
						"updating instagram account %d failed: %s", instagramAccountID, err)
					continue
				}
			}
			story, err := instagramClient.GetUserStories(instagramAccountID)
			if err != nil || story.Status != "ok" {
				if err != nil &&
					strings.Contains(err.Error(), "Please wait a few minutes before you try again.") {
					cache.GetLogger().WithField("module", "instagram").Infof(
						"hit rate limit checking Instagram Account %d,"+
							"sleeping for 20 seconds and then trying again", instagramAccountID)
					time.Sleep(20 * time.Second)
					goto RetryAccount
				}
				log.WithField("module", "instagram").Warnf(
					"updating instagram account %d failed: %s", instagramAccountID, err)
				continue
			}

			// https://github.com/golang/go/wiki/SliceTricks#reversing
			for i := len(posts.Items)/2 - 1; i >= 0; i-- {
				opp := len(posts.Items) - 1 - i
				posts.Items[i], posts.Items[opp] = posts.Items[opp], posts.Items[i]
			}
			for i := len(story.Reel.Items)/2 - 1; i >= 0; i-- {
				opp := len(story.Reel.Items) - 1 - i
				story.Reel.Items[i], story.Reel.Items[opp] = story.Reel.Items[opp], story.Reel.Items[i]
			}

			for _, entry := range entries {
				changes := false
				for _, post := range posts.Items {
					postAlreadyPosted := false
					for _, postedPosts := range entry.PostedPosts {
						if postedPosts.ID == post.ID {
							postAlreadyPosted = true
						}
					}
					if postAlreadyPosted == false {
						log.WithField("module", "instagram").Infof("Posting Post: #%s", post.ID)
						entry.PostedPosts = append(entry.PostedPosts,
							DB_Instagram_Post{ID: post.ID, CreatedAt: post.Caption.CreatedAt})
						changes = true
						go m.postPostToChannel(entry.ChannelID, post, entry.PostDirectLinks)
					}

				}

				for n, reelMedia := range story.Reel.Items {
					reelMediaAlreadyPosted := false
					for _, reelMediaPostPosted := range entry.PostedReelMedias {
						if reelMediaPostPosted.ID == reelMedia.ID {
							reelMediaAlreadyPosted = true
						}
					}
					if reelMediaAlreadyPosted == false {
						log.WithField("module", "instagram").Infof(
							"Posting Reel Media: #%s", reelMedia.ID)
						entry.PostedReelMedias = append(entry.PostedReelMedias,
							DB_Instagram_ReelMedia{ID: reelMedia.ID, CreatedAt: int64(reelMedia.DeviceTimestamp)})
						changes = true
						go m.postReelMediaToChannel(entry.ChannelID, story, n, entry.PostDirectLinks)
					}

				}

				// TODO: no broadcast information received from story anymore?
				/*
				   if entry.IsLive == false {
				       if story.Broadcast != 0 {
				           log.WithField("module", "instagram").Info(fmt.Sprintf("Posting Live: #%s", instagramUser.User.Broadcast.ID))
				           go m.postLiveToChannel(entry.ChannelID, instagramUser)
				           entry.IsLive = true
				           changes = true
				       }
				   } else {
				       if instagramUser.User.Broadcast.ID == 0 {
				           entry.IsLive = false
				           changes = true
				       }
				   }*/

				if changes == true {
					lockPostedPosts.Lock()
					m.setEntry(entry)
					lockPostedPosts.Unlock()
				}
			}
			time.Sleep(1 * time.Second)
		}

		elapsed := time.Since(start)
		cache.GetLogger().WithField("module", "instagram").Infof(
			"checked feed and story on %d accounts for %d feeds, took %s",
			len(bundledEntries), entriesCount, elapsed)
		metrics.InstagramRefreshTime.Set(elapsed.Seconds())

		if entriesCount <= 10 {
			time.Sleep(30 * time.Second)
		}
	}
}
