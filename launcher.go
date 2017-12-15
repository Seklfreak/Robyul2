package main

import (
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"time"

	"gopkg.in/olivere/elastic.v5"

	"fmt"

	"strings"

	"runtime"

	"github.com/RichardKnop/machinery/v1"
	marchineryConfig "github.com/RichardKnop/machinery/v1/config"
	marchineryLog "github.com/RichardKnop/machinery/v1/log"
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/logging"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/migrations"
	"github.com/Seklfreak/Robyul2/modules/plugins"
	"github.com/Seklfreak/Robyul2/rest"
	"github.com/Seklfreak/Robyul2/version"
	"github.com/bshuster-repo/logruzio"
	"github.com/bwmarrin/discordgo"
	"github.com/emicklei/go-restful"
	"github.com/getsentry/raven-go"
	"github.com/go-redis/redis"
	"github.com/kz/discordrus"
	"github.com/sirupsen/logrus"
	"gopkg.in/inconshreveable/go-keen.v0"
)

var (
	keenClient        *keen.Client
	BotRuntimeChannel chan os.Signal
)

// Entrypoint
func main() {
	var err error

	log := logrus.New()
	log.Out = os.Stdout
	log.Level = logrus.DebugLevel
	log.Formatter = &logrus.TextFormatter{ForceColors: true, FullTimestamp: true, TimestampFormat: time.RFC3339}
	log.Hooks = make(logrus.LevelHooks)
	cache.SetLogger(log)

	// Read config
	helpers.LoadConfig("config.json")
	config := helpers.GetConfig()

	// Check if the bot is being debugged
	if config.Path("debug").Data().(bool) {
		helpers.DEBUG_MODE = true
	}

	if config.Path("logging.jsonfile").Data().(string) != "" {
		fileHook, err := logging.NewLogrusFileHook(config.Path("logging.jsonfile").Data().(string), os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
		if err != nil {
			log.WithField("module", "launcher").Error("logrus file hook failed, err:", err.Error())
		} else {
			log.Hooks.Add(fileHook)
		}
	}

	if config.Path("logging.discord_webhook").Data().(string) != "" {
		log.Hooks.Add(discordrus.NewHook(
			config.Path("logging.discord_webhook").Data().(string),
			logrus.ErrorLevel,
			&discordrus.Opts{
				Username:           "Logging",
				DisableTimestamp:   false,
				TimestampFormat:    "Jan 2 15:04:05.00000",
				EnableCustomColors: true,
				CustomLevelColors: &discordrus.LevelColors{
					//Debug: 10170623,
					//Info:  3581519,
					//Warn:  14327864,
					Error: 13631488,
					Panic: 13631488,
					Fatal: 13631488,
				},
			},
		))
	}

	if config.Path("logging.logzio_token").Data().(string) != "" {
		logruzioCtx := logrus.Fields{}
		if version.BOT_VERSION != "UNSET" {
			logruzioCtx["version"] = version.BOT_VERSION
		}
		hook, err := logruzio.New(config.Path("logging.logzio_token").Data().(string), "robyul-discord", logruzioCtx)
		if err != nil {
			log.Fatal(err)
		}
		log.Hooks.Add(hook)
	}

	log.WithField("module", "launcher").Info("Booting Robyul...")

	// Read i18n
	helpers.LoadTranslations()

	// Show version
	version.DumpInfo()

	// Start metric server
	metrics.Init()

	// Make the randomness more random
	rand.Seed(time.Now().UTC().UnixNano())

	// Print UA
	log.WithField("module", "launcher").Info("USERAGENT: '" + helpers.DEFAULT_UA + "'")

	// Call home
	log.WithField("module", "launcher").Info("[SENTRY] Calling home...")
	err = raven.SetDSN(config.Path("sentry").Data().(string))
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

	// Connect to elastic search
	if config.Path("elasticsearch.url").Data().(string) != "" {
		log.WithField("module", "launcher").Info("Connecting bot to elastic search...")
		client, err := elastic.NewClient(
			elastic.SetURL(config.Path("elasticsearch.url").Data().(string)),
			elastic.SetSniff(false),
		)
		if err != nil {
			panic(err)
		}
		cache.SetElastic(client)
	}

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
	discordgo.Logger = func(msgL, caller int, format string, a ...interface{}) {
		pc, file, line, _ := runtime.Caller(caller)

		files := strings.Split(file, "/")
		file = files[len(files)-1]

		name := runtime.FuncForPC(pc).Name()
		fns := strings.Split(name, ".")
		name = fns[len(fns)-1]

		msg := format
		if strings.Contains(msg, "%") {
			msg = fmt.Sprintf(format, a...)
		}

		switch msgL {
		case discordgo.LogError:
			log.WithField("module", "discordgo").Errorf("%s:%d:%s() %s", file, line, name, msg)
		case discordgo.LogWarning:
			log.WithField("module", "discordgo").Warnf("%s:%d:%s() %s", file, line, name, msg)
		case discordgo.LogInformational:
			log.WithField("module", "discordgo").Infof("%s:%d:%s() %s", file, line, name, msg)
		case discordgo.LogDebug:
			log.WithField("module", "discordgo").Debugf("%s:%d:%s() %s", file, line, name, msg)
		}
	}
	log.WithField("module", "launcher").Info("Connecting Robyul to discord...")
	discord, err := discordgo.New("Bot " + config.Path("discord.token").Data().(string))
	if err != nil {
		panic(err)
	}

	discord.Lock()
	discord.Debug = false
	discord.LogLevel = discordgo.LogInformational
	discord.StateEnabled = true
	discord.Unlock()

	discord.AddHandler(BotOnReady)
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
	discord.AddHandler(BotOnGuildCreate)
	discord.AddHandler(BotOnGuildDelete)

	if cache.HasElastic() {
		discord.AddHandler(helpers.ElasticOnMessageCreate)
		discord.AddHandler(helpers.ElasticOnMessageUpdate)
		discord.AddHandler(helpers.ElasticOnMessageDelete)
		discord.AddHandler(helpers.ElasticOnGuildMemberRemove)
		discord.AddHandler(helpers.ElasticOnReactionAdd)
		discord.AddHandler(helpers.ElasticOnPresenceUpdate)
		// Guild Member Add in modules/plugins/mod.go
	}

	// Connect to discord
	err = discord.Open()
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		panic(err)
	}

	// Connect helper
	friendsConfigs, err := config.Path("friends").Children()
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		panic(err)
	}
	for _, friendConfig := range friendsConfigs {
		if friendConfig.Path("token").Data().(string) != "" {
			log.WithField("module", "launcher").Infof("Connecting friend to discord...")
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

	// create keen client
	if config.Path("keen.project_id").Data().(string) != "" &&
		config.Path("keen.key").Data().(string) != "" {
		log.WithField("module", "launcher").Info("Connecting bot to keen.io...")
		keenClient = &keen.Client{
			ProjectToken: config.Path("keen.project_id").Data().(string),
			ApiKey:       config.Path("keen.key").Data().(string),
		}
	}

	// Open REST API
	wsContainer := restful.NewContainer()

	for _, service := range rest.NewRestServices() {
		wsContainer.Add(service)
	}
	wsContainer.Filter(func(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
		// Add CORS header
		allowedHosts := []string{"https://robyul.chat", "https://api.robyul.chat", "http://localhost:8000"}
		if origin := req.Request.Header.Get("Origin"); origin != "" {
			for _, allowedHost := range allowedHosts {
				if allowedHost == origin {
					resp.AddHeader("Access-Control-Allow-Origin", origin)
					resp.AddHeader("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
					resp.AddHeader("Access-Control-Max-Age", "1000")
					resp.AddHeader("Access-Control-Allow-Headers", "origin, x-csrftoken, content-type, accept, Authorization")
				}
			}
		}
		// Log request and time
		now := time.Now()
		chain.ProcessFilter(req, resp)
		tookTime := time.Now().Sub(now)
		log.WithField("module", "launcher").Info(fmt.Sprintf("received api request: %s %s%s (took %v)",
			req.Request.Method, req.Request.Host, req.Request.URL, tookTime))
		logKeenRequest(req, tookTime.Seconds())
	})
	wsContainer.Filter(wsContainer.OPTIONSFilter)

	go func() {
		server := &http.Server{Addr: "localhost:2021", Handler: wsContainer}
		log.Fatal(server.ListenAndServe())
	}()
	log.WithField("module", "launcher").Info("REST API listening on localhost:2021")

	// Launch machinery
	marchineryLog.Set(log.WithField("module", "machinery"))
	machineryServerConfig := &marchineryConfig.Config{
		Broker:          "redis://" + config.Path("redis.address").Data().(string) + "/1",
		DefaultQueue:    "robyul_tasks",
		ResultBackend:   "redis://" + config.Path("redis.address").Data().(string) + "/1",
		ResultsExpireIn: 3600,
	}
	machineryServer, err := machinery.NewServer(machineryServerConfig)
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		panic(err)
	}
	log.WithField("module", "launcher").Info("started machinery server, default queue: robyul_tasks")
	machineryServer.RegisterTasks(map[string]interface{}{
		"unmute_user":    helpers.UnmuteUser,
		"apply_autorole": plugins.AutoroleApply,
		"log_error":      helpers.LogMachineryError,
	})
	cache.SetMachineryServer(machineryServer)
	worker := machineryServer.NewWorker("robyul_worker_1", 1)
	go func() {
		cache.AddMachineryActiveWorker(worker)
		err = worker.Launch()
		cache.RemoveMachineryActiveWorker(worker)
		if err != nil {
			if !strings.Contains(err.Error(), "Signal received: interrupt") && !strings.Contains(err.Error(), "errorWorker quit gracefully") {
				raven.CaptureErrorAndWait(err, nil)
				panic(err)
			}
		}
	}()
	log.WithField("module", "launcher").Info("started machinery worker robyul_worker_1 with concurrency 1")
	machineryRedisClient := redis.NewClient(&redis.Options{
		Addr:     config.Path("redis.address").Data().(string),
		Password: "", // no password set
		DB:       1,  // use default DB
	})
	cache.SetMachineryRedisClient(machineryRedisClient)

	// Make a channel that waits for a os signal
	BotRuntimeChannel = make(chan os.Signal, 1)
	signal.Notify(BotRuntimeChannel, os.Interrupt, os.Kill)

	// Wait until the os wants us to shutdown
	<-BotRuntimeChannel

	log.WithField("module", "launcher").Info("Robyul is stopping")
	log.WithField("module", "launcher").Info("Uninitializing plugins...")
	BotDestroy()
	log.WithField("module", "launcher").Info("Disconnecting bot discord session...")
	discord.Close()
	log.WithField("module", "launcher").Info("Disconnecting friend discord sessions...")
	for _, friendSession := range cache.GetFriends() {
		friendSession.Close()
	}
}

type KeenRestEvent struct {
	Seconds   float64
	Method    string
	Host      string
	Referer   string
	URL       string
	Origin    string
	UserAgent string
	Query     string
}

func logKeenRequest(request *restful.Request, timeInSeconds float64) {
	if keenClient.ApiKey != "" && keenClient.ProjectToken != "" {
		err := keenClient.AddEvent("Robyul_REST_API", &KeenRestEvent{
			Seconds:   timeInSeconds,
			Method:    request.Request.Method,
			Host:      request.Request.Host,
			Referer:   request.Request.Referer(),
			URL:       request.Request.URL.Path,
			Origin:    request.Request.Header.Get("Origin"),
			UserAgent: request.Request.Header.Get("User-Agent"),
			Query:     request.Request.URL.RawQuery,
		})
		if err != nil {
			cache.GetLogger().WithField("module", "launcher").Error("Error logging API request to keen: ", err.Error())
		}
	}
}
