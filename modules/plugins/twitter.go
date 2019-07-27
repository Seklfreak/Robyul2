package plugins

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"html"

	"strconv"

	"net/url"

	"github.com/ChimeraCoder/anaconda"
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/emojis"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	humanize "github.com/dustin/go-humanize"
	"github.com/globalsign/mgo/bson"
	"github.com/pkg/errors"
)

type Twitter struct{}

var (
	anacondaClient           *anaconda.TwitterApi
	twitterClient            *twitter.Client
	twitterStream            *anaconda.Stream
	twitterStreamNeedsUpdate bool
	twitterEntriesCache      []models.TwitterEntry
	twitterStreamIsStarting  sync.Mutex
	twitterEntryLocks        = make(map[string]*sync.Mutex)
	twitterEntryLock         sync.Mutex
)

const (
	TwitterFriendlyUser   = "https://twitter.com/%s"
	TwitterFriendlyStatus = "https://twitter.com/%s/status/%s"
	rfc2822               = "Mon Jan 02 15:04:05 -0700 2006"
	twitterStreamLimit    = 5000
)

func (m *Twitter) Commands() []string {
	return []string{
		"twitter",
	}
}

func (t *Twitter) Init(session *discordgo.Session) {
	config := oauth1.NewConfig(
		helpers.GetConfig().Path("twitter.consumer_key").Data().(string),
		helpers.GetConfig().Path("twitter.consumer_secret").Data().(string))
	token := oauth1.NewToken(
		helpers.GetConfig().Path("twitter.access_token").Data().(string),
		helpers.GetConfig().Path("twitter.access_secret").Data().(string))
	httpClient := config.Client(oauth1.NoContext, token)
	twitterClient = twitter.NewClient(httpClient)

	anaconda.SetConsumerKey(helpers.GetConfig().Path("twitter.consumer_key").Data().(string))
	anaconda.SetConsumerSecret(helpers.GetConfig().Path("twitter.consumer_secret").Data().(string))
	anacondaClient = anaconda.NewTwitterApi(
		helpers.GetConfig().Path("twitter.access_token").Data().(string),
		helpers.GetConfig().Path("twitter.access_secret").Data().(string),
	)
	go func() {
		defer helpers.Recover()

		for {
			if twitterStream == nil {
				time.Sleep(1 * time.Second)
				continue
			}
			for event := range twitterStream.C {
				switch item := event.(type) {
				case anaconda.Tweet:
					for _, entry := range twitterEntriesCache {
						if entry.AccountID != item.User.IdStr {
							continue
						}

						// exclude RTs?
						if entry.ExcludeRTs && item.RetweetedStatus != nil {
							continue
						}

						// exclude Mentions?
						if entry.ExcludeMentions && strings.HasPrefix(item.Text, "@") {
							continue
						}

						entryID := entry.ID
						t.lockEntry(entryID)

						err := helpers.MdbOneWithoutLogging(
							helpers.MdbCollection(models.TwitterTable).Find(bson.M{"_id": entry.ID}),
							&entry,
						)
						if err != nil {
							t.unlockEntry(entryID)
							helpers.RelaxLog(err)
							continue
						}

						changes := false
						tweetAlreadyPosted := false

						for _, postedTweet := range entry.PostedTweets {
							if postedTweet.ID == item.IdStr {
								tweetAlreadyPosted = true
							}
						}
						if tweetAlreadyPosted == false {
							// cache.GetLogger().WithField("module", "twitter").Info(fmt.Sprintf("posting tweet (via streaming): #%s to: #%s", item.IdStr, entry.ChannelID))
							entry.PostedTweets = append(entry.PostedTweets, models.TwitterTweetEntry{ID: item.IdStr, CreatedAt: item.CreatedAt})
							changes = true
							go t.postAnacondaTweetToChannel(entry.ChannelID, &item, &item.User, entry)
						}

						if changes == true {
							err = helpers.MDbUpsertIDWithoutLogging(
								models.TwitterTable,
								entry.ID,
								entry,
							)
							if err != nil {
								t.unlockEntry(entryID)
								helpers.RelaxLog(err)
								continue
							}
						}

						t.unlockEntry(entryID)
					}
				case anaconda.StallWarning:
					cache.GetLogger().WithField("module", "twitter").Warn("received stall warning from twitter stream:", item.Message)
				}
			}
		}
	}()

	go t.startTwitterStream()
	go t.updateTwitterStreamLoop()

	go func() {
		// wait for twitterEntriesCache to initialize
		time.Sleep(30 * time.Second)
		// TODO: only to REST API check on start or after stream restarts
		go t.checkTwitterFeedsLoop()
		cache.GetLogger().WithField("module", "twitter").Info("started twitter loop (10m)")
	}()
}

func (t *Twitter) Uninit(session *discordgo.Session) {
	t.stopTwitterStream()
}

