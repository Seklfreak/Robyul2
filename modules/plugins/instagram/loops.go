package instagram

import (
	"strings"
	"time"

	"encoding/json"

	"strconv"

	"sync"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/models"
	goinstaResponse "github.com/ahmdrz/goinsta/response"
	"github.com/globalsign/mgo/bson"
)

var (
	instagramEntryLocks = make(map[string]*sync.Mutex)
)

const (
	InstagramGraphQlWorkers = 15
)

func (m *Handler) checkInstagramGraphQlFeedLoop() {
	log := cache.GetLogger().WithField("module", "instagram")

	defer helpers.Recover()
	defer func() {
		go func() {
			log.Error("The checkInstagramGraphQlFeedLoop died." +
				"Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			m.checkInstagramGraphQlFeedLoop()
		}()
	}()

	for {
		bundledEntries, entriesCount, err := m.getBundledEntries()
		helpers.Relax(err)

		cache.GetLogger().Infof(
			"checking graphql feed on %d accounts for %d feeds with %d workers",
			len(bundledEntries), entriesCount, InstagramGraphQlWorkers)
		start := time.Now()

		jobs := make(chan map[int64][]models.InstagramEntry, 0)
		results := make(chan int, 0)

		workerEntries := make(map[int]map[int64][]models.InstagramEntry, 0)
		for w := 1; w <= InstagramGraphQlWorkers; w++ {
			go m.checkInstagramGraphQlFeedWorker(w, jobs, results)
			workerEntries[w] = make(map[int64][]models.InstagramEntry)
		}

		lastWorker := 1
		for code, codeEntries := range bundledEntries {
			workerEntries[lastWorker][code] = codeEntries
			lastWorker++
			if lastWorker > InstagramGraphQlWorkers {
				lastWorker = 1
			}
		}

		for _, workerEntry := range workerEntries {
			jobs <- workerEntry
		}
		close(jobs)

		for a := 1; a <= InstagramGraphQlWorkers; a++ {
			<-results
		}
		elapsed := time.Since(start)
		cache.GetLogger().Infof(
			"checked graphql feed on %d accounts for %d feeds with %d workers, took %s",
			len(bundledEntries), entriesCount, InstagramGraphQlWorkers, elapsed)
		metrics.InstagramGraphQlFeedRefreshTime.Set(elapsed.Seconds())

		if entriesCount <= 10 {
			time.Sleep(60 * time.Second)
		}
	}
}

func (m *Handler) checkInstagramGraphQlFeedWorker(id int, jobs <-chan map[int64][]models.InstagramEntry, results chan<- int) {
	defer helpers.Recover()

	var graphQlFeedResult Instagram_GraphQl_User_Feed

	currentProxy, err := helpers.GetRandomProxy()
	helpers.Relax(err)
	cache.GetLogger().WithField("module", "instagram").Infof("switched to random proxy")

	for job := range jobs {
		//cache.GetLogger().WithField("module", "instagram").WithField("worker", id).Infof(
		//	"worker %d started for %d accounts", id, len(job))
	NextEntry:
		for instagramAccountID, entries := range job {
			//cache.GetLogger().WithField("module", "instagram").WithField("worker", id).Infof(
			//	"checking graphql feed for %d for %d channels", instagramAccountID, len(entries))
		RetryGraphQl:
			graphQlUrl := m.graphQlMediaUrl(instagramAccountID)
			result, err := helpers.NetGetUAWithErrorAndTransport(graphQlUrl, helpers.DEFAULT_UA, currentProxy)
			if err != nil {
				if strings.Contains(err.Error(), "expected status 200; got 429") {
					cache.GetLogger().WithField("module", "instagram").Infof(
						"hit rate limit checking Instagram Account %d (GraphQL), "+
							"sleeping for 1 seconds, switching proxy and then trying again", instagramAccountID)
					time.Sleep(1 * time.Second)
					currentProxy, err = helpers.GetRandomProxy()
					helpers.Relax(err)
					cache.GetLogger().WithField("module", "instagram").Infof(
						"switched to random proxy")
					goto RetryGraphQl
				}
				cache.GetLogger().WithField("module", "instagram").Infof(
					"failed to connect to proxy checking Instagram Account %d (GraphQL): %s, "+
						"proxy dead?, sleeping for 1 second, switching proxy and then trying again",
					instagramAccountID, err.Error())
				time.Sleep(1 * time.Second)
				currentProxy, err = helpers.GetRandomProxy()
				helpers.Relax(err)
				cache.GetLogger().WithField("module", "instagram").Infof("switched to random proxy")
				goto RetryGraphQl
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
						if postedPosts.Type == models.InstagramPostTypePost && postedPosts.ID == fullPostID {
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

				// download specific post data
			RetryPost:
				post, err := m.getPostInformation(receivedPost.Node.Shortcode, currentProxy)
				if err != nil {
					if strings.Contains(err.Error(), "expected status 200; got 429") {
						cache.GetLogger().WithField("module", "instagram").Infof(
							"hit rate limit checking Instagram Account %d (GraphQL), "+
								"sleeping for 1 seconds, switching proxy and then trying again", instagramAccountID)
						time.Sleep(1 * time.Second)
						currentProxy, err = helpers.GetRandomProxy()
						helpers.Relax(err)
						cache.GetLogger().WithField("module", "instagram").Infof(
							"switched to random proxy")
						goto RetryPost
					}
					helpers.RelaxLog(err)
					continue NextEntry
				}

				for _, entry := range entries {
					entryID := entry.ID
					m.lockEntry(entryID)

					var entry models.InstagramEntry
					err = helpers.MdbOne(
						helpers.MdbCollection(models.InstagramTable).Find(bson.M{"_id": entryID}),
						&entry,
					)
					if err != nil {
						m.unlockEntry(entryID)
						helpers.RelaxLog(err)
						continue
					}

					changes := false
					postAlreadyPosted := false
					for _, postedPosts := range entry.PostedPosts {
						if postedPosts.Type == models.InstagramPostTypePost && postedPosts.ID == fullPostID {
							postAlreadyPosted = true
						}
					}

					if postAlreadyPosted == false {
						cache.GetLogger().WithField("module", "instagram").Infof("Posting Post (GraphQL): #%s", post.ID)
						entry.PostedPosts = append(entry.PostedPosts,
							models.InstagramPostEntry{
								ID:            post.ID,
								Type:          models.InstagramPostTypePost,
								CreatedAtTime: post.TakentAt,
							})
						changes = true
						go m.postPostToChannel(entry.ChannelID, post, entry.SendPostType)
					}

					if changes {
						err = helpers.MDbUpdate(models.InstagramTable, entry.ID, entry)
						if err != nil {
							m.unlockEntry(entryID)
							helpers.RelaxLog(err)
							continue
						}
					}

					m.unlockEntry(entryID)
				}
			}
		}
	}
	results <- len(jobs)
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
			story, err := instagramClient.GetUserStories(instagramAccountID)
			if err != nil || story.Status != "ok" {
				if err != nil &&
					strings.Contains(err.Error(), "Please wait a few minutes before you try again.") {
					cache.GetLogger().WithField("module", "instagram").Infof(
						"hit rate limit checking Instagram Account (Stories) %d, "+
							"sleeping for 20 seconds and then trying again", instagramAccountID)
					time.Sleep(20 * time.Second)
					goto RetryAccount
				}
				log.WithField("module", "instagram").Warnf(
					"updating instagram account %d (Story) failed: %s", instagramAccountID, err)
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

				entryID := entry.ID
				m.lockEntry(entryID)

				var entry models.InstagramEntry
				err = helpers.MdbOne(
					helpers.MdbCollection(models.InstagramTable).Find(bson.M{"_id": entryID}),
					&entry,
				)
				if err != nil {
					m.unlockEntry(entryID)
					if !strings.Contains(err.Error(), "The result does not contain any more rows") {
						helpers.RelaxLog(err)
					}
					continue
				}

				for n, reelMedia := range story.Reel.Items {
					reelMediaAlreadyPosted := false
					for _, reelMediaPostPosted := range entry.PostedPosts {
						if reelMediaPostPosted.Type == models.InstagramPostTypeReel && reelMediaPostPosted.ID == reelMedia.ID {
							reelMediaAlreadyPosted = true
						}
					}
					if reelMediaAlreadyPosted == false {
						log.WithField("module", "instagram").Infof(
							"Posting Reel Media (Feed and Story): #%s", reelMedia.ID)
						entry.PostedPosts = append(entry.PostedPosts,
							models.InstagramPostEntry{
								ID:            reelMedia.ID,
								Type:          models.InstagramPostTypeReel,
								CreatedAtTime: time.Unix(int64(reelMedia.DeviceTimestamp), 0),
							})
						changes = true
						go m.postReelMediaToChannel(entry.ChannelID, story, n, entry.SendPostType)
					}

				}

				// TODO: no broadcast information received from story anymore?
				/*
				   if entry.IsLive == false {
				       if story.Broadcast != 0 {
				           log.WithField("module", "instagram").Info(fmt.Sprintf("Posting Live (Feed and Story): #%s", instagramUser.User.Broadcast.ID))
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
					err = helpers.MDbUpdate(models.InstagramTable, entry.ID, entry)
					if err != nil {
						m.unlockEntry(entryID)
						helpers.RelaxLog(err)
						continue
					}
				}

				m.unlockEntry(entryID)
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

func (m *Handler) lockEntry(entryID bson.ObjectId) {
	if _, ok := instagramEntryLocks[string(entryID)]; ok {
		instagramEntryLocks[string(entryID)].Lock()
		return
	}
	instagramEntryLocks[string(entryID)] = new(sync.Mutex)
	instagramEntryLocks[string(entryID)].Lock()
}

func (m *Handler) unlockEntry(entryID bson.ObjectId) {
	if _, ok := instagramEntryLocks[string(entryID)]; ok {
		instagramEntryLocks[string(entryID)].Unlock()
	}
}
