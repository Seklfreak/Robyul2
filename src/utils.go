package main

import (
    "github.com/Jeffail/gabs"
)

func GetConfig(path string) *gabs.Container {
    json, err := gabs.ParseJSONFile(path)

    if err != nil {
        panic(err)
    }

    return json
}

