package main

import (
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"time"

	elastic "gopkg.in/olivere/elastic.v5"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/migrations"
	"github.com/Seklfreak/Robyul2/rest"
	"github.com/Seklfreak/Robyul2/version"
	"github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
	"github.com/emicklei/go-restful"
	"github.com/getsentry/raven-go"
	"github.com/go-redis/redis"
)

// Entrypoint
func main() {
	log := logrus.New()
	log.Out = os.Stdout
	log.Level = logrus.DebugLevel
	log.Formatter = &logrus.TextFormatter{ForceColors: true, FullTimestamp: true, TimestampFormat: time.RFC3339}
	cache.SetLogger(log)

	log.WithField("module", "launcher").Info("Booting Robyul...")

	// Read config
	helpers.LoadConfig("config.json")
	config := helpers.GetConfig()

	// Read i18n
	helpers.LoadTranslations()

	// Show version
	version.DumpInfo()

	// Start metric server
	metrics.Init()

	// Make the randomness more random
	rand.Seed(time.Now().UTC().UnixNano())

	// Check if the bot is being debugged
	if config.Path("debug").Data().(bool) {
		helpers.DEBUG_MODE = true
	}

	// Print UA
	log.WithField("module", "launcher").Info("USERAGENT: '" + helpers.DEFAULT_UA + "'")

	// Call home
	log.WithField("module", "launcher").Info("[SENTRY] Calling home...")
	err := raven.SetDSN(config.Path("sentry").Data().(string))
	if err != nil {
		panic(err)
	}
	if version.BOT_VERSION != "UNSET" {
		raven.SetRelease(version.BOT_VERSION)
	}
	log.WithField("module", "launcher").Info("[SENTRY] Someone picked up the phone \\^-^/")

	// Connect to DB
	log.WithField("module", "launcher").Info("Opening database connection...")
	helpers.ConnectDB(
		config.Path("rethink.url").Data().(string),
		config.Path("rethink.db").Data().(string),
	)

	// Close DB when main dies
	defer helpers.GetDB().Close()

	// Run migrations
	migrations.Run()

	// Connecting to redis
	log.WithField("module", "launcher").Info("Connecting to redis...")
	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.Path("redis.address").Data().(string),
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	cache.SetRedisClient(redisClient)

	// Connect and add event handlers
	log.WithField("module", "launcher").Info("Connecting bot to discord...")
	discord, err := discordgo.New("Bot " + config.Path("discord.token").Data().(string))
	if err != nil {
		panic(err)
	}

	discord.Lock()
	discord.Debug = false
	discord.LogLevel = discordgo.LogInformational
	discord.StateEnabled = true
	discord.Unlock()

	discord.AddHandlerOnce(BotOnReady)
	discord.AddHandler(BotOnMessageCreate)
	discord.AddHandler(BotOnMessageDelete)
	discord.AddHandler(BotOnGuildMemberAdd)
	discord.AddHandler(BotOnGuildMemberRemove)
	discord.AddHandler(BotOnReactionAdd)
	discord.AddHandler(BotOnReactionRemove)
	discord.AddHandler(BotOnGuildBanAdd)
	discord.AddHandler(BotOnGuildBanRemove)
	discord.AddHandlerOnce(metrics.OnReady)
	discord.AddHandler(metrics.OnMessageCreate)
	discord.AddHandler(BotOnMemberListChunk)
	discord.AddHandler(BotGuildOnPresenceUpdate)

	// Connect to discord
	err = discord.Open()
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		panic(err)
	}

	// Connect helper
	friendsConfigs, err := config.Path("friends").Children()
	if err != nil {
		panic(err)
	}
	for _, friendConfig := range friendsConfigs {
		if friendConfig.Path("token").Data().(string) != "" {
			log.WithField("module", "launcher").Info("Connecting helper to discord...")
			discordFriend, err := discordgo.New(
				friendConfig.Path("token").Data().(string),
			)
			if err != nil {
				panic(err)
			}

			discordFriend.Lock()
			discordFriend.Debug = false
			discordFriend.LogLevel = discordgo.LogInformational
			discordFriend.StateEnabled = true
			discordFriend.Unlock()

			discordFriend.AddHandlerOnce(FriendOnReady)

			// Connect to discord
			err = discordFriend.Open()
			if err != nil {
				raven.CaptureErrorAndWait(err, nil)
				panic(err)
			}
		}
	}

	// Open REST API
	for _, service := range rest.NewRestServices() {
		restful.Add(service)
	}
	go func() {
		log.Fatal(http.ListenAndServe("localhost:2021", nil))
	}()
	log.WithField("module", "launcher").Info("REST API listening on localhost:2021")

	// Connect to elastic search
	if config.Path("elasticsearch.url").Data().(string) != "" {
		log.WithField("module", "launcher").Info("Connecting bot to elastic search...")
		client, err := elastic.NewClient(
			elastic.SetURL(config.Path("elasticsearch.url").Data().(string)),
		)
		if err != nil {
			panic(err)
		}
		cache.SetElastic(client)
		discord.AddHandler(helpers.ElasticOnMessageCreate)
		discord.AddHandler(helpers.ElasticOnGuildMemberAdd)
		discord.AddHandler(helpers.ElasticOnGuildMemberRemove)
		discord.AddHandler(helpers.ElasticOnReactionAdd)
	}

	// Make a channel that waits for a os signal
	channel := make(chan os.Signal, 1)
	signal.Notify(channel, os.Interrupt, os.Kill)

	// Wait until the os wants us to shutdown
	<-channel

	log.WithField("module", "launcher").Info("The OS is killing me :c")
	log.WithField("module", "launcher").Info("Disconnecting...")
	discord.Close()
}
