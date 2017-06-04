package plugins

import (
    "github.com/bwmarrin/discordgo"
    "github.com/miekg/dns"
    "fmt"
    "github.com/Seklfreak/Robyul2/helpers"
    "strings"
    "net"
)

type Dig struct{}

func (d *Dig) Commands() []string {
    return []string{
        "dig",
    }
}

func (d *Dig) Init(session *discordgo.Session) {
}

func (d *Dig) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    session.ChannelTyping(msg.ChannelID)

    args := strings.Fields(content)

    if len(args) < 2 {
        session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
        return
    }
    dnsIp := "8.8.8.8"
    if len(args) >= 3 {
        dnsIp = strings.Replace(args[2], "@", "", 1)
    }

    var lookupType uint16
    if k, ok := dns.StringToType[strings.ToUpper(args[1])]; ok {
        lookupType = k
    }
    if k, ok := dns.StringToClass[strings.ToUpper(args[1])]; ok {
        lookupType = k
    }
    if lookupType == 0 {
        lookupType = dns.TypeA
    }

    m := new(dns.Msg)
    m.SetQuestion(dns.Fqdn(args[0]), lookupType)

    in, err := dns.Exchange(m, dnsIp+":53")
    if err != nil {
        if err, ok := err.(*net.OpError); ok {
            session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.errors.general", err.Err.Error()))
            return
        } else {
            helpers.Relax(err)
        }
    }

    questionText := ""
    for _, question := range in.Question {
        questionText += question.String() + "\n"
    }
    if questionText == "" {
        questionText = "N/A"
    }

    answerText := ""
    for _, answer := range in.Answer {
        answerText += "`" + answer.String() + "`\n"
    }
    if answerText == "" {
        answerText = "N/A"
    }

    resultEmbed := &discordgo.MessageEmbed{
        Title:       fmt.Sprintf("Dig `%s`:", questionText),
        Description: answerText,
        Footer:      &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Server: %s", dnsIp)},
    }

    _, err = session.ChannelMessageSendEmbed(msg.ChannelID, resultEmbed)
    helpers.Relax(err)
}
