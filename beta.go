package main

import (
    "github.com/bwmarrin/discordgo"
    "github.com/sn0w/Karen/logger"
    "time"
)

var BETA_GUILDS = [...]string{
    "259831440064118784", // FADED's Sandbox
    "180818466847064065", // Karen's Sandbox
    "172041631258640384", // P0WERPLANT
    "161637499939192832", // Coding Lounge
    "225168913808228352", // Emily's Space
}

// Automatically leaves guilds that are not registered beta testers
func autoLeaver(session *discordgo.Session) {
    for {
        for _, guild := range session.State.Guilds {
            match := false

            for _, betaGuild := range BETA_GUILDS {
                if guild.ID == betaGuild {
                    match = true
                    break
                }
            }

            if !match {
                logger.WARNING.L("beta", "Leaving guild " + guild.ID + " (" + guild.Name + ") because it didn't apply for the beta")
                session.GuildLeave(guild.ID)
            }
        }

        time.Sleep(10 * time.Second)
    }
}
