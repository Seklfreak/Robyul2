package plugins

import (
    "github.com/bwmarrin/discordgo"
    "fmt"
    "math/rand"
)

type Bolizei struct{}

func (b Bolizei) Name() string {
    return "Bolizei"
}

func (b Bolizei) HelpHidden() bool {
    return true
}

func (b Bolizei) Description() string {
    return ""
}

func (b Bolizei) Commands() map[string]string {
    return map[string]string{
        "polizei" : "",
        "bolizei" : "",
        "wiiuu" : "",
    }
}

func (b Bolizei) Init(session *discordgo.Session) {

}

func (b Bolizei) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    kenny := []string{
        "VERHAFTEN!",
        "EINKNASTEN!",
        "ICH HAB BOLIZEI!",
    }

    channel, err := session.Channel(msg.ChannelID)

    if err == nil {
        guild, err := session.Guild(channel.GuildID)

        if err == nil {
            if guild.ID == "161637499939192832" {
                session.ChannelMessageSend(
                    msg.ChannelID,
                    fmt.Sprintf(
                        ":oncoming_police_car: <@130291394404155392> %s \n %s",
                        kenny[rand.Intn(len(kenny))],
                        "https://youtu.be/PNjG22Gbo6U?t=44s",
                    ),
                )
            }
        }
    }
}