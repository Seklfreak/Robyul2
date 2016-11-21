package main

import (
    "fmt"
	Logger "./logger"
	"github.com/bwmarrin/discordgo"
)

func onReady(session *discordgo.Session, event *discordgo.Ready) {
	Logger.INF("Connected to discord!")
    fmt.Printf(
        "\n To add me to your discord server visit https://discordapp.com/oauth2/authorize?client_id=%s&scope=bot&permissions=%d\n\n",
        "249908516880515072",
        65535,
    )
}

func onMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
    fmt.Println("[MSG] " + message.Content)
}