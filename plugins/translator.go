package plugins

import (
    "github.com/bwmarrin/discordgo"
)

type Translator struct{}

func (t *Translator) Commands() []string {
    return []string{
        "translator",
        "translate",
        "t",
    }
}

func (t *Translator) Init(session *discordgo.Session) {

}

func (t *Translator) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
}
