package main

import (
	Logger "./logger"
	"github.com/bwmarrin/discordgo"
)

func onReady(session *discordgo.Session, event *discordgo.Ready) {
	Logger.INF("Connected to discord!")
}

func onMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
    if message.Content == "!ping" {
        session.ChannelMessageSend(message.ChannelID, "Pong!")
    }
}