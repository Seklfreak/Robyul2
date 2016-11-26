package models

import "github.com/bwmarrin/discordgo"

type GuildSettings struct {
    Guild    *discordgo.Guild
    Settings []GuildSetting
}

type GuildSetting struct {
    Key   string
    Value string
}