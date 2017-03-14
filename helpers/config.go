package helpers

import "github.com/Jeffail/gabs"

// config Saves the bot-config
var config *gabs.Container

// LoadConfig loads the config from $path into $config
func LoadConfig(path string) {
    json, err := gabs.ParseJSONFile(path)

    if err != nil {
        panic(err)
    }

    config = json
}

// GetConfig is a config getter
func GetConfig() *gabs.Container {
    return config
}
