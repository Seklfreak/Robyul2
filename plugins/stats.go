package plugins

import (
    "github.com/bwmarrin/discordgo"
    "github.com/dustin/go-humanize"
    "github.com/sn0w/Karen/version"
    "runtime"
    "strconv"
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

    session.ChannelMessageSendEmbed(msg.ChannelID, &discordgo.MessageEmbed{
        Color: 0x0FADED,
        Fields: []*discordgo.MessageEmbedField{
            // System
            {Name: "Bot Version", Value: version.BOT_VERSION, Inline: true},
            {Name: "GO Version", Value: runtime.Version(), Inline: true},
            {Name: "Build Time", Value: version.BUILD_TIME, Inline: true},

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