func (t *Twitter) startTwitterStream() {
	defer helpers.Recover()

	twitterStreamIsStarting.Lock()
	defer twitterStreamIsStarting.Unlock()

	var err error
	var accountIDs []string

	err = helpers.MDbIterWithoutLogging(
		helpers.MdbCollection(models.TwitterTable).Find(nil).Sort("_id"),
	).All(&twitterEntriesCache)
	helpers.Relax(err)

	for _, entry := range twitterEntriesCache {
		// check if channel exists
		channel, err := helpers.GetChannelWithoutApi(entry.ChannelID)
		if err != nil || channel == nil || channel.ID == "" {
			continue
		}

		// check if we can send messages
		channelPermission, err := cache.GetSession().State.UserChannelPermissions(cache.GetSession().State.User.ID, channel.ID)
		if err != nil {
			continue
		}

		if channelPermission&discordgo.PermissionSendMessages != discordgo.PermissionSendMessages {
			continue
		}

		if entry.PostMode == models.TwitterPostModeRobyulEmbed {
			if channelPermission&discordgo.PermissionEmbedLinks != discordgo.PermissionEmbedLinks {
				continue
			}
		}

		idToAdd := entry.AccountID

		if idToAdd == "" && entry.AccountScreenName != "" {
			user, _, err := twitterClient.Users.Show(&twitter.UserShowParams{
				ScreenName: entry.AccountScreenName,
			})
			if err != nil {
				if strings.Contains(err.Error(), "User not found.") {
					continue
				}
			}
			helpers.RelaxLog(err)
			if err == nil {
				idToAdd = user.IDStr
				if idToAdd != "" && idToAdd != "0" {
					entry.AccountID = idToAdd
					err = helpers.MDbUpsertID(
						models.TwitterTable,
						entry.ID,
						entry,
					)
					if err != nil {
						helpers.RelaxLog(err)
						continue
					}
				}
			}
			cache.GetLogger().WithField("module", "twitter").Infof("saved User ID %s for Twitter Account @%s", idToAdd, entry.AccountScreenName)
		}

		if idToAdd != "" {
			idInSlice := false
			for _, accountID := range accountIDs {
				if idToAdd == accountID {
					idInSlice = true
				}
			}
			if idInSlice == false {
				accountIDs = append(accountIDs, entry.AccountID)
			}
		}
	}

	if len(accountIDs) > twitterStreamLimit {
		accountIDs = accountIDs[0:twitterStreamLimit]
	}

	twitterStream = anacondaClient.PublicStreamFilter(url.Values{
		"follow":         accountIDs,
		"stall_warnings": []string{"true"},
	})
	helpers.Relax(err)
	cache.GetLogger().WithField("module", "twitter").Infof("started Twitter stream for %d accounts", len(accountIDs))
}

func (t *Twitter) stopTwitterStream() {
	if twitterStream != nil {
		twitterStream.Stop()
		cache.GetLogger().WithField("module", "twitter").Info("stopped stream")
	}
}

