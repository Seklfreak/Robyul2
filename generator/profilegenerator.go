package generator

import (
    "errors"
    "sync"
    "github.com/Seklfreak/Robyul2/modules/plugins"
)

var (
    levels      *plugins.Levels
    levelsMutex sync.RWMutex
)

func SetProfileGenerator(l *plugins.Levels) {
    levelsMutex.Lock()
    levels = l
    levelsMutex.Unlock()
}

func GetProfileGenerator() *plugins.Levels {
    levelsMutex.RLock()
    defer levelsMutex.RUnlock()

    if levels == nil {
        panic(errors.New("Tried to get discord session before cache#SetProfileGenerator() was called"))
    }

    return levels
}
