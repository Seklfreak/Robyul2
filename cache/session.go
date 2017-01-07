package cache

import (
    "github.com/bwmarrin/discordgo"
    "errors"
)

var session *discordgo.Session

func SetSession(s *discordgo.Session) {
    session = s
}

func GetSession() *discordgo.Session {
    if session == nil {
        panic(errors.New("Tried to get discord session before cache#setSession() was called"))
    }

    return session
}
