package main

import (
    "git.lukas.moe/sn0w/Karen/logger"
    "github.com/bwmarrin/discordgo"
    "time"
)

var BETA_GUILDS = [...]string{
    "180818466847064065", // FADED's Sandbox        (Me)
    "172041631258640384", // P0WERPLANT             (Me)
    "286474230634381312", // Ronin                  (Me/Serenity)
    "161637499939192832", // Coding Lounge          (Devsome)
    "225168913808228352", // Emily's Space          (Kaaz)
    "267186654312136704", // Shinda Sekai Sensen    (黒ゲロロロ)
    "244110097599430667", // S E K A I              (Senpai /「 ステ 」Abuse)
    "268279577598492672", // ZAKINET                (Senpai /「 ステ 」Abuse)
    "266326434505687041", // Bot Test               (quoththeraven)
    "267658193407049728", // Bot Creation           (quoththeraven)
    "106029722458136576", // Shadow Realm           (WhereIsMyAim)
    "268143270520029187", // Joel's Beasts          (Joel)
    "271346578189582339", // Universe Internet Ltd. (Inside24)
    "270353850085408780", // Turdy Republic         (Moopdedoop)
    "275720670045011968", // Omurice                (Katsurice)
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
                logger.WARNING.L("beta", "Leaving guild "+guild.ID+" ("+guild.Name+") because it didn't apply for the beta")
                session.GuildLeave(guild.ID)
            }
        }

        time.Sleep(10 * time.Second)
    }
}
