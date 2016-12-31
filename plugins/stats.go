package plugins

import (
    "github.com/bwmarrin/discordgo"
    "github.com/cloudfoundry/gosigar"
    "github.com/dustin/go-humanize"
    "os"
    "runtime"
    "strconv"
    "strings"
)

type Stats struct{}

func (s Stats) Commands() []string {
    return []string{
        "stats",
    }
}

func (s Stats) Init(session *discordgo.Session) {

}

func (s Stats) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    // Count guilds, channels and users
    users := make(map[string]string)
    channels := 0
    guilds := session.State.Guilds

    for _, guild := range guilds {
        channels += len(guild.Channels)

        for _, u := range guild.Members {
            users[u.User.ID] = u.User.Username
        }
    }

    // Get RAM stats
    var ram runtime.MemStats
    runtime.ReadMemStats(&ram)

    // Get uptime
    uptime := sigar.Uptime{}
    uptime.Get()

    // Get hostname
    hostname, err := os.Hostname()
    if err != nil {
        hostname = "Unknown"
    }

    session.ChannelMessageSendEmbed(msg.ChannelID, &discordgo.MessageEmbed{
        Color:       0x0FADED,
        Fields: []*discordgo.MessageEmbedField{
            // System
            {Name: "Hostname", Value: hostname, Inline: true},
            {Name: "Uptime", Value: strings.TrimSpace(uptime.Format()), Inline: true},
            {Name: "GO Version", Value: runtime.Version(), Inline: true},

            // Bot
            {Name: "Used RAM", Value: humanize.Bytes(ram.Alloc) + "/" + humanize.Bytes(ram.Sys), Inline: true},
            {Name: "Collected garbage", Value: humanize.Bytes(ram.TotalAlloc), Inline: true},
            {Name: "Running coroutines", Value: strconv.Itoa(runtime.NumGoroutine()), Inline: true},

            // Discord
            {Name: "Connected servers", Value: strconv.Itoa(len(guilds)), Inline: true},
            {Name: "Watching channels", Value: strconv.Itoa(channels), Inline: true},
            {Name: "Users with access to me", Value: strconv.Itoa(len(users)), Inline: true},

            // Link
            {Name: "Want more stats and awesome graphs?", Value: "Visit [stats.meetkaren.xyz](https://stats.meetkaren.xyz)", Inline: false},
        },
    })
}
