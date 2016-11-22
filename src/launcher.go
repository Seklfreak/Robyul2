package main

import (
    "github.com/bwmarrin/discordgo"
    Logger "./logger"
    "os"
    "os/signal"
    redis "gopkg.in/redis.v5"
    "github.com/Jeffail/gabs"
)

var (
    config *gabs.Container
    rcli *redis.Client
)

func main() {
    Logger.INF("Bootstrapping...")

    // Read config
    config = GetConfig("config.json")

    // Connect to DB
    rcli = redis.NewClient(&redis.Options{
        Addr: config.Path("redis").Data().(string),
        DB: 0,
    })

    _, err := rcli.Ping().Result()
    if err != nil {
        Logger.ERR("Cannot connect to redis!")
        os.Exit(1)
    }

    // Connect and add event handlers
    discord, err := discordgo.New(config.Path("discord.token").Data().(string))
    if err != nil {
        panic(err)
    }

    discord.AddHandler(onReady)
    discord.AddHandler(onMessageCreate)

    err = discord.Open()
    if err != nil {
        panic(err)
    }

    // Make a channel that waits for a os signal
    channel := make(chan os.Signal, 1)
    signal.Notify(channel, os.Interrupt, os.Kill)

    <-channel

    Logger.WRN("The OS is killing me :c")
    Logger.WRN("Disconnecting...")
    discord.Close()
}