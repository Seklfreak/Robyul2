package plugins

import (
	"bytes"
	"cloud.google.com/go/translate"
	"context"
	"errors"
	"fmt"
	"github.com/Jeffail/gabs"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	"golang.org/x/text/language"
	"google.golang.org/api/option"
	"net/http"
	"strings"
)

type Translator struct {
	ctx    context.Context
	client *translate.Client
}

const (
	googleTranslateHexColor string = "#4285f4"
	naverTranslateEndpoint  string = "http://labspace.naver.com/api/n2mt/translate"
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
		option.WithAPIKey(helpers.GetConfig().Path("google.api_key").Data().(string)),
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

	translatedNaverResult := ""
	if (strings.ToLower(source.String()) == "en" || strings.ToLower(source.String()) == "ko") && (strings.ToLower(target.String()) == "en" || strings.ToLower(target.String()) == "ko") {
		fullInputText := ""
		for _, partInputText := range parts[2:] {
			fullInputText += " " + partInputText
		}
		fullInputText = strings.Trim(fullInputText, " ")
		targetInput := ""
		var targetInputs []string
		for _, word := range strings.Split(fullInputText, " ") {
			if len(targetInput)+len(word)+1 < 200 {
				targetInput += " " + word
			} else {
				targetInputs = append(targetInputs, strings.Trim(targetInput, " "))
				targetInput = ""
				if len(word) < 200 {
					targetInput = word
				}
			}
		}
		targetInputs = append(targetInputs, strings.Trim(targetInput, " "))

		for _, partToTranslaste := range targetInputs {
			jsonData := fmt.Sprintf("{\"source\": \"%s\", \"target\": \"%s\", \"text\": \"%s\"}",
				strings.ToLower(source.String()),
				strings.ToLower(target.String()),
				strings.Replace(partToTranslaste, "\"", "'", -1),
			)
			client := &http.Client{}
			request, err := http.NewRequest("POST", naverTranslateEndpoint, bytes.NewBufferString(jsonData))
			if err != nil {
				helpers.SendError(msg, err)
				continue
			}
			request.Header.Set("User-Agent", helpers.DEFAULT_UA)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("x-naver-client-id", "labspace")
			response, err := client.Do(request)
			if err != nil {
				helpers.SendError(msg, err)
				continue
			}
			if response.StatusCode == 200 {
				defer response.Body.Close()

				resultParsed, err := gabs.ParseJSONBuffer(response.Body)
				if err != nil {
					helpers.SendError(msg, err)
					continue
				}
				translatedNaverResult += " " + resultParsed.Path("message.result.translatedText").Data().(string)
			} else {
				helpers.SendError(msg, errors.New(fmt.Sprintf("Unexpected status code from Naver API: %d", response.StatusCode)))
			}
		}
	}

	translateEmbed := &discordgo.MessageEmbed{
		Title:       helpers.GetTextF("plugins.translator.translation-embed-title", strings.ToUpper(source.String()), strings.ToUpper(target.String())),
		Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.translator.embed-footer")},
		Description: m,
		Color:       helpers.GetDiscordColorFromHex(googleTranslateHexColor),
		Fields:      []*discordgo.MessageEmbedField{},
	}
	if translatedNaverResult != "" {
		translateEmbed.Fields = append(translateEmbed.Fields, &discordgo.MessageEmbedField{
			Name:  helpers.GetText("plugins.translator.embed-title-alternative-naver"),
			Value: translatedNaverResult})
		translateEmbed.Footer = &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.translator.embed-footer-plus-naver")}
	}

	_, err = session.ChannelMessageSendEmbed(msg.ChannelID, translateEmbed)
	helpers.Relax(err)
}
