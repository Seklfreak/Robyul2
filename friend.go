package main

import (
	"fmt"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/bwmarrin/discordgo"
)

func FriendOnReady(session *discordgo.Session, event *discordgo.Ready) {
	log := cache.GetLogger()

	log.WithField("module", "friend").Info(fmt.Sprintf(
		"Connected friend %s (#%s) to discord!",
		session.State.User.Username, session.State.User.ID))

	// Cache the session
	cache.AddFriend(session)
}
