package main

import (
    d "github.com/bwmarrin/discordgo"
)

type ProxiedEventHandlers []interface{}

func ProxyAttachListeners(session *d.Session, handlers ProxiedEventHandlers) {
    for _, eventHandler := range handlers {
        session.AddHandler(eventHandler)
        session.AddHandler(func(session *d.Session, data interface{}) {

        })
    }
}
