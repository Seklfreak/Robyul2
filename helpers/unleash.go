package helpers

import (
	"fmt"

	unleash "github.com/Unleash/unleash-client-go"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/davecgh/go-spew/spew"
	raven "github.com/getsentry/raven-go"
)

var (
	// UnleashInitialised will be set to true when it has been initialised
	UnleashInitialised = false
)

func FeatureEnabled(feature string, fallback bool) bool {
	if !UnleashInitialised {
		return fallback
	}

	return unleash.IsEnabled(feature, unleash.WithFallback(fallback))
}

// UnleashListener is our listener for Unleash events
type UnleashListener struct{}

// OnError logs errors
func (l UnleashListener) OnError(err error) {
	cache.GetLogger().WithField("module", "unleash").Error(err)
	raven.CaptureError(fmt.Errorf(spew.Sdump(err)), nil)
}

// OnWarning logs warnings
func (l UnleashListener) OnWarning(warning error) {
	cache.GetLogger().WithField("module", "unleash").Warn(warning)
}

// OnReady prints to the console when the repository is ready.
func (l UnleashListener) OnReady() {
}

// OnCount prints to the console when the feature is queried.
func (l UnleashListener) OnCount(name string, enabled bool) {
}

// OnSent prints to the console when the server has uploaded metrics.
func (l UnleashListener) OnSent(payload unleash.MetricsData) {
}

// OnRegistered prints to the console when the client has registered.
func (l UnleashListener) OnRegistered(payload unleash.ClientData) {
}
