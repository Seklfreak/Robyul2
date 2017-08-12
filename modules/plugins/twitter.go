package plugins

import (
    "fmt"
    "github.com/Seklfreak/Robyul2/cache"
    "github.com/Seklfreak/Robyul2/helpers"
    "github.com/bwmarrin/discordgo"
    "github.com/dghubble/go-twitter/twitter"
    "github.com/dghubble/oauth1"
    "github.com/dustin/go-humanize"
    rethink "github.com/gorethink/gorethink"
    "strings"
    "time"
    "sync"
)

type Twitter struct{}

type DB_Twitter_Entry struct {
    ID                string             `gorethink:"id,omitempty"`
    ServerID          string             `gorethink:"serverid"`
    ChannelID         string             `gorethink:"channelid"`
    AccountScreenName string             `gorethink:"account_screen_name"`
    PostedTweets      []DB_Twitter_Tweet `gorethink:"posted_tweets"`
}

type DB_Twitter_Tweet struct {
    ID        string `gorethink:"id,omitempty"`
    CreatedAt string `gorethink:"createdat`
}

type Twitter_Safe_Entries struct {
    entries []DB_Twitter_Entry
    mux     sync.Mutex
}

var (
    twitterClient *twitter.Client
)

const (
    TwitterFriendlyUser   string = "https://twitter.com/%s"
    TwitterFriendlyStatus string = "https://twitter.com/%s/status/%s"
    TwitterFooterIconUrl  string = "https://abs.twimg.com/favicons/favicon.ico"
    rfc2822               string = "Mon Jan 02 15:04:05 -0700 2006"
)

func (m *Twitter) Commands() []string {
    return []string{
        "twitter",
    }
}

