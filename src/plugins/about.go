package plugins

import "github.com/bwmarrin/discordgo"

type About struct{}

func (a About) Name() string {
    return "About"
}

func (a About) Description() string {
    return "-"
}

func (a About) Commands() map[string]string {
    return map[string]string{
        "help" : "Get help",
        "h" : "Alias for help",
    }
}

func (a About) Action(command string, msg *discordgo.Message, session *discordgo.Session) {
}

func (a About) New() Plugin {
    return &About{}
}