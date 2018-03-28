package generator

import (
	"errors"
	"sync"

	levelsPlugin "github.com/Seklfreak/Robyul2/modules/plugins/levels"
)

var (
	levels      *levelsPlugin.Levels
	levelsMutex sync.RWMutex
)

func SetProfileGenerator(l *levelsPlugin.Levels) {
	levelsMutex.Lock()
	levels = l
	levelsMutex.Unlock()
}

func GetProfileGenerator() *levelsPlugin.Levels {
	levelsMutex.RLock()
	defer levelsMutex.RUnlock()

	if levels == nil {
		panic(errors.New("Tried to get discord session before cache#SetProfileGenerator() was called"))
	}

	return levels
}
