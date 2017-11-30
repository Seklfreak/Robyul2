package plugins

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"html"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/bwmarrin/discordgo"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"github.com/dustin/go-humanize"
	rethink "github.com/gorethink/gorethink"
	"github.com/pkg/errors"
)

type Twitter struct{}

type DB_Twitter_Entry struct {
	ID                string             `gorethink:"id,omitempty"`
	ServerID          string             `gorethink:"serverid"`
	ChannelID         string             `gorethink:"channelid"`
	AccountScreenName string             `gorethink:"account_screen_name"`
	PostedTweets      []DB_Twitter_Tweet `gorethink:"posted_tweets"`
	AccountID         string             `gorethink:"account_id"`
}

type DB_Twitter_Tweet struct {
	ID        string `gorethink:"id,omitempty"`
	CreatedAt string `gorethink:"createdat"`
}

type Twitter_Safe_Entries struct {
	entries []DB_Twitter_Entry
	mux     sync.Mutex
}

var (
	twitterClient            *twitter.Client
	twitterStream            *twitter.Stream
	twitterDemux             twitter.SwitchDemux
	twitterStreamNeedsUpdate bool
	twitterEntriesCache      []DB_Twitter_Entry
	twitterStreamIsStarting  sync.Mutex
)

const (
	TwitterFriendlyUser   = "https://twitter.com/%s"
	TwitterFriendlyStatus = "https://twitter.com/%s/status/%s"
	rfc2822               = "Mon Jan 02 15:04:05 -0700 2006"
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

	twitterDemux = twitter.NewSwitchDemux()
	twitterDemux.Tweet = func(tweet *twitter.Tweet) {
		//fmt.Println("received tweet:", tweet.Text, "by:", tweet.User.ScreenName)
		for _, entry := range twitterEntriesCache {
			if entry.AccountID != tweet.User.IDStr {
				continue
			}

			entry := t.getEntryBy("id", entry.ID)

			changes := false
			tweetAlreadyPosted := false

			for _, postedTweet := range entry.PostedTweets {
				if postedTweet.ID == tweet.IDStr {
					tweetAlreadyPosted = true
				}
			}
			if tweetAlreadyPosted == false {
				cache.GetLogger().WithField("module", "twitter").Info(fmt.Sprintf("posting tweet (via streaming): #%s", tweet.IDStr))
				entry.PostedTweets = append(entry.PostedTweets, DB_Twitter_Tweet{ID: tweet.IDStr, CreatedAt: tweet.CreatedAt})
				changes = true
				go t.postTweetToChannel(entry.ChannelID, tweet, tweet.User)
			}

			if changes == true {
				t.setEntry(entry)
			}
		}
	}

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

	cursor, err := rethink.Table("twitter").Run(helpers.GetDB())
	helpers.Relax(err)
	err = cursor.All(&twitterEntriesCache)
	helpers.Relax(err)

	for _, entry := range twitterEntriesCache {
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
					t.setEntry(entry)
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

	twitterStream, err = twitterClient.Streams.Filter(&twitter.StreamFilterParams{
		Follow:        accountIDs,
		StallWarnings: twitter.Bool(true),
	})
	helpers.Relax(err)
	go twitterDemux.HandleChan(twitterStream.Messages)
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

		time.Sleep(30 * time.Second)
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

	var bundledEntries map[string][]DB_Twitter_Entry

	for {
		bundledEntries = make(map[string][]DB_Twitter_Entry, 0)

		for _, entry := range twitterEntriesCache {
			channel, err := helpers.GetChannelWithoutApi(entry.ChannelID)
			if err != nil || channel == nil || channel.ID == "" {
				//cache.GetLogger().WithField("module", "twitter").Warn(fmt.Sprintf("skipped twitter @%s for Channel #%s on Guild #%s: channel not found!",
				//	entry.AccountScreenName, entry.ChannelID, entry.ServerID))
				continue
			}

			if _, ok := bundledEntries[entry.AccountScreenName]; ok {
				bundledEntries[entry.AccountScreenName] = append(bundledEntries[entry.AccountScreenName], entry)
			} else {
				bundledEntries[entry.AccountScreenName] = []DB_Twitter_Entry{entry}
			}
		}

		cache.GetLogger().WithField("module", "twitter").Infof("checking %d accounts for %d feeds", len(bundledEntries), len(twitterEntriesCache))
		start := time.Now()

		for twitterAccoutnScreenName, entries := range bundledEntries {
			// cache.GetLogger().WithField("module", "twitter").Info(fmt.Sprintf("checking Twitter Account @%s", twitterAccoutnScreenName))

			twitterUser, _, err := twitterClient.Users.Show(&twitter.UserShowParams{
				ScreenName: twitterAccoutnScreenName,
			})
			if err != nil {
				cache.GetLogger().WithField("module", "twitter").Error(fmt.Sprintf("updating twitter account @%s failed: %s", twitterAccoutnScreenName, err.Error()))
				continue
			}

			twitterUserTweets, _, err := twitterClient.Timelines.UserTimeline(&twitter.UserTimelineParams{
				ScreenName:      twitterAccoutnScreenName,
				Count:           10,
				ExcludeReplies:  twitter.Bool(true),
				IncludeRetweets: twitter.Bool(true),
			})
			if err != nil {
				cache.GetLogger().WithField("module", "twitter").Error(fmt.Sprintf("getting tweets of @%s failed: %s", twitterAccoutnScreenName, err.Error()))
				continue
			}

			// https://github.com/golang/go/wiki/SliceTricks#reversing
			for i := len(twitterUserTweets)/2 - 1; i >= 0; i-- {
				opp := len(twitterUserTweets) - 1 - i
				twitterUserTweets[i], twitterUserTweets[opp] = twitterUserTweets[opp], twitterUserTweets[i]
			}

			for _, entry := range entries {
				entry := m.getEntryBy("id", entry.ID)

				changes := false

				for _, tweet := range twitterUserTweets {
					tweetAlreadyPosted := false
					for _, postedTweet := range entry.PostedTweets {
						if postedTweet.ID == tweet.IDStr {
							tweetAlreadyPosted = true
						}
					}
					if tweetAlreadyPosted == false {
						cache.GetLogger().WithField("module", "twitter").Info(fmt.Sprintf("posting tweet (via REST): #%s", tweet.IDStr))
						entry.PostedTweets = append(entry.PostedTweets, DB_Twitter_Tweet{ID: tweet.IDStr, CreatedAt: tweet.CreatedAt})
						changes = true
						tweetToPost := tweet
						go m.postTweetToChannel(entry.ChannelID, &tweetToPost, twitterUser)
					}

				}
				if changes == true {
					m.setEntry(entry)
				}
			}
		}

		elapsed := time.Since(start)
		cache.GetLogger().WithField("module", "twitter").Infof("checked %d accounts for %d feeds, took %s", len(bundledEntries), len(twitterEntriesCache), elapsed)
		metrics.TwitterRefreshTime.Set(elapsed.Seconds())

		time.Sleep(10 * time.Minute)
	}
}

func (m *Twitter) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
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
					targetChannel, err = helpers.GetChannelFromMention(msg, args[len(args)-1])
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
				twitterUsername := strings.Replace(args[1], "@", "", 1)
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
				// Create DB Entries
				var dbTweets []DB_Twitter_Tweet
				for _, tweet := range twitterUserTweets {
					tweetEntry := DB_Twitter_Tweet{ID: tweet.IDStr, CreatedAt: tweet.CreatedAt}
					dbTweets = append(dbTweets, tweetEntry)

				}
				// create new entry in db
				entry := m.getEntryByOrCreateEmpty("id", "")
				entry.ServerID = targetChannel.GuildID
				entry.ChannelID = targetChannel.ID
				entry.AccountScreenName = twitterUser.ScreenName
				entry.PostedTweets = dbTweets
				entry.AccountID = twitterUser.IDStr
				m.setEntry(entry)

				twitterStreamNeedsUpdate = true

				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.twitter.account-added-success", entry.AccountScreenName, entry.ChannelID))
				cache.GetLogger().WithField("module", "twitter").Info(fmt.Sprintf("Added Twitter Account @%s to Channel %s (#%s) on Guild %s (#%s)", entry.AccountScreenName, targetChannel.Name, entry.ChannelID, targetGuild.Name, targetGuild.ID))
			})
		case "delete", "del", "remove": // [p]twitter delete <id>
			helpers.RequireMod(msg, func() {
				if len(args) >= 2 {
					session.ChannelTyping(msg.ChannelID)
					entryId := args[1]
					entryBucket := m.getEntryBy("id", entryId)
					if entryBucket.ID != "" {
						m.deleteEntryById(entryBucket.ID)

						twitterStreamNeedsUpdate = true

						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.twitter.account-delete-success", entryBucket.AccountScreenName))
						cache.GetLogger().WithField("module", "twitter").Info(fmt.Sprintf("Deleted Twitter Account @%s", entryBucket.AccountScreenName))
					} else {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.twitter.account-delete-not-found-error"))
						return
					}
				} else {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					return
				}
			})
		case "list": // [p]twitter list
			currentChannel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)
			var entryBucket []DB_Twitter_Entry
			listCursor, err := rethink.Table("twitter").Filter(
				rethink.Row.Field("serverid").Eq(currentChannel.GuildID),
			).Run(helpers.GetDB())
			helpers.Relax(err)
			defer listCursor.Close()
			err = listCursor.All(&entryBucket)

			if err == rethink.ErrEmptyResult || len(entryBucket) <= 0 {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.twitter.account-list-no-accounts-error"))
				return
			} else if err != nil {
				helpers.Relax(err)
			}

			resultMessage := ""
			for _, entry := range entryBucket {
				resultMessage += fmt.Sprintf("`%s`: Twitter Account `@%s` posting to <#%s>\n", entry.ID, entry.AccountScreenName, entry.ChannelID)
			}
			resultMessage += fmt.Sprintf("Found **%d** Twitter Accounts in total.", len(entryBucket))
			for _, resultPage := range helpers.Pagify(resultMessage, "\n") {
				_, err := helpers.SendMessage(msg.ChannelID, resultPage)
				helpers.Relax(err)
			}
		default:
			session.ChannelTyping(msg.ChannelID)
			twitterUsername := strings.Replace(args[0], "@", "", 1)
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

