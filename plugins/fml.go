package plugins

import (
    "github.com/PuerkitoBio/goquery"
    "github.com/bwmarrin/discordgo"
    "github.com/sn0w/Karen/helpers"
)

type FML struct{}

func (f FML) Commands() []string {
    return []string{
        "fml",
    }
}

func (f FML) Init(session *discordgo.Session) {

}

func (f FML) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    doc, err := goquery.NewDocument("http://www.fmylife.com/random")
    helpers.Relax(err)

    node := doc.Find(".fmllink").Get(0).FirstChild.Data

    session.ChannelMessageSend(msg.ChannelID, "```\n" + node + "\n```")
}
