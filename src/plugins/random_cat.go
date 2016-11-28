package plugins

import (
    "github.com/bwmarrin/discordgo"
    "../utils"
)

type RandomCat struct{}

func (rc RandomCat) Name() string {
    return "RandomCat"
}

func (rc RandomCat) Description() string {
    return "Get a random cat image"
}

func (rc RandomCat) Commands() map[string]string {
    return map[string]string{
        "cat" : "",
    }
}

func (rc RandomCat) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    const ENDPOINT = "http://random.cat/meow"

    session.ChannelTyping(msg.ChannelID)

    json := utils.GetJSON(ENDPOINT)
    session.ChannelMessageSend(
        msg.ChannelID,
        "MEOW! :smiley_cat: \n " + json.Path("file").Data().(string),
    )
}

func (rc RandomCat) New() Plugin {
    return &RandomCat{}
}