func (m *Twitter) postTweetToChannel(channelID string, tweet *twitter.Tweet, twitterUser *twitter.User) {
	twitterNameModifier := ""
	if twitterUser.Verified {
		twitterNameModifier += " ☑"
	}
	if twitterUser.Protected {
		twitterNameModifier += " 🔒"
	}

	mediaModifier := ""
	// Not always accurate
	//if tweet.Entities != nil && len(tweet.Entities.Media) > 0 {
	//	mediaModifier += fmt.Sprintf(" (%d image", len(tweet.Entities.Media))
	//	if len(tweet.Entities.Media) > 1 {
	//		mediaModifier += "s"
	//	}
	//	mediaModifier += ")"
	//}
	tweetText := html.UnescapeString(tweet.Text)

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
		URL:   fmt.Sprintf(TwitterFriendlyStatus, twitterUser.ScreenName, tweet.IDStr),
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.twitter.embed-footer"),
			IconURL: helpers.GetText("plugins.twitter.embed-footer-imageurl"),
		},
		Description: tweetText,
		Color:       helpers.GetDiscordColorFromHex(twitterUser.ProfileLinkColor),
		Author: &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("%s (@%s)%s", twitterUser.Name, twitterUser.ScreenName, twitterNameModifier),
			URL:     fmt.Sprintf(TwitterFriendlyUser, twitterUser.ScreenName),
			IconURL: twitterUser.ProfileImageURLHttps,
		},
	}

	if tweet.Entities != nil && len(tweet.Entities.Media) > 0 {
		channelEmbed.Image = &discordgo.MessageEmbedImage{URL: tweet.Entities.Media[0].MediaURLHttps}
	}

	_, err := helpers.SendComplex(
		channelID, &discordgo.MessageSend{
			Content: fmt.Sprintf("<%s>", fmt.Sprintf(TwitterFriendlyStatus, twitterUser.ScreenName, tweet.IDStr)),
			Embed:   channelEmbed,
		})
	if err != nil {
		cache.GetLogger().WithField("module", "twitter").Info(fmt.Sprintf("posting tweet: #%d to channel: #%s failed: %s", tweet.ID, channelID, err))
	}
}

