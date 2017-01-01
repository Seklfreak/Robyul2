package helpers

import (
    "github.com/bwmarrin/discordgo"
    "errors"
)

var BOT_ADMINS = []string{
    "157834823594016768", // 0xFADED#3237
    "165345731706748929", // Serraniel#8978
}

func IsBotAdmin(id string) bool {
    for _, s := range BOT_ADMINS {
        if s == id {
            return true
        }
    }

    return false
}


// RequireAdmin only calls $cb if the author is an admin or has MANAGE_SERVER permission
func RequireAdmin(session *discordgo.Session, msg *discordgo.Message, cb Callback) {
    channel, e := session.Channel(msg.ChannelID)
    if e != nil {
        SendError(session, msg, errors.New("Cannot verify permissions"))
        return
    }

    guild, e := session.Guild(channel.GuildID)
    if e != nil {
        SendError(session, msg, errors.New("Cannot verify permissions"))
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

    session.ChannelMessageSend(msg.ChannelID, "You are not an admin :frowning:")
}
