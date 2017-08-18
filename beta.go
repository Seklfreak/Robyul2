package main

import (
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/bwmarrin/discordgo"
)

var BETA_GUILDS = [...]string{
	"208673735580844032", // sekl's cord
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
				cache.GetLogger().WithField("module", "beta").Info("beta", "Leaving guild "+guild.ID+" ("+guild.Name+") because it didn't apply for the beta")
				session.GuildLeave(guild.ID)
			}
		}

		time.Sleep(10 * time.Second)
	}
}
