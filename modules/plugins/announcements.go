package plugins

import (
    "github.com/bwmarrin/discordgo"
)

// Announcement such as updates, downtimes...
type Announcement struct {}

// Commands that are availble to trigger an announcement
func (a *Announcement) Commands() []string {
    return []string {
        "announce_u", // For updates
        "announce_d", // For downtimes
        "announce_m", // For maintenance
    }
}

// Init func
func (a *Announcement) Init(s *discordgo.Session) {}

// Action of the announcement
func (a *Announcement) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    title := ""
    switch command {
    case "anounce_u":
        title = ":loudspeaker: **UPDATE**"
    case "announce_d":
        title = ":warning: **DOWNTIME**"
    case "announce_m":
        title = ":clock5: **MAINTENANCE**"
    }
    for _, guild := range session.State.Guilds {
        // Always announce on #general
        chID := ""
        for _, ch := range guild.Channels {
            if ch.Name == "general" {
                chID = ch.ID
            }
        }
        session.ChannelMessageSendEmbed(chID, &discordgo.MessageEmbed{
            Title: title,
            Description: content,
            Color: 0x0FADED,
        })
    }
}