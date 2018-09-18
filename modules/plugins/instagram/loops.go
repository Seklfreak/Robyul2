package instagram

import (
	"strings"
	"time"

	"sync"

	"net/url"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/globalsign/mgo/bson"
)

var (
	instagramEntryLocks = make(map[string]*sync.Mutex)
)

const (
	InstagramGraphQlWorkers = 15
)

func (m *Handler) checkInstagramPublicFeedLoop() {
	log := cache.GetLogger().WithField("module", "instagram")

	defer helpers.Recover()
	defer func() {
		go func() {
			defer helpers.Recover()
			log.Error("The checkInstagramPublicFeedLoop died." +
				"Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			m.checkInstagramPublicFeedLoop()
		}()
	}()

	var wg sync.WaitGroup
	for {
		bundledEntries, entriesCount, err := m.getBundledEntries()
		helpers.Relax(err)

		log.Infof(
			"checking graphql feed on %d accounts for %d feeds with %d workers",
			len(bundledEntries), entriesCount, InstagramGraphQlWorkers)
		start := time.Now()

		wg.Add(InstagramGraphQlWorkers)

		jobs := make(chan map[string][]models.InstagramEntry, InstagramGraphQlWorkers)

		workerEntries := make(map[int]map[string][]models.InstagramEntry, 0)
		for w := 1; w <= InstagramGraphQlWorkers; w++ {
			go m.checkInstagramPublicFeedLoopWorker(w, jobs, &wg)
			workerEntries[w] = make(map[string][]models.InstagramEntry)
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

		wg.Wait()
		elapsed := time.Since(start)
		log.Infof(
			"checked graphql feed on %d accounts for %d feeds with %d workers, took %s",
			len(bundledEntries), entriesCount, InstagramGraphQlWorkers, elapsed)
		metrics.InstagramGraphQlFeedRefreshTime.Set(elapsed.Seconds())

		if entriesCount <= 10 {
			time.Sleep(60 * time.Second)
		}
	}
}

func (m *Handler) checkInstagramPublicFeedLoopWorker(id int, jobs <-chan map[string][]models.InstagramEntry, wg *sync.WaitGroup) {
	defer helpers.Recover()
	defer func() {
		if wg != nil {
			wg.Done()
		}
	}()

	currentProxy, err := helpers.GetRandomProxy()
	helpers.Relax(err)

	for job := range jobs {
		//cache.GetLogger().WithField("module", "instagram").WithField("worker", id).Infof(
		//	"worker %d started for %d accounts", id, len(job))
	NextEntry:
		for instagramUsername, entries := range job {
			//cache.GetLogger().WithField("module", "instagram").WithField("worker", id).Infof(
			//	"checking graphql feed for %d for %d channels", instagramAccountID, len(entries))
		RetryGraphQl:
			_, receivedPosts, err := m.getInformationAndPosts(instagramUsername, currentProxy)
			if err != nil {
				if strings.Contains(err.Error(), "expected status 200; got 404") {
					// account got deleted/username got changed
					continue NextEntry
				}
				if m.retryOnError(err) {
					//cache.GetLogger().WithField("module", "instagram").Infof(
					//	"proxy error connecting to Instagram Account %s (GraphQL), "+
					//		"waiting 5 seconds, switching proxy and then trying again", instagramAccountID)
					time.Sleep(5 * time.Second)
					currentProxy, err = helpers.GetRandomProxy()
					helpers.Relax(err)
					goto RetryGraphQl
				}
				helpers.RelaxLog(err)
				continue NextEntry
			}

			postCheckTime := time.Now()

			for _, receivedPost := range receivedPosts {
				postHasBeenPostedEverywhere := true
				for _, entry := range entries {
					postAlreadyPosted := false
					if receivedPost.CreatedAt.Before(entry.LastPostCheck) {
						postAlreadyPosted = true
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
				post, err := m.getPostInformation(receivedPost.Shortcode, currentProxy)
				if err != nil {
					if m.retryOnError(err) {
						//cache.GetLogger().WithField("module", "instagram").Infof(
						//	"hit rate limit checking Instagram Account %s (GraphQL), "+
						//		"waiting 5 seconds, switching proxy and then trying again", instagramAccountID)
						time.Sleep(5 * time.Second)
						currentProxy, err = helpers.GetRandomProxy()
						helpers.Relax(err)
						goto RetryPost
					}
					if strings.Contains(err.Error(), "expected status 200; got 404") {
						// post got deleted
						continue
					}
					helpers.RelaxLog(err)
					continue NextEntry
				}

				for _, entry := range entries {
					entryID := entry.ID
					m.lockEntry(entryID)

					var entry models.InstagramEntry
					err = helpers.MdbOneWithoutLogging(
						helpers.MdbCollection(models.InstagramTable).Find(bson.M{"_id": entryID}),
						&entry,
					)

					if entry.LastPostCheck.IsZero() { // prevent spam
						entry.LastPostCheck = time.Now()
					}

					if err != nil {
						m.unlockEntry(entryID)
						helpers.RelaxLog(err)
						continue
					}

					postAlreadyPosted := false
					if receivedPost.CreatedAt.Before(entry.LastPostCheck) {
						postAlreadyPosted = true
					}

					if postAlreadyPosted == false {
						cache.GetLogger().WithField("module", "instagram").Infof("Posting Post (GraphQL): #%s", post.ID)
						go m.postPostToChannel(entry.ChannelID, post, entry.SendPostType)
					}

					entry.LastPostCheck = postCheckTime
					err = helpers.MDbUpdateWithoutLogging(models.InstagramTable, entry.ID, entry)
					if err != nil {
						m.unlockEntry(entryID)
						helpers.RelaxLog(err)
						continue
					}

					m.unlockEntry(entryID)
				}
			}
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

func (m *Handler) retryOnError(err error) (retry bool) {
	if err != nil {
		if _, ok := err.(*url.Error); ok ||
			strings.Contains(err.Error(), "net/http") ||
			strings.Contains(err.Error(), "expected status 200; got 429") ||
			strings.Contains(err.Error(), "Please wait a few minutes before you try again.") ||
			strings.Contains(err.Error(), "expected status 200; got 500") ||
			strings.Contains(err.Error(), "expected status 200; got 502") ||
			strings.Contains(err.Error(), "expected status 200; got 503") ||
			strings.Contains(err.Error(), "tls: bad record MAC") ||
			strings.Contains(err.Error(), "unexpected EOF") {
			return true
		}
	}
	return false
}
