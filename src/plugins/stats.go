package plugins

import (
    "github.com/bwmarrin/discordgo"
    "os"
    "runtime"
    "github.com/cloudfoundry/gosigar"
    "strings"
    "strconv"
)

type Stats struct{}

func (s Stats) Name() string {
    return "Stats"
}

func (s Stats) Description() string {
    return "Shows some stats"
}

func (s Stats) Commands() map[string]string {
    return map[string]string{
        "stats" : "",
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

    m := []string{
        "Hi! Here are some stats about me :smiley:",
        "```",
        "--------------------- System Information ---------------------",
        "OS:       " + runtime.GOOS + " [Arch: " + runtime.GOARCH + "]",
        "Hostname: " + hostname,
        "Uptime:   " + strings.Trim(uptime.Format(), " "),
        "--------------------- RAM Information ------------------------",
        "Used heap:     " + u64tos(ram.HeapAlloc / 1048576) + " mb",
        "Reserved heap: " + u64tos(ram.TotalAlloc / 1048576) + " mb",
        "Overall:       " + u64tos(ram.Sys / 1048576) + " mb",
        "--------------------- Discord Information --------------------",
        "Connected Servers:       " + strconv.Itoa(len(guilds)),
        "Watching Channels:       " + strconv.Itoa(channels),
        "Users with access to me: " + strconv.Itoa(len(users)),
        "```",
    }

    session.ChannelMessageSend(msg.ChannelID, strings.Join(m, "\n"))
}

func u64tos(i uint64) string {
    return strconv.Itoa(int(i))
}