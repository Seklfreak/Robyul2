package helpers

import (
    "git.lukas.moe/sn0w/Karen/cache"
    "github.com/bwmarrin/discordgo"
    "strings"
    "fmt"
    "time"
    "strconv"
)

const (
    DISCORD_EPOCH int64 = 1420070400000
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

func IsAdmin(msg *discordgo.Message) bool {
    channel, e := cache.GetSession().Channel(msg.ChannelID)
    if e != nil {
        return false
    }

    guild, e := cache.GetSession().Guild(channel.GuildID)
    if e != nil {
        return false
    }

    if msg.Author.ID == guild.OwnerID || IsBotAdmin(msg.Author.ID) {
        return true
    }

    // Check if role may manage server
    for _, role := range guild.Roles {
        if role.Permissions&8 == 8 {
            return true
        }
    }

    return false
}

// RequireAdmin only calls $cb if the author is an admin or has MANAGE_SERVER permission
func RequireAdmin(msg *discordgo.Message, cb Callback) {
    if !IsAdmin(msg) {
        cache.GetSession().ChannelMessageSend(msg.ChannelID, GetText("admin.no_permission"))
        return
    }

    cb()
}

func GetAvatarUrl(user *discordgo.User) string {
    return GetAvatarUrlWithSize(user, 1024)
}

func GetAvatarUrlWithSize(user *discordgo.User, size uint16) string {
    if user.Avatar == "" {
        return ""
    }

    avatarUrl := "https://cdn.discordapp.com/avatars/%s/%s.%s?size=%d"

    if strings.HasPrefix(user.Avatar, "a_") {
        return fmt.Sprintf(avatarUrl, user.ID, user.Avatar, "gif", size)
    }

    return fmt.Sprintf(avatarUrl, user.ID, user.Avatar, "webp", size)
}

func GetTimeFromSnowflake(id string) time.Time {
    iid, err := strconv.ParseInt(id, 10, 64)
    Relax(err)

    return time.Unix(((int64(iid)>>22)+DISCORD_EPOCH)/1000, 0).UTC()
}
