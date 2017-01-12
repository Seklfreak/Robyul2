package main

import (
    "github.com/bwmarrin/discordgo"
    "github.com/getsentry/raven-go"
    "github.com/sn0w/Karen/helpers"
    Logger "github.com/sn0w/Karen/logger"
    "github.com/sn0w/Karen/metrics"
    "github.com/sn0w/Karen/migrations"
    "github.com/sn0w/Karen/version"
    "math/rand"
    "os"
    "os/signal"
    "time"
)

// Entrypoint
func main() {
    Logger.INFO.L("launcher", "Booting Karen...")

    // Show version
    version.DumpInfo()

    // Start metric server
    metrics.Init()

    // Make the randomness more random
    rand.Seed(time.Now().UTC().UnixNano())

    // Read config
    helpers.LoadConfig("config.json")
    config := helpers.GetConfig()

    // Check if the bot is being debugged
    if config.Path("debug").Data().(bool) {
        helpers.DEBUG_MODE = true
    }

    // Print UA
    Logger.WARNING.L("launcher", "USERAGENT: '" + helpers.DEFAULT_UA + "'")

    // Call home
    Logger.INFO.L("launcher", "[SENTRY] Calling home...")
    err := raven.SetDSN(config.Path("sentry").Data().(string))
    if err != nil {
        panic(err)
    }
    Logger.INFO.L("launcher", "[SENTRY] Someone picked up the phone \\^-^/")

    // Connect to DB
    helpers.ConnectDB(
        config.Path("rethink.url").Data().(string),
        config.Path("rethink.db").Data().(string),
    )

    // Close DB when main dies
    defer helpers.GetDB().Close()

    // Run migrations
    migrations.Run()

    // Connect and add event handlers
    Logger.INFO.L("launcher", "Connecting to discord...")
    discord, err := discordgo.New("Bot " + config.Path("discord.token").Data().(string))
    if err != nil {
        panic(err)
    }

    discord.AddHandler(BotOnReady)
    discord.AddHandler(BotOnMessageCreate)
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

    Logger.WARNING.L("launcher", "The OS is killing me :c")
    Logger.WARNING.L("launcher", "Disconnecting...")
    discord.Close()
}