func (t *Twitter) updateTwitterStreamLoop() {
	defer helpers.Recover()
	defer func() {
		go func() {
			cache.GetLogger().WithField("module", "twitter").Error("the updateTwitterStreamLoop died. Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			t.updateTwitterStreamLoop()
		}()
	}()

	for {
		if twitterStreamNeedsUpdate {
			cache.GetLogger().WithField("module", "twitter").Info("restarting stream since update is required")
			t.stopTwitterStream()
			t.startTwitterStream()
			twitterStreamNeedsUpdate = false
		}

		time.Sleep(10 * time.Minute)
	}
}

func (m *Twitter) checkTwitterFeedsLoop() {
	defer helpers.Recover()
	defer func() {
		go func() {
			cache.GetLogger().WithField("module", "twitter").Error("The checkTwitterFeedsLoop died. Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			m.checkTwitterFeedsLoop()
		}()
	}()

	var err error
	var bundledEntries map[string][]models.TwitterEntry
	var accountID int64

	for {
		bundledEntries = make(map[string][]models.TwitterEntry, 0)

		for _, entry := range twitterEntriesCache {
			// check if channel exists
			channel, err := helpers.GetChannelWithoutApi(entry.ChannelID)
			if err != nil || channel == nil || channel.ID == "" {
				continue
			}

			// check if we can send messages
			channelPermission, err := cache.GetSession().State.UserChannelPermissions(cache.GetSession().State.User.ID, channel.ID)
			if err != nil {
				continue
			}

			if channelPermission&discordgo.PermissionSendMessages != discordgo.PermissionSendMessages {
				continue
			}

			if entry.PostMode == models.TwitterPostModeRobyulEmbed {
				if channelPermission&discordgo.PermissionEmbedLinks != discordgo.PermissionEmbedLinks {
					continue
				}
			}

			if _, ok := bundledEntries[entry.AccountID]; ok {
				bundledEntries[entry.AccountID] = append(bundledEntries[entry.AccountID], entry)
			} else {
				bundledEntries[entry.AccountID] = []models.TwitterEntry{entry}
			}
		}

		cache.GetLogger().WithField("module", "twitter").Infof("checking %d accounts for %d feeds", len(bundledEntries), len(twitterEntriesCache))
		start := time.Now()

		for twitterAccountID, entries := range bundledEntries {
			accountID, err = strconv.ParseInt(twitterAccountID, 10, 64)
			if err != nil {
				continue
			}

			twitterUserTweets, _, err := twitterClient.Timelines.UserTimeline(&twitter.UserTimelineParams{
				UserID:          accountID,
				Count:           10,
				ExcludeReplies:  twitter.Bool(true),
				IncludeRetweets: twitter.Bool(true),
			})
			if err != nil {
				if strings.Contains(err.Error(), "34 Sorry, that page does not exist") ||
					strings.Contains(err.Error(), "50 User not found") ||
					strings.Contains(err.Error(), "63 User has been suspended") {
					for _, entry := range entries {
						err = helpers.MDbDelete(models.TwitterTable, entry.ID)
						if err != nil {
							helpers.RelaxLog(err)
							continue
						}
						cache.GetLogger().WithField("module", "twitter").Infof(
							"removed entry %s (@%s) because user suspended or deleted",
							helpers.MdbIdToHuman(entry.ID), entry.AccountScreenName,
						)
					}
					continue
				}
				cache.GetLogger().WithField("module", "twitter").Warnf("getting tweets of @%d failed: %s", accountID, err.Error())
				continue
			}

			cache.GetLogger().WithField("module", "twitter").Info(fmt.Sprintf("checking Twitter Account %d", accountID))

			// https://github.com/golang/go/wiki/SliceTricks#reversing
			for i := len(twitterUserTweets)/2 - 1; i >= 0; i-- {
				opp := len(twitterUserTweets) - 1 - i
				twitterUserTweets[i], twitterUserTweets[opp] = twitterUserTweets[opp], twitterUserTweets[i]
			}

			for _, entry := range entries {
				entryID := entry.ID
				m.lockEntry(entryID)

				err = helpers.MdbOneWithoutLogging(
					helpers.MdbCollection(models.TwitterTable).Find(bson.M{"_id": entry.ID}),
					&entry,
				)
				if err != nil {
					m.unlockEntry(entryID)
					if !helpers.IsMdbNotFound(err) {
						helpers.RelaxLog(err)
					}
					continue
				}

				changes := false

				for _, tweet := range twitterUserTweets {
					tweetCreatedAt, err := tweet.CreatedAtTime()
					if err != nil || time.Now().Sub(tweetCreatedAt) > time.Hour {
						continue
					}

					// exclude RTs?
					if entry.ExcludeRTs && tweet.RetweetedStatus != nil {
						continue
					}

					// exclude Mentions?
					if entry.ExcludeMentions && strings.HasPrefix(tweet.Text, "@") {
						continue
					}

					tweetAlreadyPosted := false
					for _, postedTweet := range entry.PostedTweets {
						if postedTweet.ID == tweet.IDStr {
							tweetAlreadyPosted = true
						}
					}
					if tweetAlreadyPosted == false {
						// cache.GetLogger().WithField("module", "twitter").Info(fmt.Sprintf("posting tweet (via REST): #%s to: #%s", tweet.IDStr, entry.ChannelID))
						entry.PostedTweets = append(entry.PostedTweets, models.TwitterTweetEntry{ID: tweet.IDStr, CreatedAt: tweet.CreatedAt})
						changes = true
						tweetToPost := tweet
						go m.postTweetToChannel(entry.ChannelID, &tweetToPost, entry)
					}

				}
				if changes == true {
					err = helpers.MDbUpsertIDWithoutLogging(
						models.TwitterTable,
						entry.ID,
						entry,
					)
					if err != nil {
						m.unlockEntry(entryID)
						helpers.RelaxLog(err)
						continue
					}
				}

				m.unlockEntry(entryID)
			}
			time.Sleep(5 * time.Second)
		}

		elapsed := time.Since(start)
		cache.GetLogger().WithField("module", "twitter").Infof("checked %d accounts for %d feeds, took %s", len(bundledEntries), len(twitterEntriesCache), elapsed)
		metrics.TwitterRefreshTime.Set(elapsed.Seconds())

		if len(bundledEntries) <= 10 {
			time.Sleep(10 * time.Minute)
		}
	}
}

func (m *Twitter) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermTwitter) {
		return
	}

	args := strings.Fields(content)
	if len(args) >= 1 {
		switch args[0] {
		case "add": // [p]twitter add <twitter account name (with or without @)> <discord channel>
			helpers.RequireMod(msg, func() {
				session.ChannelTyping(msg.ChannelID)
				// get target channel
				var err error
				var targetChannel *discordgo.Channel
				var targetGuild *discordgo.Guild
				if len(args) >= 3 {
					targetChannel, err = helpers.GetChannelFromMention(msg, args[2])
					if err != nil {
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
						return
					}
				} else {
					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
					return
				}
				targetGuild, err = helpers.GetGuild(targetChannel.GuildID)
				helpers.Relax(err)
				// get twitter account and tweets
				twitterUsername := strings.TrimSpace(strings.Replace(args[1], "@", "", 1))
				twitterUser, _, err := twitterClient.Users.Show(&twitter.UserShowParams{
					ScreenName: twitterUsername,
				})
				if err != nil {
					errText := m.handleError(err)
					helpers.SendMessage(msg.ChannelID, helpers.GetTextF(errText))
					return
				}

				twitterUserTweets, _, err := twitterClient.Timelines.UserTimeline(&twitter.UserTimelineParams{
					ScreenName:      twitterUser.ScreenName,
					Count:           10,
					ExcludeReplies:  twitter.Bool(true),
					IncludeRetweets: twitter.Bool(true),
				})
				if err != nil {
					if strings.Contains(err.Error(), "invalid character 'x' looking for beginning of value") {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.twitter.rate-limit-exceed"))
						return
					}
				}
				helpers.Relax(err)

				mentionRole := new(discordgo.Role)
				if len(args) >= 4 && (args[3] != "discord-embed" && args[3] != "text") {
					mentionRoleName := args[3]
					serverRoles, err := session.GuildRoles(targetGuild.ID)
					if err != nil {
						if errD, ok := err.(*discordgo.RESTError); ok {
							if errD.Message.Code == discordgo.ErrCodeMissingPermissions {
								_, err = helpers.SendMessage(msg.ChannelID, "Please give me the `Manage Roles` permission.")
								helpers.Relax(err)
								return
							} else {
								helpers.Relax(err)
							}
						} else {
							helpers.Relax(err)
						}
					}
					for _, serverRole := range serverRoles {
						if serverRole.Mentionable == true &&
							(strings.ToLower(serverRole.Name) == strings.ToLower(mentionRoleName) || serverRole.ID == mentionRoleName) {
							mentionRole = serverRole
						}
					}
				}
				postMode := models.TwitterPostModeRobyulEmbed
				if strings.Contains(strings.ToLower(content), " discord-embed") {
					postMode = models.TwitterPostModeDiscordEmbed
				}
				if strings.Contains(strings.ToLower(content), " text") {
					postMode = models.TwitterPostModeText
				}
				// Create DB Entries
				var dbTweets []models.TwitterTweetEntry
				for _, tweet := range twitterUserTweets {
					tweetEntry := models.TwitterTweetEntry{ID: tweet.IDStr, CreatedAt: tweet.CreatedAt}
					dbTweets = append(dbTweets, tweetEntry)

				}
				// exclude RTs or Mentions?
				var excludeRTs, excludeMentions bool
				if strings.Contains(strings.ToLower(msg.Content), " exclude-rts") {
					excludeRTs = true
				}
				if strings.Contains(strings.ToLower(msg.Content), " exclude-mentions") {
					excludeMentions = true
				}
				// create new entry in db
				newID, err := helpers.MDbInsert(
					models.TwitterTable,
					models.TwitterEntry{
						GuildID:           targetChannel.GuildID,
						ChannelID:         targetChannel.ID,
						AccountScreenName: twitterUser.ScreenName,
						AccountID:         twitterUser.IDStr,
						PostedTweets:      dbTweets,
						MentionRoleID:     mentionRole.ID,
						PostMode:          postMode,
						ExcludeRTs:        excludeRTs,
						ExcludeMentions:   excludeMentions,
					},
				)
				helpers.Relax(err)

				twitterStreamNeedsUpdate = true

				postModeText := "robyul embed"
				switch postMode {
				case models.TwitterPostModeDiscordEmbed:
					postModeText = "discord embed"
				case models.TwitterPostModeText:
					postModeText = "text"
				}

				_, err = helpers.EventlogLog(time.Now(), targetChannel.GuildID, helpers.MdbIdToHuman(newID),
					models.EventlogTargetTypeRobyulTwitterFeed, msg.Author.ID,
					models.EventlogTypeRobyulTwitterFeedAdd, "",
					nil,
					[]models.ElasticEventlogOption{
						{
							Key:   "twitter_channelid",
							Value: targetChannel.ID,
							Type:  models.EventlogTargetTypeChannel,
						},
						{
							Key:   "twitter_accountscreename",
							Value: twitterUser.ScreenName,
						},
						{
							Key:   "twitter_accountid",
							Value: twitterUser.IDStr,
						},
						{
							Key:   "twitter_mentionroleid",
							Value: mentionRole.ID,
							Type:  models.EventlogTargetTypeRole,
						},
						{
							Key:   "twitter_postmode",
							Value: postModeText,
						},
						{
							Key:   "twitter_exclude_rts",
							Value: helpers.StoreBoolAsString(excludeRTs),
						},
						{
							Key:   "twitter_exclude_mentions",
							Value: helpers.StoreBoolAsString(excludeMentions),
						},
					}, false)
				helpers.RelaxLog(err)

				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.twitter.account-added-success", twitterUser.ScreenName, targetChannel.ID))
				cache.GetLogger().WithField("module", "twitter").Info(fmt.Sprintf("Added Twitter Account @%s to Channel %s (#%s) on Guild %s (#%s)", twitterUser.ScreenName, targetChannel.Name, targetChannel.ID, targetGuild.Name, targetGuild.ID))
			})
		case "delete", "del", "remove": // [p]twitter delete <id>
			helpers.RequireMod(msg, func() {
				if len(args) >= 2 {
					session.ChannelTyping(msg.ChannelID)

					channel, err := helpers.GetChannel(msg.ChannelID)
					helpers.Relax(err)

					entryId := args[1]
					var entryBucket models.TwitterEntry
					err = helpers.MdbOne(
						helpers.MdbCollection(models.TwitterTable).Find(bson.M{"_id": helpers.HumanToMdbId(entryId), "guildid": channel.GuildID}),
						&entryBucket,
					)
					if helpers.IsMdbNotFound(err) || entryBucket.ID == "" {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.twitter.account-delete-not-found-error"))
						return
					}
					helpers.Relax(err)

					err = helpers.MDbDelete(models.TwitterTable, entryBucket.ID)
					helpers.Relax(err)

					twitterStreamNeedsUpdate = true

					postModeText := "robyul embed"
					switch entryBucket.PostMode {
					case models.TwitterPostModeDiscordEmbed:
						postModeText = "discord embed"
					case models.TwitterPostModeText:
						postModeText = "text"
					}

					_, err = helpers.EventlogLog(time.Now(), entryBucket.GuildID, helpers.MdbIdToHuman(entryBucket.ID),
						models.EventlogTargetTypeRobyulTwitterFeed, msg.Author.ID,
						models.EventlogTypeRobyulTwitterFeedRemove, "",
						nil,
						[]models.ElasticEventlogOption{
							{
								Key:   "twitter_channelid",
								Value: entryBucket.ChannelID,
								Type:  models.EventlogTargetTypeChannel,
							},
							{
								Key:   "twitter_accountscreename",
								Value: entryBucket.AccountScreenName,
							},
							{
								Key:   "twitter_accountid",
								Value: entryBucket.AccountID,
							},
							{
								Key:   "twitter_mentionroleid",
								Value: entryBucket.MentionRoleID,
								Type:  models.EventlogTargetTypeRole,
							},
							{
								Key:   "twitter_postmode",
								Value: postModeText,
							},
							{
								Key:   "twitter_exclude_rts",
								Value: helpers.StoreBoolAsString(entryBucket.ExcludeRTs),
							},
							{
								Key:   "twitter_exclude_mentions",
								Value: helpers.StoreBoolAsString(entryBucket.ExcludeMentions),
							},
						}, false)
					helpers.RelaxLog(err)

					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.twitter.account-delete-success", entryBucket.AccountScreenName))
					cache.GetLogger().WithField("module", "twitter").Info(fmt.Sprintf("Deleted Twitter Account @%s", entryBucket.AccountScreenName))

				} else {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					return
				}
			})
		case "list": // [p]twitter list
			currentChannel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)

			var entryBucket []models.TwitterEntry
			err = helpers.MDbIter(helpers.MdbCollection(models.TwitterTable).Find(bson.M{"guildid": currentChannel.GuildID})).All(&entryBucket)
			helpers.Relax(err)

			helpers.Relax(err)
			if entryBucket == nil || len(entryBucket) <= 0 {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.twitter.account-list-no-accounts-error"))
				return
			}

			resultMessage := ""
			for _, entry := range entryBucket {
				var specialText string
				switch entry.PostMode {
				case models.TwitterPostModeDiscordEmbed:
					specialText += " as discord embed"
				case models.TwitterPostModeText:
					specialText += " as text"
				}
				if entry.MentionRoleID != "" {
					role, err := session.State.Role(currentChannel.GuildID, entry.MentionRoleID)
					if err == nil {
						specialText += fmt.Sprintf(" mentioning `@%s`", role.Name)
					} else {
						specialText += " mentioning N/A"
					}
				}
				if entry.ExcludeRTs {
					specialText += " ignoring RTs"
				}
				if entry.ExcludeMentions {
					specialText += " ignoring Mentions"
				}
				resultMessage += fmt.Sprintf("`%s`: Twitter Account `@%s` posting to <#%s>%s\n",
					helpers.MdbIdToHuman(entry.ID), entry.AccountScreenName, entry.ChannelID, specialText)
			}
			resultMessage += fmt.Sprintf("Found **%d** Twitter Accounts in total.", len(entryBucket))
			for _, resultPage := range helpers.Pagify(resultMessage, "\n") {
				_, err := helpers.SendMessage(msg.ChannelID, resultPage)
				helpers.Relax(err)
			}
		default:
			session.ChannelTyping(msg.ChannelID)
			twitterUsername := strings.TrimSpace(strings.Replace(args[0], "@", "", 1))
			twitterUser, _, err := twitterClient.Users.Show(&twitter.UserShowParams{
				ScreenName: twitterUsername,
			})
			if err != nil {
				errText := m.handleError(err)
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF(errText))
				return
			}

			createdAtTime, err := time.Parse(rfc2822, twitterUser.CreatedAt)
			helpers.Relax(err)

			twitterUserDescription := html.UnescapeString(twitterUser.Description)
			if twitterUser.Entities != nil {
				if len(twitterUser.Entities.Description.Urls) > 0 {
					for _, urlEntity := range twitterUser.Entities.Description.Urls {
						if len(urlEntity.ExpandedURL) <= 100 {
							twitterUserDescription = strings.Replace(twitterUserDescription, urlEntity.URL, urlEntity.ExpandedURL, -1)
						}
					}
				}
			}

			twitterNameModifier := ""
			if twitterUser.Verified {
				twitterNameModifier += " :ballot_box_with_check:"
			}
			if twitterUser.Protected {
				twitterNameModifier += " :lock:"
			}
			accountEmbed := &discordgo.MessageEmbed{
				Title:     helpers.GetTextF("plugins.twitter.account-embed-title", twitterUser.Name, twitterUser.ScreenName, twitterNameModifier),
				URL:       fmt.Sprintf(TwitterFriendlyUser, twitterUser.ScreenName),
				Thumbnail: &discordgo.MessageEmbedThumbnail{URL: twitterUser.ProfileImageURLHttps},
				Footer: &discordgo.MessageEmbedFooter{
					Text:    helpers.GetText("plugins.twitter.embed-footer"),
					IconURL: helpers.GetText("plugins.twitter.embed-footer-imageurl"),
				},
				Description: twitterUserDescription,
				Fields: []*discordgo.MessageEmbedField{
					{Name: "Followers", Value: humanize.Comma(int64(twitterUser.FollowersCount)), Inline: true},
					{Name: "Following", Value: humanize.Comma(int64(twitterUser.FriendsCount)), Inline: true},
					{Name: "Tweets", Value: humanize.Comma(int64(twitterUser.StatusesCount)), Inline: true},
					{Name: "Account Creation", Value: humanize.Time(createdAtTime), Inline: true}},
				Color: helpers.GetDiscordColorFromHex(twitterUser.ProfileLinkColor),
			}
			if twitterUser.Entities.URL.Urls != nil && len(twitterUser.Entities.URL.Urls) >= 1 {
				accountEmbed.Fields = append(accountEmbed.Fields, &discordgo.MessageEmbedField{
					Name:   "Website",
					Value:  fmt.Sprintf("[%s](%s)", twitterUser.Entities.URL.Urls[0].ExpandedURL, twitterUser.Entities.URL.Urls[0].ExpandedURL),
					Inline: true,
				})
			}
			_, err = helpers.SendComplex(msg.ChannelID,
				&discordgo.MessageSend{
					Content: fmt.Sprintf("<%s>", fmt.Sprintf(TwitterFriendlyUser, twitterUser.ScreenName)),
					Embed:   accountEmbed,
				})
			helpers.Relax(err)
			return
		}
	} else {
		helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
	}
}

