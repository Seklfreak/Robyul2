package plugins

import (
    "github.com/bwmarrin/discordgo"
    "../utils"
    "strings"
)

// Plugin interface to enforce a basic structure
type Plugin interface {
    // The name of this plugin
    Name() string

    // Hidden from !help ?
    HelpHidden() bool

    // A short but meaningful description
    Description() string

    // A map of commands and their usage
    // Example:
    // music => show info
    // start => ...
    // pause => ...
    // ...
    Commands() map[string]string

    // The action to execute if any command matches
    Action(
    command string,
    content string,
    msg *discordgo.Message,
    session *discordgo.Session,
    )

    // Initializer
    Init(session *discordgo.Session)
}

// List of plugin instances
var PluginList = []Plugin{
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
}

// CallBotPlugin iterates through the list of registered
// plugins and tries to guess whice one is the intended call
// Fist match wins.
func CallBotPlugin(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    // Iterate over all plugins
    for _, plug := range PluginList {
        // Iterate over all commands of the current plugin
        for cmd := range plug.Commands() {
            if command == cmd {
                go safePluginCall(command, strings.Trim(content, " "), msg, session, plug)
                break
            }
        }
    }
}

// Wrapper that catches any panics from plugins
func safePluginCall(command string, content string, msg *discordgo.Message, session *discordgo.Session, plug Plugin) {
    defer func() {
        err := recover()

        if err != nil {
            utils.SendError(session, msg, err)
        }
    }()

    utils.CCTV(session, msg)
    plug.Action(command, content, msg, session)
}

// Getter for this plugin list
func GetPlugins() []Plugin {
    return PluginList
}