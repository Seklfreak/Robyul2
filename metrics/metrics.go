package metrics

import (
	"expvar"
	"net/http"
	"runtime"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"gopkg.in/mgo.v2/bson"
)

var (
	// MessagesReceived counts all ever received messages
	MessagesReceived = expvar.NewInt("messages_received")

	// MessagesSent counts all ever sent messages
	MessagesSent = expvar.NewInt("messages_sent")

	// ChatbotRequests counts all chatbot requests made
	ChatbotRequests = expvar.NewInt("chatbot_requests")

	// UserCount counts all logged-in users
	UserCount = expvar.NewInt("user_count")

	// ChannelCount counts all watching channels
	ChannelCount = expvar.NewInt("channel_count")

	// GuildCount counts all joined guilds
	GuildCount = expvar.NewInt("guild_count")

	// CommandsExecuted increases after each command execution
	CommandsExecuted = expvar.NewInt("commands_executed")

	// CoroutineCount counts all running coroutines
	CoroutineCount = expvar.NewInt("coroutine_count")

	// Uptime stores the timestamp of the bot's boot
	Uptime = expvar.NewInt("uptime")

	// VliveChannelsCount counts all connected vlive channels
	VliveChannelsCount = expvar.NewInt("vlive_channels_count")

	// VLiveRequests increases after each request to vlive.tv
	VLiveRequests = expvar.NewInt("vlive_requests")

	// VliveRefreshTime is the latest refresh time
	VliveRefreshTime = expvar.NewFloat("vlive_refresh_time")

	// TwitterAccountsCount counts all connected twitter accounts
	TwitterAccountsCount = expvar.NewInt("twitter_accounts_count")

	// TwitterRefreshTime is the latest refresh time
	TwitterRefreshTime = expvar.NewFloat("twitter_refresh_time")

	// InstagramAccountsCount counts all connected instagram accounts
	InstagramAccountsCount = expvar.NewInt("instagram_accounts_count")

	// InstagramRefreshTime is the latest Feeds and Story refresh time
	InstagramRefreshTime = expvar.NewFloat("instagram_refresh_time")

	// InstagramGraphQlRefreshTime is the latest GraphQL feed refresh time
	InstagramGraphQlFeedRefreshTime = expvar.NewFloat("instagram_graphql_feed_refresh_time")

	// FacebookPagesCount counts all connected instagram accounts
	FacebookPagesCount = expvar.NewInt("facebook_pages_count")

	// RedditSubredditsCount counts all connected subreddits accounts
	RedditSubredditsCount = expvar.NewInt("reddit_subreddits_count")

	// WolframAlphaRequests increases after each request to wolframalpha.com
	WolframAlphaRequests = expvar.NewInt("wolframalpha_requests")

	// LastFmRequests increases after each request to last.fm
	LastFmRequests = expvar.NewInt("lastfm_requests")

	// DarkSkyRequests increases after each request to darksky.net
	DarkSkyRequests = expvar.NewInt("darksky_requests")

	// KeywordNotificationsSentCount increased after every keyword notification sent
	KeywordNotificationsSentCount = expvar.NewInt("keywordnotifications_sent_count")

	// GalleriesCount counts all galleries in the db
	GalleriesCount = expvar.NewInt("galleries_count")

	// GalleryPostsSent increased with every link reposted
	GalleryPostsSent = expvar.NewInt("gallery_posts_sent")

	// GalleriesCount counts all galleries in the db
	MirrorsCount = expvar.NewInt("mirrors_count")

	// GalleryPostsSent increased with every link reposted
	MirrorsPostsSent = expvar.NewInt("mirrors_posts_sent")

	// LevelImagesGeneratedCount increased with every level image generated
	LevelImagesGenerated = expvar.NewInt("levels_images_generated")

	// RandomPictureSourcesCount counts all randompicture sources connected
	RandomPictureSourcesCount = expvar.NewInt("randompictures_sources_count")

	// RandomPictureSourcesImagesCachedCount counts all randompicture images in cache
	RandomPictureSourcesImagesCachedCount = expvar.NewInt("randompictures_sources_imagescached_count")

	// CustomCommandsCount counts all custom commands
	CustomCommandsCount = expvar.NewInt("customcommands_count")

	// CustomCommandsTriggered increased with every time a custom command is triggered
	CustomCommandsTriggered = expvar.NewInt("customcommands_triggered")

	// ReactionPollsCount increased with every time a new ReactionPoll is created
	ReactionPollsCount = expvar.NewInt("reactionpolls_count")

	// MachineryDelayedTasksCount counts all delayed machinery tasks
	MachineryDelayedTasksCount = expvar.NewInt("machinery_delayedtasks_count")

	// YoutubeChannelCount counts all connected youtube channels
	YoutubeChannelsCount = expvar.NewInt("youtube_channel_count")

	// YoutubeLeftQuota counts how many left youtube quotas
	YoutubeLeftQuota = expvar.NewInt("youtube_left_quota")

	// TwitchRefreshTime counts all connected twitch channels
	TwitchChannelsCount = expvar.NewInt("twitch_channels_count")

	// TwitchRefreshTime is the latest refresh time
	TwitchRefreshTime = expvar.NewFloat("twitch_refresh_time")

	// VanityInvitesCount counts all vanity invites channels
	VanityInvitesCount = expvar.NewInt("vanityinvites_count")

	// GimmeProxyCachedProxies counts all cached gimmeproxy proxies
	GimmeProxyCachedProxies = expvar.NewInt("gimmeproxy_cached_proxies")

	// EventlogAuditLogRequests counts all audit log requests made
	EventlogAuditLogRequests = expvar.NewInt("eventlog_audit_log_requests")

	// EventlogAuditLogBackfillTime is the latest AuditLog Backfill time
	EventlogAuditLogBackfillTime = expvar.NewFloat("eventlog_audit_log_backfill_time")

	// PerspectiveApiRequests counts all google perspective api requests made
	PerspectiveApiRequests = expvar.NewInt("perspective_api_requests")

	// CloudVisionApiRequests counts all google cloud vision api requests made
	CloudVisionApiRequests = expvar.NewInt("cloudvision_api_requests")

	// LevelsStackSize is the size of the exp processing stack
	LevelsStackSize = expvar.NewInt("levels_stack_size")

	// BiasgameImagesCount is the number of images in the biasgame
	BiasgameImagesCount = expvar.NewInt("biasgame_images_count")

	// BiasgameSuggestionsCount is the number of images in the biasgame
	BiasgameSuggestionsCount = expvar.NewInt("biasgame_suggestions_count")

	// BiasgameGamesCount is the number of games completed
	BiasgameGamesCount = expvar.NewInt("biasgame_games_count")

	// EventlogPendingAuditlogBackfills is the number of games completed
	EventlogPendingAuditlogBackfills = expvar.NewInt("eventlog_pending_auditlog_backfills")
)

