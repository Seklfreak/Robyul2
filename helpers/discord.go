package helpers

import (
    "errors"
    "github.com/bwmarrin/discordgo"
    "git.lukas.moe/sn0w/Karen/cache"
)

var botAdmins = []string{
    "157834823594016768", // 0xFADED#3237
    "165345731706748929", // Serraniel#8978
}

// IsBotAdmin checks if $id is in $botAdmins
func IsBotAdmin(id string) bool {
    for _, s := range botAdmins {
        if s == id {
            return true
        }
    }

    return false
}

// RequireAdmin only calls $cb if the author is an admin or has MANAGE_SERVER permission
func RequireAdmin(msg *discordgo.Message, cb Callback) {
    channel, e := cache.GetSession().Channel(msg.ChannelID)
    if e != nil {
        SendError(msg, errors.New("Cannot verify permissions"))
        return
    }

    guild, e := cache.GetSession().Guild(channel.GuildID)
    if e != nil {
        SendError(msg, errors.New("Cannot verify permissions"))
        return
    }

    if msg.Author.ID == guild.OwnerID {
        cb()
        return
    }

    // Check if role may manage server
    for _, role := range guild.Roles {
        if role.Permissions & 8 == 8 {
            cb()
            return
        }
    }

    cache.GetSession().ChannelMessageSend(msg.ChannelID, "You are not an admin :frowning:")
}