func (m *Twitter) postTweetToChannel(channelID string, tweet *twitter.Tweet, entry models.TwitterEntry) {
	if entry.PostMode == models.TwitterPostModeDiscordEmbed || entry.PostMode == models.TwitterPostModeText {
		content := fmt.Sprintf("%s", fmt.Sprintf(TwitterFriendlyStatus, tweet.User.ScreenName, tweet.IDStr))
		if entry.PostMode == models.TwitterPostModeText {
			content = "<" + content + ">"
		}
		if entry.MentionRoleID != "" {
			content = fmt.Sprintf("<@&%s>\n%s", entry.MentionRoleID, content)
		}
		if entry.PostMode == models.TwitterPostModeText {
			// hide URL previews
			content += "\n" + helpers.URLRegex.ReplaceAllStringFunc(tweet.Text, func(link string) string {
				return "<" + link + ">"
			})
			if tweet.ExtendedEntities != nil && len(tweet.ExtendedEntities.Media) > 0 {
				content += "\n"
				for _, mediaUrl := range tweet.ExtendedEntities.Media {
					switch mediaUrl.Type {
					case "video", "animated_gif":
						if len(mediaUrl.VideoInfo.Variants) > 0 && m.bestVideoVariant(mediaUrl.VideoInfo.Variants).URL != "" {
							content += m.bestVideoVariant(mediaUrl.VideoInfo.Variants).URL + "\n"
						} else {
							content += m.maxQualityMediaUrl(mediaUrl.DisplayURL) + "\n"
						}
					default:
						content += m.maxQualityMediaUrl(mediaUrl.MediaURLHttps) + "\n"
					}
				}
			}
		}

		helpers.SendComplex(
			channelID, &discordgo.MessageSend{
				Content: content,
			})
		return
	}

	twitterNameModifier := ""
	if tweet.User.Verified {
		twitterNameModifier += " ☑"
	}
	if tweet.User.Protected {
		twitterNameModifier += " 🔒"
	}

	mediaModifier := ""

	tweetText := m.escapeTwitterContent(html.UnescapeString(tweet.Text))

	if tweet.ExtendedEntities != nil {
		if len(tweet.ExtendedEntities.Media) > 0 {
			if tweet.ExtendedEntities.Media[0].Type == "video" {
				mediaModifier += " (video)"
			}
		}
	}
	if tweet.Entities != nil {
		if len(tweet.Entities.Urls) > 0 {
			for _, urlEntity := range tweet.Entities.Urls {
				if len(urlEntity.ExpandedURL) <= 100 {
					tweetText = strings.Replace(tweetText, urlEntity.URL, urlEntity.ExpandedURL, -1)
				}
			}
		}
	}

	channelEmbed := &discordgo.MessageEmbed{
		Title: helpers.GetText("plugins.twitter.tweet-embed-title") + mediaModifier,
		URL:   fmt.Sprintf(TwitterFriendlyStatus, tweet.User.ScreenName, tweet.IDStr),
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.twitter.embed-footer"),
			IconURL: helpers.GetText("plugins.twitter.embed-footer-imageurl"),
		},
		Description: tweetText,
		Color:       helpers.GetDiscordColorFromHex(tweet.User.ProfileLinkColor),
		Author: &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("%s (@%s)%s", tweet.User.Name, tweet.User.ScreenName, twitterNameModifier),
			URL:     fmt.Sprintf(TwitterFriendlyUser, tweet.User.ScreenName),
			IconURL: tweet.User.ProfileImageURLHttps,
		},
	}

	if tweet.Entities != nil && len(tweet.Entities.Media) > 0 {
		channelEmbed.Image = &discordgo.MessageEmbedImage{URL: tweet.Entities.Media[0].MediaURLHttps}
	}

	if tweet.ExtendedEntities != nil && len(tweet.ExtendedEntities.Media) > 0 {
		channelEmbed.Description += "\n\n`Links:` "
		for i, mediaUrl := range tweet.ExtendedEntities.Media {
			switch mediaUrl.Type {
			case "video", "animated_gif":
				if len(mediaUrl.VideoInfo.Variants) > 0 && m.bestVideoVariant(mediaUrl.VideoInfo.Variants).URL != "" {
					channelEmbed.Description += fmt.Sprintf("[%s](%s) ", emojis.From(strconv.Itoa(i+1)), m.escapeTwitterContent(m.bestVideoVariant(mediaUrl.VideoInfo.Variants).URL))
				} else {
					channelEmbed.Description += fmt.Sprintf("[%s](%s) ", emojis.FromToText(strconv.Itoa(i+1)), m.escapeTwitterContent(m.maxQualityMediaUrl(mediaUrl.DisplayURL)))
				}
				break
			default:
				channelEmbed.Description += fmt.Sprintf("[%s](%s) ", emojis.FromToText(strconv.Itoa(i+1)), m.escapeTwitterContent(m.maxQualityMediaUrl(mediaUrl.MediaURLHttps)))
				break
			}
		}
	}

	content := fmt.Sprintf("<%s>", fmt.Sprintf(TwitterFriendlyStatus, tweet.User.ScreenName, tweet.IDStr))
	if entry.MentionRoleID != "" {
		content = fmt.Sprintf("<@&%s>\n%s", entry.MentionRoleID, content)
	}

	helpers.SendComplex(
		channelID, &discordgo.MessageSend{
			Content: content,
			Embed:   channelEmbed,
		})
}

