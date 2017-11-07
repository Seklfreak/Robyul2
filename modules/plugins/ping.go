package plugins

import (
	"strconv"
	"time"

	"strings"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

type Ping struct{}

func (p *Ping) Commands() []string {
	return []string{
		"ping",
	}
}

var (
	pingMessage string
)

func (p *Ping) Init(session *discordgo.Session) {
	pingMessage = helpers.GetText("plugins.ping.message")
	session.AddHandler(p.OnMessage)
}

func (p *Ping) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	_, err := helpers.SendMessage(msg.ChannelID, pingMessage+" ~ "+strconv.FormatInt(time.Now().UnixNano(), 10))
	helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
}

func (p *Ping) OnMessage(session *discordgo.Session, message *discordgo.MessageCreate) {
	if message.Author.ID != session.State.User.ID {
		return
	}

	if !strings.HasPrefix(message.Content, pingMessage+" ~ ") {
		return
	}

	textUnixNano := strings.Replace(message.Content, pingMessage+" ~ ", "", 1)

	parsedUnixNano, err := strconv.ParseInt(textUnixNano, 10, 64)
	if err != nil {
		return
	}

	gatewayTaken := time.Duration(time.Now().UnixNano() - parsedUnixNano)

	text := strings.Replace(message.Content, " ~ "+textUnixNano, "", 1) + "\nGateway Time: " + gatewayTaken.String()

	started := time.Now()
	helpers.EditMessage(message.ChannelID, message.ID, text)
	apiTaken := time.Since(started)

	text = text + "\nHTTP API Time (edit message): " + apiTaken.String()
	helpers.EditMessage(message.ChannelID, message.ID, text)

}
