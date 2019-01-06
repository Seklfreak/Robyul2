package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"

	unleash "github.com/Unleash/unleash-client-go"

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
	"github.com/Seklfreak/Robyul2/robyulstate"
	"github.com/Seklfreak/Robyul2/version"
	polr "github.com/Seklfreak/polr-go"
	"github.com/bwmarrin/discordgo"
	restful "github.com/emicklei/go-restful"
	raven "github.com/getsentry/raven-go"
	"github.com/go-redis/redis"
	"github.com/kz/discordrus"
	"github.com/olivere/elastic"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"

	_ "net/http/pprof"
)

var (
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
	log.WithField("module", "launcher").Info("Connecting to sentry...")
	err = raven.SetDSN(config.Path("sentry").Data().(string))
	if err != nil {
		panic(err)
	}
	if version.BOT_VERSION != "UNSET" {
		raven.SetRelease(version.BOT_VERSION)
	}
	log.WithField("module", "launcher").Info(
		"Connected to Sentry Project ID: ", raven.ProjectID(), " Release: ", raven.Release())

	// Connect to MongoDB
	helpers.ConnectMDB(
		config.Path("mongodb.url").Data().(string),
		config.Path("mongodb.db").Data().(string),
	)
	defer helpers.GetMDbSession().Close()

	// Connect to elastic search
	if config.Path("elasticsearch.url").Data().(string) != "" {
		log.WithField("module", "launcher").Info("Connecting to ElasticSearch...")
		client, err := elastic.NewClient(
			elastic.SetURL(config.Path("elasticsearch.url").Data().(string)),
			elastic.SetSniff(false),
			elastic.SetErrorLog(log),
			//elastic.SetInfoLog(log),
		)
		if err != nil {
			panic(err)
		}
		cache.SetElastic(client)

		version, err := client.ElasticsearchVersion(config.Path("elasticsearch.url").Data().(string))
		if err != nil {
			panic(err)
		}
		log.WithField("module", "launcher").Info("Connected to ElasticSearch v" + version)
	}

	if config.ExistsP("polr.url") &&
		config.ExistsP("polr.api-key") &&
		config.Path("polr.url").Data().(string) != "" &&
		config.Path("polr.api-key").Data().(string) != "" {
		polrClient, err := polr.New(
			config.Path("polr.url").Data().(string),
			config.Path("polr.api-key").Data().(string),
			nil,
		)
		if err != nil {
			panic(err)
		}
		cache.SetPolr(polrClient)
	}

	// Run migrations
	migrations.Run()

	// stop after migrations?
	for _, arg := range os.Args {
		if arg == "stop-after-migration" {
			log.WithField("module", "launcher").Info("stopping after migration")
			return
		}
	}

	// Connecting to redis
	log.WithField("module", "launcher").Info("Connecting to redis...")
	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.Path("redis.address").Data().(string),
		Password: "", // no password set
		DB:       0,  // use default DB
		//DialTimeout: 5 * time.Minute,
		//ReadTimeout: 5 * time.Minute,
	})
	cache.SetRedisClient(redisClient)

	// Set up Google Drive Client
	if helpers.GetConfig().Path("google.client_credentials_json_location").Data().(string) != "" {
		driveCtx := context.Background()
		driveAuthJson, err := ioutil.ReadFile(helpers.GetConfig().Path("google.client_credentials_json_location").Data().(string))
		if err != nil {
			panic(err)
		}

		driveConfigs, err := google.JWTConfigFromJSON(driveAuthJson, drive.DriveReadonlyScope)
		if err != nil {
			panic(err)
		}

		driveClient := driveConfigs.Client(driveCtx)
		driveService, err := drive.New(driveClient)
		if err != nil {
			panic(err)
		}

		cache.SetGoogleDriveService(driveService)
	}

	// connect to unleash
	if helpers.GetConfig().ExistsP("unleash.app-name") &&
		helpers.GetConfig().ExistsP("unleash.instance-id") &&
		helpers.GetConfig().ExistsP("unleash.url") &&
		helpers.GetConfig().Path("unleash.app-name").Data().(string) != "" &&
		helpers.GetConfig().Path("unleash.instance-id").Data().(string) != "" &&
		helpers.GetConfig().Path("unleash.url").Data().(string) != "" {
		log.WithField("module", "launcher").Info("Connecting to unleash…")
		err := unleash.Initialize(
			unleash.WithListener(&helpers.UnleashListener{}),
			unleash.WithAppName(helpers.GetConfig().Path("unleash.app-name").Data().(string)),
			unleash.WithUrl(helpers.GetConfig().Path("unleash.url").Data().(string)),
			unleash.WithInstanceId(helpers.GetConfig().Path("unleash.instance-id").Data().(string)),
		)
		if err != nil {
			panic(err)
		}
		helpers.UnleashInitialised = true
	}

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
	//discord.LogLevel = discordgo.LogInformational
	discord.LogLevel = discordgo.LogError
	discord.StateEnabled = true
	discord.MaxRestRetries = 5
	discord.State.MaxMessageCount = 10
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
		discord.AddHandler(helpers.ElasticOnPresenceUpdate)
		// Guild Member Add in modules/plugins/mod.go
	}

	robyulState := robyulstate.NewState()
	robyulState.Logger = func(msgL, caller int, format string, a ...interface{}) {
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
			log.WithField("module", "robyulState").Errorf("%s:%d:%s() %s", file, line, name, msg)
		case discordgo.LogWarning:
			log.WithField("module", "robyulState").Warnf("%s:%d:%s() %s", file, line, name, msg)
		case discordgo.LogInformational:
			log.WithField("module", "robyulState").Infof("%s:%d:%s() %s", file, line, name, msg)
		case discordgo.LogDebug:
			log.WithField("module", "robyulState").Debugf("%s:%d:%s() %s", file, line, name, msg)
		}
	}

	discord.AddHandler(robyulState.OnInterface)

	// Connect to discord
	err = discord.Open()
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		panic(err)
	}

	// Open REST API
	wsContainer := restful.NewContainer()

	// configure CORS filter
	cors := restful.CrossOriginResourceSharing{
		AllowedDomains: []string{
			"https://robyul.chat",
			"https://api.robyul.chat",
			"http://localhost:8000",
			"http://robyul-web.local:8000",
		},
		AllowedHeaders: []string{"Content-Type", "Accept", "Origin", "X-CSRF-Token", "Authorization"},
		AllowedMethods: []string{"GET", "POST"},
		MaxAge:         1000,
		Container:      wsContainer,
	}
	wsContainer.Filter(cors.Filter)
	wsContainer.Filter(wsContainer.OPTIONSFilter)

	for _, service := range rest.NewRestServices() {
		wsContainer.Add(service)
	}

	wsContainer.Filter(func(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
		// Log request and time
		now := time.Now()
		chain.ProcessFilter(req, resp)
		tookTime := time.Now().Sub(now)
		log.WithField("module", "launcher").Info(fmt.Sprintf("received api request: %s %s%s (took %v)",
			req.Request.Method, req.Request.Host, req.Request.URL, tookTime))
		logKeenRequest(req, tookTime.Seconds())
	})

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
		"unmute_user":    helpers.UnmuteUserMachinery,
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
			if !strings.Contains(err.Error(), "Signal received: interrupt") && !strings.Contains(err.Error(), "Worker quit gracefully") {
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

	// start proxies healthcheck loop
	go helpers.CachedProxiesHealthcheckLoop()

	// Make a channel that waits for a os signal
	BotRuntimeChannel = make(chan os.Signal, 1)
	signal.Notify(BotRuntimeChannel, os.Interrupt, os.Kill)

	// Wait until the os wants us to shutdown
	<-BotRuntimeChannel

	log.WithField("module", "launcher").Info("Robyul is stopping")

	// shutdown everything
	finished := make(chan bool, 1)
	go func() {
		log.WithField("module", "launcher").Info("Uninitializing plugins...")
		BotDestroy()
		log.WithField("module", "launcher").Info("Disconnecting bot discord session...")
		discord.Close()
		log.WithField("module", "launcher").Info("Disconnecting friend discord sessions...")
		for _, friendSession := range cache.GetFriends() {
			friendSession.Close()
		}
		finished <- true
	}()

	// wait 60 second for everything to finish, or shut it down anyway
	select {
	case <-finished:
		log.WithField("module", "launcher").Infoln("shutdown successful")
	case <-time.After(60 * time.Second):
		log.WithField("module", "launcher").Infoln("forcing shutdown after 60 seconds")
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
	if cache.HasKeen() {
		err := cache.GetKeen().AddEvent("Robyul_REST_API", &KeenRestEvent{
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