func (m *Twitter) postAnacondaTweetToChannel(channelID string, tweet *anaconda.Tweet, twitterUser *anaconda.User, entry models.TwitterEntry) {
	if entry.PostMode == models.TwitterPostModeDiscordEmbed || entry.PostMode == models.TwitterPostModeText {
		content := fmt.Sprintf("%s", fmt.Sprintf(TwitterFriendlyStatus, twitterUser.ScreenName, tweet.IdStr))
		if entry.PostMode == models.TwitterPostModeText {
			content = "<" + content + ">"
		}
		if entry.MentionRoleID != "" {
			content = fmt.Sprintf("<@&%s>\n%s", entry.MentionRoleID, content)
		}
		if entry.PostMode == models.TwitterPostModeText {
			// hide URL previews
			content += "\n" + helpers.URLRegex.ReplaceAllStringFunc(tweet.Text, func(link string) string {
				return "<" + link + ">"
			})
			// link medias
			if len(tweet.ExtendedEntities.Media) > 0 {
				content += "\n"
				for _, mediaUrl := range tweet.ExtendedEntities.Media {
					switch mediaUrl.Type {
					case "video", "animated_gif":
						if len(mediaUrl.VideoInfo.Variants) > 0 && m.bestAnacondaVideoVariant(mediaUrl.VideoInfo.Variants).Url != "" {
							content += m.bestAnacondaVideoVariant(mediaUrl.VideoInfo.Variants).Url + "\n"
						} else {
							content += m.maxQualityMediaUrl(mediaUrl.Display_url) + "\n"
						}
					default:
						content += m.maxQualityMediaUrl(mediaUrl.Media_url_https) + "\n"
					}
				}
			}
		}

		helpers.SendComplex(
			channelID, &discordgo.MessageSend{
				Content: content,
			})
		return
	}

	twitterNameModifier := ""
	if twitterUser.Verified {
		twitterNameModifier += " ☑"
	}
	if twitterUser.Protected {
		twitterNameModifier += " 🔒"
	}

	mediaModifier := ""

	tweetText := m.escapeTwitterContent(html.UnescapeString(tweet.Text))

	if len(tweet.ExtendedEntities.Media) > 0 {
		if tweet.ExtendedEntities.Media[0].Type == "video" {
			mediaModifier += " (video)"
		}
	}
	if len(tweet.Entities.Urls) > 0 {
		for _, urlEntity := range tweet.Entities.Urls {
			if len(urlEntity.Expanded_url) <= 100 {
				tweetText = strings.Replace(tweetText, urlEntity.Url, urlEntity.Expanded_url, -1)
			}
		}
	}

	channelEmbed := &discordgo.MessageEmbed{
		Title: helpers.GetText("plugins.twitter.tweet-embed-title") + mediaModifier,
		URL:   fmt.Sprintf(TwitterFriendlyStatus, twitterUser.ScreenName, tweet.IdStr),
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.twitter.embed-footer"),
			IconURL: helpers.GetText("plugins.twitter.embed-footer-imageurl"),
		},
		Description: tweetText,
		Color:       helpers.GetDiscordColorFromHex(twitterUser.ProfileLinkColor),
		Author: &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("%s (@%s)%s", twitterUser.Name, twitterUser.ScreenName, twitterNameModifier),
			URL:     fmt.Sprintf(TwitterFriendlyUser, twitterUser.ScreenName),
			IconURL: twitterUser.ProfileImageUrlHttps,
		},
	}

	if len(tweet.Entities.Media) > 0 {
		channelEmbed.Image = &discordgo.MessageEmbedImage{URL: tweet.Entities.Media[0].Media_url_https}
	}

	if len(tweet.ExtendedEntities.Media) > 0 {
		channelEmbed.Description += "\n\n`Links:` "
		for i, mediaUrl := range tweet.ExtendedEntities.Media {
			switch mediaUrl.Type {
			case "video", "animated_gif":
				if len(mediaUrl.VideoInfo.Variants) > 0 && m.bestAnacondaVideoVariant(mediaUrl.VideoInfo.Variants).Url != "" {
					channelEmbed.Description += fmt.Sprintf("[%s](%s) ", emojis.FromToText(strconv.Itoa(i+1)), m.escapeTwitterContent(m.bestAnacondaVideoVariant(mediaUrl.VideoInfo.Variants).Url))
				} else {
					channelEmbed.Description += fmt.Sprintf("[%s](%s) ", emojis.FromToText(strconv.Itoa(i+1)), m.escapeTwitterContent(m.maxQualityMediaUrl(mediaUrl.Display_url)))
				}
				break
			default:
				channelEmbed.Description += fmt.Sprintf("[%s](%s) ", emojis.FromToText(strconv.Itoa(i+1)), m.escapeTwitterContent(m.maxQualityMediaUrl(mediaUrl.Media_url_https)))
				break
			}
		}
	}

	content := fmt.Sprintf("<%s>", fmt.Sprintf(TwitterFriendlyStatus, twitterUser.ScreenName, tweet.IdStr))
	if entry.MentionRoleID != "" {
		content = fmt.Sprintf("<@&%s>\n%s", entry.MentionRoleID, content)
	}

	helpers.SendComplex(
		channelID, &discordgo.MessageSend{
			Content: content,
			Embed:   channelEmbed,
		})
}

