package plugins

import (
	"cloud.google.com/go/translate"
	"context"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	"golang.org/x/text/language"
	"google.golang.org/api/option"
	"strings"
)

type Translator struct {
	ctx    context.Context
	client *translate.Client
}

const (
	googleTranslateHexColor string = "#4285f4"
)

func (t *Translator) Commands() []string {
	return []string{
		"translator",
		"translate",
		"t",
	}
}

func (t *Translator) Init(session *discordgo.Session) {
	t.ctx = context.Background()

	client, err := translate.NewClient(
		t.ctx,
		option.WithAPIKey(helpers.GetConfig().Path("google.translate").Data().(string)),
	)
	helpers.Relax(err)
	t.client = client
}

func (t *Translator) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	// Assumed format: <lang_in> <lang_out> <text>
	parts := strings.Split(content, " ")

	if len(parts) < 3 {
		session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.translator.check_format"))
		return
	}

	source, err := language.Parse(parts[0])
	if err != nil {
		session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.translator.unknown_lang", parts[0]))
		return
	}

	target, err := language.Parse(parts[1])
	if err != nil {
		session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.translator.unknown_lang", parts[1]))
		return
	}

	translations, err := t.client.Translate(
		t.ctx,
		parts[2:],
		target,
		&translate.Options{
			Format: translate.Text,
			Source: source,
		},
	)

	if err != nil {
		//session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.translator.error"))
		helpers.SendError(msg, err)
		return
	}

	m := ""
	for _, piece := range translations {
		m += piece.Text + " "
	}

	translateEmbed := &discordgo.MessageEmbed{
		Title:       helpers.GetTextF("plugins.translator.translation-embed-title", strings.ToUpper(source.String()), strings.ToUpper(target.String())),
		Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.translator.embed-footer")},
		Description: m,
		Color:       helpers.GetDiscordColorFromHex(googleTranslateHexColor),
	}

	_, err = session.ChannelMessageSendEmbed(msg.ChannelID, translateEmbed)
	helpers.Relax(err)
}
