package main

import (
    "github.com/bwmarrin/discordgo"
    redis "gopkg.in/redis.v3"
    "io/ioutil"
    "./logger"
    "strings"
    "os"
    "os/signal"
)

var (
    discord *discordgo.Session
    rcli    *redis.Client
)

func main() {
    logger.INF("Bootstrapping...")

    // Read token
    file, _ := ioutil.ReadFile("token")
    token := string(file)

    // Remove newline from token
    token = strings.TrimRight(token, "\n")

    // Connect and add event handlers
    discord, err := discordgo.New(token)

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

    <- channel
}