// Init starts a http server on 127.0.0.1:1337
func Init() {
	cache.GetLogger().WithField("module", "metrics").Info("Listening on TCP/1337")
	Uptime.Set(time.Now().Unix())
	go http.ListenAndServe(helpers.GetConfig().Path("metrics_ip").Data().(string)+":1337", nil)
}

// OnReady listens for said discord event
func OnReady(session *discordgo.Session, event *discordgo.Ready) {
	go CollectDiscordMetrics(session)
	go CollectRuntimeMetrics()
}

// OnMessageCreate listens for said discord event
func OnMessageCreate(session *discordgo.Session, event *discordgo.MessageCreate) {
	MessagesReceived.Add(1)

	if event.Author.ID == session.State.User.ID {
		MessagesSent.Add(1)
	}
}

// CollectDiscordMetrics counts Guilds, Channels and Users
func CollectDiscordMetrics(session *discordgo.Session) {
	for {
		time.Sleep(15 * time.Second)

		users := make(map[string]string)
		channels := 0
		guilds := session.State.Guilds

		for _, guild := range guilds {
			channels += len(guild.Channels)
			for _, u := range guild.Members {
				users[u.User.ID] = u.User.Username
			}
		}

		UserCount.Set(int64(len(users)))
		ChannelCount.Set(int64(channels))
		GuildCount.Set(int64(len(guilds)))
	}
}

