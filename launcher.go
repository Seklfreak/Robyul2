package main

import (
    "github.com/bwmarrin/discordgo"
    "github.com/getsentry/raven-go"
    Logger "github.com/sn0w/Karen/logger"
    "github.com/sn0w/Karen/migrations"
    "os"
    "os/signal"
    "github.com/sn0w/Karen/helpers"
    "github.com/sn0w/Karen/metrics"
)

// The discord session holder
var discordSession *discordgo.Session

// Defines what a callback is
type Callback func()

// Entrypoint
func main() {
    Logger.INF("Bootstrapping...")

    // Start metric server
    metrics.Init()

    // Read config
    helpers.LoadConfig("config.json")
    config := helpers.GetConfig()

    // Check if the bot is being debugged
    if config.Path("debug").Data().(bool) {
        helpers.DEBUG_MODE = true
    }

    // Call home
    Logger.INF("[SENTRY] Calling home...")
    err := raven.SetDSN(config.Path("sentry").Data().(string))
    if err != nil {
        panic(err)
    }
    Logger.INF("[SENTRY] Someone picked up the phone \\^-^/")

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
    Logger.INF("Connecting to discord...")
    discord, err := discordgo.New("Bot " + config.Path("discord.token").Data().(string))
    if err != nil {
        panic(err)
    }

    // Register callbacks in proxy
    ProxyAttachListeners(discord, ProxiedEventHandlers{
        BotOnReady,
        BotOnMessageCreate,

        metrics.OnReady,
        metrics.OnMessageCreate,
    })

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

    Logger.WRN("The OS is killing me :c")
    Logger.WRN("Disconnecting...")
    discord.Close()
}
