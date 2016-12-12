package main

import (
    "github.com/bwmarrin/discordgo"
    Logger "github.com/sn0w/Karen/logger"
    "os"
    "os/signal"
    "github.com/sn0w/Karen/utils"
    "github.com/getsentry/raven-go"
    "github.com/sn0w/Karen/music"
)

var discordSession *discordgo.Session

type Callback func()

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

    defer utils.GetDB().Close()

    // Connect and add event handlers
    discord, err := discordgo.New("Bot " + config.Path("discord.token").Data().(string))
    if err != nil {
        panic(err)
    }

    // Add event listeners
    discord.AddHandler(onReady)
    discord.AddHandler(onMessageCreate)

    err = discord.Open()
    if err != nil {
        raven.CaptureErrorAndWait(err, nil)
        panic(err)
    }

    // Launch music worker
    stopMusicQueueManager := make(chan bool)
    go music.StartQueueManager(stopMusicQueueManager)

    // Make a channel that waits for a os signal
    channel := make(chan os.Signal, 1)
    signal.Notify(channel, os.Interrupt, os.Kill)

    <-channel

    stopMusicQueueManager <- true

    Logger.WRN("The OS is killing me :c")
    Logger.WRN("Disconnecting...")
    discord.Close()
}