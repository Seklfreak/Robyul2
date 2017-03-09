package plugins

import (
    "fmt"
    "time"
    "strings"

    Logger "git.lukas.moe/sn0w/Karen/logger"
    "github.com/bwmarrin/discordgo"
)

// WhoIs command
type WhoIs struct{}

// Commands for WhoIs
func (w *WhoIs) Commands() []string {
    return []string{
        "whois",
    }
}

// Init func
func (w *WhoIs) Init(s *discordgo.Session) {}

// Action will return info about the first @user
func (w *WhoIs) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    // Check if the msg contains at least 1 mention
    if len(msg.Mentions) == 0 {
        session.ChannelMessageSend(msg.ChannelID, "you need to @mention someone")
        return
    }

    // Get channel info
    channel, err := session.Channel(msg.ChannelID)
    if err != nil {
        Logger.PLUGIN.L("whois", err.Error())
        return
    }

    // Guild info
    guild, err := session.Guild(channel.GuildID)
    if err != nil {
        Logger.PLUGIN.L("whois", err.Error())
        return
    }

    // Get the member object for the @user
    target, err := session.GuildMember(guild.ID, msg.Mentions[0].ID)
    if err != nil {
        Logger.PLUGIN.L("whois", err.Error())
        return
    }

    // The @user's avatar url
    avatarURL := func(width int) string {
        return fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.webp?size=%v", target.User.ID, target.User.Avatar, width)
    }

    // Parses a string -> time.Time
    // tim must be RFC3339 formatted (works with discord)
    // i.e:
    // 18-05-2017
    // Time since: XyXhXmXs -> see time.Duration.String() for more info on this
    parseTimeAndMakeItReadable := func(tim string) string {
        t, _ := time.Parse(time.RFC3339, tim)
        date := t.Format("02-01-2006")
        date += "\n"
        duration := time.Since(t)
        date += "Time since: " + duration.String()
        return date
    }

    // The roles name of the @user
    roles := []string{}
    for _, grole := range guild.Roles {
        for _, urole := range target.Roles {
            if urole == grole.ID {
                roles = append(roles, grole.Name)
            }
        }
    }

    session.ChannelMessageSendEmbed(msg.ChannelID, &discordgo.MessageEmbed{
        Title: target.Nick + "#" + target.User.Discriminator,
        Image: &discordgo.MessageEmbedImage{
            // Make it 128x128 -> this may change
            URL:    avatarURL(128),
            Width:  128,
            Height: 128,
        },
        Color: 0x0FADED,
        Fields: []*discordgo.MessageEmbedField {
            {
                Name:   "Joined server",
                Value:  parseTimeAndMakeItReadable(target.JoinedAt),
                Inline: true,
            },
            {
                Name:   "Roles",
                Value:  strings.Join(roles, ","),
                Inline: true,
            },
            {
                Name:  "Avatar link",
                Value: avatarURL(1024),
            },
            {
                Name: "UserID",
                Value: target.User.ID,
            },
        },
    })
}
