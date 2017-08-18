package cache

import (
	"errors"
	"sync"
)

var (
	pluginCommandList         []string
	pluginExtendedCommandList []string
	triggerPluginCommandList  []string
	modulelistsMutex          sync.RWMutex
)

func SetPluginList(l []string) {
	modulelistsMutex.Lock()
	pluginCommandList = l
	modulelistsMutex.Unlock()
}

func GetPluginList() []string {
	modulelistsMutex.RLock()
	defer modulelistsMutex.RUnlock()

	if pluginCommandList == nil {
		panic(errors.New("Tried to get plugin list before cache#SetPluginList() was called"))
	}

	return pluginCommandList
}

func SetPluginExtendedList(l []string) {
	modulelistsMutex.Lock()
	pluginExtendedCommandList = l
	modulelistsMutex.Unlock()
}

func GetPluginExtendedList() []string {
	modulelistsMutex.RLock()
	defer modulelistsMutex.RUnlock()

	if pluginExtendedCommandList == nil {
		panic(errors.New("Tried to get plugin extended list before cache#SetPluginExtendedList() was called"))
	}

	return pluginExtendedCommandList
}

func SetTriggerPluginList(l []string) {
	modulelistsMutex.Lock()
	triggerPluginCommandList = l
	modulelistsMutex.Unlock()
}

func GetTriggerPluginList() []string {
	modulelistsMutex.RLock()
	defer modulelistsMutex.RUnlock()

	if triggerPluginCommandList == nil {
		panic(errors.New("Tried to get trigger plugin list before cache#SetTriggerPluginList() was called"))
	}

	return triggerPluginCommandList
}
