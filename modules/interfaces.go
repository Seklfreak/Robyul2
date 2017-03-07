package modules

import "github.com/bwmarrin/discordgo"

type BaseModule interface{}

type Plugin interface {
    BaseModule

    Commands() []string

    Init(session *discordgo.Session)

    Action(
        command string,
        content string,
        msg *discordgo.Message,
        session *discordgo.Session,
    )
}

type TriggerPlugin interface {
    BaseModule

    Triggers() []string
    Response(trigger string, content string) string
}
