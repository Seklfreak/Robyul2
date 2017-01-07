package plugins

import (
    "github.com/bwmarrin/discordgo"
    "github.com/sn0w/Karen/cache"
    "github.com/sn0w/Karen/helpers"
    "github.com/sn0w/Karen/metrics"
    "strings"
)

// Plugin interface to enforce a basic structure
type Plugin interface {
    // List of commands and aliases
    Commands() []string

    // Plugin constructor
    Init(session *discordgo.Session)

    // Action to execute on message receive
    Action(
    command string,
    content string,
    msg *discordgo.Message,
    session *discordgo.Session,
    )
}

type TriggerPlugin interface {
    Init(session *discordgo.Session)
    Action(msg *discordgo.Message, session *discordgo.Session)
}

// PluginList is the list of active plugins
var PluginList = []Plugin{
    Avatar{},
    About{},
    Stats{},
    Ping{},
    Invite{},
    Giphy{},
    Google{},
    RandomCat{},
    Stone{},
    Roll{},
    Reminders{},
    &Music{},
    FML{},
    UrbanDict{},
    Weather{},
    Minecraft{},
    XKCD{},
}

// TriggerPluginList is the list of plugins that activate on normal chat
var TriggerPluginList = []TriggerPlugin{}

// CallBotPlugin iterates through the list of registered
// plugins and tries to guess which one is the intended call
// Fist match wins.
//
// command - The command that triggered this execution
// content - The content without command
// msg     - The message object
// session - The discord session
func CallBotPlugin(command string, content string, msg *discordgo.Message) {
    // Iterate over all plugins
    for _, plug := range PluginList {
        // Iterate over all commands of the current plugin
        for _, cmd := range plug.Commands() {
            if command == cmd {
                go safePluginCall(command, strings.TrimSpace(content), msg, plug)
                break
            }
        }
    }
}

// CallTriggerPlugins iterates through all trigger plugins
// and calls *all* of them (async).
//
// msg     - The message that triggered the execution
// session - The discord session
func CallTriggerPlugins(msg *discordgo.Message) {
    // Iterate over all plugins
    for _, plug := range TriggerPluginList {
        go func(plugin TriggerPlugin) {
            defer helpers.RecoverDiscord(msg)
            plugin.Action(msg, cache.GetSession())
        }(plug)
    }
}

// Wrapper that catches any panics from plugins
// Arguments: Same as CallBotPlugin().
func safePluginCall(command string, content string, msg *discordgo.Message, plug Plugin) {
    defer helpers.RecoverDiscord(msg)
    metrics.CommandsExecuted.Add(1)
    plug.Action(command, content, msg, cache.GetSession())
}
