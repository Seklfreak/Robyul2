package main

import (
    "github.com/Seklfreak/Robyul2/helpers"
    Logger "github.com/Seklfreak/Robyul2/logger"
    "github.com/Seklfreak/Robyul2/metrics"
    "github.com/Seklfreak/Robyul2/migrations"
    "github.com/Seklfreak/Robyul2/version"
    "github.com/bwmarrin/discordgo"
    "github.com/getsentry/raven-go"
    "math/rand"
    "os"
    "os/signal"
    "time"
    "github.com/go-redis/redis"
    "github.com/Seklfreak/Robyul2/cache"
)

// Entrypoint
func main() {
    Logger.BOOT.L("launcher", "Booting Karen...")

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
        Logger.DEBUG_MODE = true
    }

    // Print UA
    Logger.BOOT.L("launcher", "USERAGENT: '"+helpers.DEFAULT_UA+"'")

    // Call home
    Logger.BOOT.L("launcher", "[SENTRY] Calling home...")
    err := raven.SetDSN(config.Path("sentry").Data().(string))
    if err != nil {
        panic(err)
    }
    if version.BOT_VERSION != "UNSET" {
        raven.SetRelease(version.BOT_VERSION)
    }
    Logger.BOOT.L("launcher", "[SENTRY] Someone picked up the phone \\^-^/")

    // Connect to DB
    Logger.BOOT.L("launcher", "Opening database connection...")
    helpers.ConnectDB(
        config.Path("rethink.url").Data().(string),
        config.Path("rethink.db").Data().(string),
    )

    // Close DB when main dies
    defer helpers.GetDB().Close()

    // Run migrations
    migrations.Run()

    // Connecting to redis
    Logger.BOOT.L("launcher", "Connecting to redis...")
    redisClient := redis.NewClient(&redis.Options{
        Addr:     config.Path("redis.address").Data().(string),
        Password: "", // no password set
        DB:       0,  // use default DB
    })
    cache.SetRedisClient(redisClient)

    // Connect and add event handlers
    Logger.BOOT.L("launcher", "Connecting to discord...")
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
    discord.AddHandler(BotOnGuildMemberAdd)
    discord.AddHandler(BotOnGuildMemberRemove)
    discord.AddHandler(BotOnReactionAdd)
    discord.AddHandler(BotOnReactionRemove)
    discord.AddHandler(BotOnGuildBanAdd)
    discord.AddHandler(BotOnGuildBanRemove)
    discord.AddHandler(metrics.OnReady)
    discord.AddHandler(metrics.OnMessageCreate)

    // Connect to discord
    err = discord.Open()
    if err != nil {
        raven.CaptureErrorAndWait(err, nil)
        panic(err)
    }

    // Make a channel that waits for a os signal
    channel := make(chan os.Signal, 1)
    signal.Notify(channel, os.Interrupt, os.Kill)

    // Wait until the os wants us to shutdown
    <-channel

    Logger.ERROR.L("launcher", "The OS is killing me :c")
    Logger.ERROR.L("launcher", "Disconnecting...")
    discord.Close()
}