func (m *Twitter) bestVideoVariant(videoVariants []twitter.VideoVariant) (bestVariant twitter.VideoVariant) {
	for _, videoVariant := range videoVariants {
		if videoVariant.ContentType == "application/x-mpegURL" {
			continue
		}
		if videoVariant.Bitrate >= bestVariant.Bitrate {
			bestVariant = videoVariant
		}
	}
	return bestVariant
}

func (m *Twitter) bestAnacondaVideoVariant(videoVariants []anaconda.Variant) (bestVariant anaconda.Variant) {
	for _, videoVariant := range videoVariants {
		if videoVariant.ContentType == "application/x-mpegURL" {
			continue
		}
		if videoVariant.Bitrate >= bestVariant.Bitrate {
			bestVariant = videoVariant
		}
	}
	return bestVariant
}

func (t *Twitter) escapeTwitterContent(input string) (result string) {
	result = strings.Replace(input, "_", "\\_", -1)
	result = strings.Replace(result, "*", "\\*", -1)
	result = strings.Replace(result, "~", "\\~", -1)
	return result
}

func (t *Twitter) maxQualityMediaUrl(input string) (result string) {
	if strings.HasSuffix(input, ".jpg") || strings.HasSuffix(input, ".png") {
		return input + ":orig"
	}
	return input
}

