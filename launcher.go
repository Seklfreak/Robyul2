package main

import (
    "github.com/bwmarrin/discordgo"
    "github.com/getsentry/raven-go"
    Logger "github.com/sn0w/Karen/logger"
    "github.com/sn0w/Karen/migrations"
    "github.com/sn0w/Karen/utils"
    "os"
    "os/signal"
)

// The discord session holder
var discordSession *discordgo.Session

// Defines what a callback is
type Callback func()

// Entrypoint
func main() {
    Logger.INF("Bootstrapping...")

    // Read config
    utils.LoadConfig("config.json")
    config := utils.GetConfig()

    // Call home
    Logger.INF("[SENTRY] Calling home...")
    err := raven.SetDSN(config.Path("sentry").Data().(string))
    if err != nil {
        panic(err)
    }
    Logger.INF("[SENTRY] Someone picked up the phone \\^-^/")

    // Connect to DB
    utils.ConnectDB(
        config.Path("rethink.url").Data().(string),
        config.Path("rethink.db").Data().(string),
    )

    // Close DB when main dies
    defer utils.GetDB().Close()

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