func (m *Twitter) Init(session *discordgo.Session) {
    config := oauth1.NewConfig(
        helpers.GetConfig().Path("twitter.consumer_key").Data().(string),
        helpers.GetConfig().Path("twitter.consumer_secret").Data().(string))
    token := oauth1.NewToken(
        helpers.GetConfig().Path("twitter.access_token").Data().(string),
        helpers.GetConfig().Path("twitter.access_secret").Data().(string))
    httpClient := config.Client(oauth1.NoContext, token)
    twitterClient = twitter.NewClient(httpClient)

    go m.checkTwitterFeedsLoop()
    cache.GetLogger().WithField("module", "twitter").Info("Started Twitter loop (10m)")
}
func (m *Twitter) checkTwitterFeedsLoop() {
    var safeEntries Twitter_Safe_Entries

    defer func() {
        helpers.Recover()

        cache.GetLogger().WithField("module", "twitter").Error("The checkTwitterFeedsLoop died. Please investigate! Will be restarted in 60 seconds")
        time.Sleep(60 * time.Second)
        m.checkTwitterFeedsLoop()
    }()

    for {
        cursor, err := rethink.Table("twitter").Run(helpers.GetDB())
        helpers.Relax(err)

        err = cursor.All(&safeEntries.entries)
        helpers.Relax(err)

        // TODO: Check multiple entries at once
        for _, entry := range safeEntries.entries {
            safeEntries.mux.Lock()
            changes := false
            cache.GetLogger().WithField("module", "twitter").Info(fmt.Sprintf("checking Twitter Account @%s", entry.AccountScreenName))

            twitterUser, _, err := twitterClient.Users.Show(&twitter.UserShowParams{
                ScreenName: entry.AccountScreenName,
            })
            if err != nil {
                cache.GetLogger().WithField("module", "twitter").Error(fmt.Sprintf("updating twitter account @%s failed: %s", entry.AccountScreenName, err.Error()))
                safeEntries.mux.Unlock()
                continue
            }

            twitterUserTweets, _, err := twitterClient.Timelines.UserTimeline(&twitter.UserTimelineParams{
                ScreenName:      entry.AccountScreenName,
                Count:           10,
                ExcludeReplies:  twitter.Bool(true),
                IncludeRetweets: twitter.Bool(true),
            })
            if err != nil {
                cache.GetLogger().WithField("module", "twitter").Error(fmt.Sprintf("getting tweets of @%s failed: %s", entry.AccountScreenName, err.Error()))
                safeEntries.mux.Unlock()
                continue
            }

            // https://github.com/golang/go/wiki/SliceTricks#reversing
            for i := len(twitterUserTweets)/2 - 1; i >= 0; i-- {
                opp := len(twitterUserTweets) - 1 - i
                twitterUserTweets[i], twitterUserTweets[opp] = twitterUserTweets[opp], twitterUserTweets[i]
            }

            for _, tweet := range twitterUserTweets {
                tweetAlreadyPosted := false
                for _, postedTweet := range entry.PostedTweets {
                    if postedTweet.ID == tweet.IDStr {
                        tweetAlreadyPosted = true
                    }
                }
                if tweetAlreadyPosted == false {
                    cache.GetLogger().WithField("module", "twitter").Info(fmt.Sprintf("Posting Tweet: #%s", tweet.IDStr))
                    entry.PostedTweets = append(entry.PostedTweets, DB_Twitter_Tweet{ID: tweet.IDStr, CreatedAt: tweet.CreatedAt})
                    changes = true
                    go m.postTweetToChannel(entry.ChannelID, tweet, twitterUser)
                }

            }
            if changes == true {
                m.setEntry(entry)
            }
            safeEntries.mux.Unlock()
        }

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
                        session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
                        return
                    }
                } else {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
                    return
                }
                targetGuild, err = helpers.GetGuild(targetChannel.GuildID)
                helpers.Relax(err)
                // get twitter account and tweets
                twitterUsername := strings.Replace(args[1], "@", "", 1)
                twitterUser, _, err := twitterClient.Users.Show(&twitter.UserShowParams{
                    ScreenName: twitterUsername,
                })
                if err != nil && err.Error() == "twitter: 50 User not found." {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.twitter.account-not-found"))
                    return
                } else {
                    helpers.Relax(err)
                }

                twitterUserTweets, _, err := twitterClient.Timelines.UserTimeline(&twitter.UserTimelineParams{
                    ScreenName:      twitterUser.ScreenName,
                    Count:           10,
                    ExcludeReplies:  twitter.Bool(true),
                    IncludeRetweets: twitter.Bool(true),
                })
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
                m.setEntry(entry)

                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.twitter.account-added-success", entry.AccountScreenName, entry.ChannelID))
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

                        session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.twitter.account-delete-success", entryBucket.AccountScreenName))
                        cache.GetLogger().WithField("module", "twitter").Info(fmt.Sprintf("Deleted Twitter Account @%s", entryBucket.AccountScreenName))
                    } else {
                        session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.twitter.account-delete-not-found-error"))
                        return
                    }
                } else {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
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
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.twitter.account-list-no-accounts-error"))
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
                _, err := session.ChannelMessageSend(msg.ChannelID, resultPage)
                helpers.Relax(err)
            }
        default:
            session.ChannelTyping(msg.ChannelID)
            twitterUsername := strings.Replace(args[0], "@", "", 1)
            twitterUser, _, err := twitterClient.Users.Show(&twitter.UserShowParams{
                ScreenName: twitterUsername,
            })
            if err != nil && err.Error() == "twitter: 50 User not found." {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.twitter.account-not-found"))
                return
            } else {
                helpers.Relax(err)
            }

            createdAtTime, err := time.Parse(rfc2822, twitterUser.CreatedAt)
            helpers.Relax(err)

            twitterUserDescription := twitterUser.Description
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
                    IconURL: TwitterFooterIconUrl,
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
            _, err = session.ChannelMessageSendComplex(msg.ChannelID,
                &discordgo.MessageSend{
                    Content: fmt.Sprintf("<%s>", fmt.Sprintf(TwitterFriendlyUser, twitterUser.ScreenName)),
                    Embed: accountEmbed,
                })
            helpers.Relax(err)
            return
        }
    } else {
        session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
    }
}

func (m *Twitter) postTweetToChannel(channelID string, tweet twitter.Tweet, twitterUser *twitter.User) {
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
    tweetText := tweet.Text

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
            IconURL: TwitterFooterIconUrl,
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

    _, err := cache.GetSession().ChannelMessageSendComplex(
        channelID, &discordgo.MessageSend{
            Content: fmt.Sprintf("<%s>", fmt.Sprintf(TwitterFriendlyStatus, twitterUser.ScreenName, tweet.IDStr)),
            Embed: channelEmbed,
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
    _, err := rethink.Table("twitter").Update(entry).Run(helpers.GetDB())
    helpers.Relax(err)
}

func (m *Twitter) deleteEntryById(id string) {
    _, err := rethink.Table("twitter").Filter(
        rethink.Row.Field("id").Eq(id),
    ).Delete().RunWrite(helpers.GetDB())
    helpers.Relax(err)
}
