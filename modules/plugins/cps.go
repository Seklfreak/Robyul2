package plugins

import (
    "github.com/bwmarrin/discordgo"
    "git.lukas.moe/sn0w/Karen/metrics"
    "time"
    "fmt"
)

// Cps stands for Commands per second
type CPS struct {}

// Commands func
func (c *CPS) Commands() []string {
    return []string {
        "CPS",
        "cps",
    }
}

// Init func
func (c *CPS) Init(session *discordgo.Session) {}


// Action func
func (c *CPS) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    old := metrics.CommandsExecuted.Value()
    time.Sleep(time.Second)
    now := metrics.CommandsExecuted.Value()
    session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("%v CPS :fast_forward:", now-old))
}

