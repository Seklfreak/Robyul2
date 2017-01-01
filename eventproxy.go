package main

import (
    d "github.com/bwmarrin/discordgo"
)

// ProxiedEventHandlers is an alias of a interface slice for better readability
type ProxiedEventHandlers []interface{}

// ProxyAttachListeners attaches listeners to passed events *and*
// registers a proxy event to all of them.
// Might be used for advanced stats in the future
func ProxyAttachListeners(session *d.Session, handlers ProxiedEventHandlers) {
    for _, eventHandler := range handlers {
        session.AddHandler(eventHandler)
    }
}
