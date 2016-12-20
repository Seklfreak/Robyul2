package main

import (
    d "github.com/bwmarrin/discordgo"
)

// Alias of interface slice for better readability
type ProxiedEventHandlers []interface{}

// Attaches listeners to passed events *and* registers a proxy event to all of them.
// Might be used for advanced stats in the future
func ProxyAttachListeners(session *d.Session, handlers ProxiedEventHandlers) {
    for _, eventHandler := range handlers {
        session.AddHandler(eventHandler)
        //session.AddHandler(func(session *d.Session, data interface{}) {})
    }
}
