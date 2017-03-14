package cache

import (
    "errors"
    "github.com/bwmarrin/discordgo"
    "sync"
)

var (
    session      *discordgo.Session
    sessionMutex sync.RWMutex
)

func SetSession(s *discordgo.Session) {
    sessionMutex.Lock()
    session = s
    sessionMutex.Unlock()
}

func GetSession() *discordgo.Session {
    sessionMutex.RLock()
    defer sessionMutex.RUnlock()

    if session == nil {
        panic(errors.New("Tried to get discord session before cache#setSession() was called"))
    }

    return session
}
