package main

import (
    d "github.com/bwmarrin/discordgo"
    "fmt"
)

type ProxiedEventHandlers []interface{}

func ProxyAttachListeners(session *d.Session, handlers ProxiedEventHandlers) {
    for _, eventHandler := range handlers {
        session.AddHandler(eventHandler)
        session.AddHandler(func(session *d.Session, data interface{}) {
            // Do something with the proxied event
            fmt.Printf("%#v", data)
        })
    }
}
