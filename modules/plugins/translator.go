package plugins

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"cloud.google.com/go/translate"
	"github.com/Jeffail/gabs"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	"golang.org/x/text/language"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

type Translator struct {
	ctx    context.Context
	client *translate.Client
}

const (
	googleTranslateHexColor = "#4285f4"
	naverTranslateEndpoint  = "https://papago.naver.com/apis/n2mt/translate"
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
	session.ChannelTyping(msg.ChannelID)
	// Assumed format: <lang_in> <lang_out> <text>
	parts := strings.Fields(content)

	if len(parts) < 3 {
		helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.translator.check_format"))
		return
	}

	source, err := language.Parse(parts[0])
	if err != nil {
		helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.translator.unknown_lang_specific", parts[0]))
		return
	}

	target, err := language.Parse(parts[1])
	if err != nil {
		helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.translator.unknown_lang_specific", parts[1]))
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
		if err, ok := err.(*googleapi.Error); ok {
			if err.Code == 400 {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.translator.unknown_lang"))
				return
			}
		}
		//helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.translator.error"))
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
		fullInputText = strings.TrimSpace(strings.Replace(fullInputText, "\n", " ", -1))
		targetInput := ""
		var targetInputs []string
		for _, word := range strings.Fields(fullInputText) {
			if len(targetInput)+len(word)+1 < 200 {
				targetInput += " " + word
			} else {
				targetInputs = append(targetInputs, strings.TrimSpace(targetInput))
				targetInput = ""
				if len(word) < 200 {
					targetInput = word
				}
			}
		}
		targetInputs = append(targetInputs, strings.TrimSpace(targetInput))

		for _, partToTranslaste := range targetInputs {
			data := url.Values{}
			decoded, err := base64.StdEncoding.DecodeString("rlWxnJA0VwczLJkmZSwiZGljdERpc3BsYXkiOjUsInNvdXJjZSI6ImVuIiwidGFyZ2V0Ijoia28iLCJ0ZXh0IjoiQUFBQUFBQUFBQSJ9")
			helpers.Relax(err)
			jsonData := string(decoded)
			jsonData = strings.Replace(jsonData, "\"dictDisplay\":5", "\"dictDisplay\":0", -1)
			jsonData = strings.Replace(jsonData, "\"source\":\"en\"", fmt.Sprintf("\"source\":\"%s\"", source.String()), -1)
			jsonData = strings.Replace(jsonData, "\"target\":\"ko\"", fmt.Sprintf("\"target\":\"%s\"", target.String()), -1)
			jsonData = strings.Replace(jsonData, "\"text\":\"AAAAAAAAAA\"", fmt.Sprintf("\"text\":\"%s\"", strings.Replace(partToTranslaste, "\"", "'", -1)), -1)
			data.Set("data", base64.StdEncoding.EncodeToString([]byte(jsonData)))

			client := &http.Client{}
			request, err := http.NewRequest("POST", naverTranslateEndpoint, bytes.NewBufferString(data.Encode()))
			if err != nil {
				helpers.SendError(msg, err)
				continue
			}
			request.Header.Set("User-Agent", helpers.DEFAULT_UA)
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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
				translatedNaverResult += " " + resultParsed.Path("translatedText").Data().(string)
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

	_, err = helpers.SendEmbed(msg.ChannelID, translateEmbed)
	helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
}
