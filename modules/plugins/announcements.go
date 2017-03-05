package plugins

import (
    "strings"
    
    "github.com/bwmarrin/discordgo"
)

// Announcement such as updates, downtimes...
type Announcement struct {}

// Commands that are availble to trigger an announcement
func (a *Announcement) Commands() []string {
    return []string {
        "announce",
    }
}

// Init func
func (a *Announcement) Init(s *discordgo.Session) {}

// Action of the announcement
func (a *Announcement) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    title := ""
    contentSplit := strings.Fields(content)
    subcommand := contentSplit[0]
    text := content[len(subcommand):]
    switch subcommand {
    case "update":
        title = ":loudspeaker: **UPDATE**"
    case "downtime":
        title = ":warning: **DOWNTIME**"
    case "maintenance":
        title = ":clock5: **MAINTENANCE**"
    }
    // Iterate through all joined guilds
    for _, guild := range session.State.Guilds {
        // Check if we have an announcement channel set for this guild
        if helpers.GuildSettingsGetCached(guild.ID).AnnouncementsEnabled {
            // Get the announcement channel id
            channelID := helpers.GuildSettingsGetCached(guild.ID).AnnouncementsChannel
            // Send the announce to the channel
            session.ChannelMessageSendEmbed(channelID, &discordgo.MessageEmbed{
                Title: title,
                Description: text,
                Color: 0x0FADED,
            })
        }
    }
}