package plugins

import (
    "github.com/bwmarrin/discordgo"
    "github.com/sn0w/Karen/utils"
)

type RandomCat struct{}

func (rc RandomCat) Commands() []string {
    return []string{
        "cat",
    }
}

func (rc RandomCat) Init(session *discordgo.Session) {

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