func (m *Twitter) getEntryBy(key string, id string) DB_Twitter_Entry {
	var entryBucket DB_Twitter_Entry
	listCursor, err := rethink.Table("twitter").Filter(
		rethink.Row.Field(key).Eq(id),
	).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer listCursor.Close()
	err = listCursor.One(&entryBucket)

	if err == rethink.ErrEmptyResult {
		return entryBucket
	} else if err != nil {
		panic(err)
	}

	return entryBucket
}

func (m *Twitter) getEntryByOrCreateEmpty(key string, id string) DB_Twitter_Entry {
	var entryBucket DB_Twitter_Entry
	listCursor, err := rethink.Table("twitter").Filter(
		rethink.Row.Field(key).Eq(id),
	).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer listCursor.Close()
	err = listCursor.One(&entryBucket)

	// If user has no DB entries create an empty document
	if err == rethink.ErrEmptyResult {
		insert := rethink.Table("twitter").Insert(DB_Twitter_Entry{})
		res, e := insert.RunWrite(helpers.GetDB())
		// If the creation was successful read the document
		if e != nil {
			panic(e)
		} else {
			return m.getEntryByOrCreateEmpty("id", res.GeneratedKeys[0])
		}
	} else if err != nil {
		panic(err)
	}

	return entryBucket
}

func (m *Twitter) setEntry(entry DB_Twitter_Entry) {
	if entry.ID != "" {
		_, err := rethink.Table("twitter").Update(entry).Run(helpers.GetDB())
		helpers.Relax(err)
	}
}

func (m *Twitter) deleteEntryById(id string) {
	if id != "" {
		_, err := rethink.Table("twitter").Filter(
			rethink.Row.Field("id").Eq(id),
		).Delete().RunWrite(helpers.GetDB())
		helpers.Relax(err)
	}
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
