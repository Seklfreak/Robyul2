package main

import (
    "github.com/bwmarrin/discordgo"
    Logger "./logger"
    "os"
    "os/signal"
    "./utils"
)

var discordSession *discordgo.Session

type Callback func()

func main() {
    Logger.INF("Bootstrapping...")

    // Read config
    utils.LoadConfig("config.json")
    config := utils.GetConfig()

    // Connect to DB
    utils.ConnectDB(
        config.Path("mongo.url").Data().(string),
        config.Path("mongo.db").Data().(string),
    )

    defer utils.GetDBSession().Close()

    // Connect and add event handlers
    discord, err := discordgo.New("Bot " + config.Path("discord.token").Data().(string))
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