func (m *Twitter) handleError(err error) string {
	// Extract 'error code' from dghubble/go-twitter's err message.
	// Has a dependency with dghubble/go-twitter package.
	var errCode int
	var errMsg string
	_, scanErr := fmt.Sscanf(err.Error(), "twitter: %d %s", &errCode, &errMsg)
	if scanErr != nil {
		return helpers.GetTextF("plugins.twitter.rate-limit-exceed")
	}

	// Handle twitter API error by code
	switch errCode {
	case 50:
		return helpers.GetTextF("plugins.twitter.account-not-found")
	case 63:
		return helpers.GetTextF("plugins.twitter.account-has-been-suspended")
	case 88:
		return helpers.GetTextF("plugins.twitter.rate-limit-exceed")
	case 130:
		return helpers.GetTextF("plugins.twitter.over-capacity")
	case 131:
		return helpers.GetTextF("plugins.twitter.internal-error")
	default:
		helpers.Relax(err)
	}

	// Unreachable
	err = errors.Wrap(err, "reached to unreachable code")
	panic(err)
}

func (m *Twitter) lockEntry(entryID bson.ObjectId) {
	twitterEntryLock.Lock()
	defer twitterEntryLock.Unlock()

	key := string(entryID)
	if _, ok := twitterEntryLocks[key]; ok && twitterEntryLocks[key] != nil {
		twitterEntryLocks[key].Lock()
		return
	}
	twitterEntryLocks[key] = new(sync.Mutex)
	twitterEntryLocks[key].Lock()
}

func (m *Twitter) unlockEntry(entryID bson.ObjectId) {
	twitterEntryLock.Lock()
	defer twitterEntryLock.Unlock()

	key := string(entryID)
	if _, ok := twitterEntryLocks[key]; ok && twitterEntryLocks[key] != nil {
		twitterEntryLocks[key].Unlock()
	}
}

func (t *Twitter) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {

}

func (t *Twitter) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {

}

func (t *Twitter) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {

}

func (t *Twitter) OnMessageDelete(msg *discordgo.MessageDelete, session *discordgo.Session) {

}

func (t *Twitter) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {

}

func (t *Twitter) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {

}

func (t *Twitter) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {

}

func (t *Twitter) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}
