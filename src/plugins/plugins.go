package plugins

import "github.com/bwmarrin/discordgo"

// Plugin interface to enforce a basic structure
type Plugin interface {
    // The name of this plugin
    Name() string

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
    msg *discordgo.Message,
    session *discordgo.Session,
    )
}

// List of plugin instances
var PluginList = []Plugin{
    About{},
    Stats{},
}

// CallBotPlugin iterates through the list of registered
// plugins and tries to guess whice one is the intended call
// Fist match wins.
func CallBotPlugin(command string, msg *discordgo.Message, session *discordgo.Session) {
    // Iterate over all plugins
    for _, plug := range PluginList {
        // Iterate over all commands of the current plugin
        for cmd := range plug.Commands() {
            if command == cmd {
                plug.Action(command, msg, session)
                break
            }
        }
    }
}

func GetPlugins() []Plugin {
    return PluginList
}