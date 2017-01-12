package plugins

import (
    "github.com/bwmarrin/discordgo"
    "math/rand"
)

type Triggered struct{}

var triggeredImages = [...]string{
    "http://i.imgur.com/tcccxs9.png",
    "http://i.imgur.com/ntgix3j.png",
    "http://i.imgur.com/3iw4qPg.gif",
    "http://i.imgur.com/avHnbUZ.gif",
    "http://i.imgur.com/gJ00Rwg.png",
    "http://i.imgur.com/qju5UHg.jpg",
    "http://i.imgur.com/RV7IcG4.gif",
    "http://i.imgur.com/oOF8IXq.gif",
    "http://i.imgur.com/VUiPGKS.gifv",
    "http://i.imgur.com/ztsvV7K.gifv",
    "http://i.imgur.com/NOwS64t.png",
    "http://i.imgur.com/KBDdUqz.gifv",
    "http://i.imgur.com/pMHQabO.gifv",
    "http://i.imgur.com/UgYbH8O.jpg",
    "http://i.imgur.com/BkAeFbP.gif",
    "http://i.imgur.com/QWt5k4x.jpg",
}

func (t Triggered) Commands() []string {
    return []string{
        "triggered",
        "trigger",
    }
}

func (t Triggered) Init(session *discordgo.Session) {

}

func (t Triggered) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    session.ChannelMessageSend(msg.ChannelID, triggeredImages[rand.Intn(len(triggeredImages))])
}