// CollectRuntimeMetrics counts all running coroutines
func CollectRuntimeMetrics() {
	defer helpers.Recover()

	for {
		time.Sleep(15 * time.Second)

		CoroutineCount.Set(int64(runtime.NumGoroutine()))

		VliveChannelsCount.Set(entriesCountMgo(models.VliveTable, nil))

		InstagramAccountsCount.Set(entriesCountMgo(models.InstagramTable, nil))

		TwitterAccountsCount.Set(entriesCountMgo(models.TwitterTable, nil))

		FacebookPagesCount.Set(entriesCountMgo(models.FacebookTable, nil))

		GalleriesCount.Set(entriesCountMgo(models.GalleryTable, nil))

		MirrorsCount.Set(entriesCountMgo(models.MirrorsTable, nil))

		RandomPictureSourcesCount.Set(entriesCountMgo(models.RandompictureSourcesTable, nil))

		RedditSubredditsCount.Set(entriesCountMgo(models.RedditSubredditsTable, nil))

		YoutubeChannelsCount.Set(entriesCountMgo(models.YoutubeChannelTable, nil))

		TwitchChannelsCount.Set(entriesCountMgo(models.TwitchTable, nil))

		VanityInvitesCount.Set(entriesCountMgo(models.VanityInvitesTable, nil))

		BiasgameImagesCount.Set(entriesAggregateMgo(models.IdolTable, "images"))

		BiasgameSuggestionsCount.Set(entriesCountMgo(models.IdolSuggestionsTable, bson.M{"status": ""}))

		BiasgameGamesCount.Set(entriesCountMgo(models.BiasGameTable, nil))

		var key string
		if cache.HasMachineryRedisClient() {
			key = "delayed_tasks"
			delayedTasks, err := cache.GetMachineryRedisClient().ZCard(key).Result()
			helpers.Relax(err)
			MachineryDelayedTasksCount.Set(delayedTasks)
		}

		key = models.YoutubeQuotaRedisKey
		codec := cache.GetRedisCacheCodec()
		var q models.YoutubeQuota
		if err := codec.Get(key, &q); err == nil {
			YoutubeLeftQuota.Set(q.Left)
		} else {
			YoutubeLeftQuota.Set(0)
		}

		redis := cache.GetRedisClient()
		numberOfProxies, _ := redis.SCard(helpers.PROXIES_KEY).Result()
		GimmeProxyCachedProxies.Set(numberOfProxies)

		auditLogBackfills, _ := redis.LLen(models.AuditLogBackfillRedisList).Result()
		EventlogPendingAuditlogBackfills.Set(auditLogBackfills)
	}
}

func entriesCountMgo(table models.MongoDbCollection, query interface{}) (count int64) {
	countT, err := helpers.MdbCollection(table).Find(query).Count()
	helpers.Relax(err)
	return int64(countT)
}

func entriesAggregateMgo(table models.MongoDbCollection, arrayField string) (count int64) {
	pipe := helpers.MdbCollection(table).Pipe([]bson.M{
		{
			"$project": bson.M{"sizeOfArrayField": bson.M{"$size": "$" + arrayField}},
		},
		{
			"$group": bson.M{
				"_id": nil,
				"count": bson.M{
					"$sum": "$sizeOfArrayField",
				},
			},
		},
	})

	resp := bson.M{}
	err := pipe.One(&resp)
	if err != nil {
		cache.GetLogger().WithField("module", "metrics").Errorln("Unable to aggregate for metrics: ", err.Error())
		return
	}

	if countT, ok := resp["count"].(int); ok {
		count = int64(countT)
	}

	return
}
