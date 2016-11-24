package main

import (
    "github.com/Jeffail/gabs"
    "github.com/ugjka/cleverbot-go"
)

var (
    cleverbotSession *cleverbot.Session
)

func GetConfig(path string) *gabs.Container {
    json, err := gabs.ParseJSONFile(path)

    if err != nil {
        panic(err)
    }

    return json
}

func SendToCleverbot(message string) string {
    if cleverbotSession == nil {
        cleverbotSession = cleverbot.New()
    }

    response, err := cleverbotSession.Ask(message)
    if err != nil {
        return "Error :frowning:\n```\n" + err.Error() + "\n```"
    }

    return response
}

