package plugins

import (
	"context"
	"runtime/pprof"
	"strings"

	vision "cloud.google.com/go/vision/apiv1"

	"bytes"

	"bufio"

	"fmt"

	"os"

	"strconv"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

type Debug struct{}

func (d *Debug) Commands() []string {
	return []string{
		"debug",
	}
}

func (d *Debug) Init(session *discordgo.Session) {
}

func (d *Debug) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	helpers.RequireRobyulMod(msg, func() {
		args := strings.Fields(content)

		if len(args) <= 0 {
			return
		}

		switch args[0] {
		case "goroutines", "goroutine":
			session.ChannelTyping(msg.ChannelID)

			var buf bytes.Buffer
			writer := bufio.NewWriter(&buf)
			err := pprof.Lookup("goroutine").WriteTo(writer, 1)
			helpers.Relax(err)
			err = writer.Flush()
			helpers.Relax(err)

			_, err = helpers.SendComplex(
				msg.ChannelID, &discordgo.MessageSend{
					Content: fmt.Sprintf("<@%s> Your request is ready:", msg.Author.ID),
					Files: []*discordgo.File{
						{
							Name:   "robyul-goroutines-dump.txt",
							Reader: bytes.NewReader(buf.Bytes()),
						},
					},
				})
			helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
			return
		case "cloudvision":
			session.ChannelTyping(msg.ChannelID)

			var imageUrl string
			if len(args) >= 2 {
				imageUrl = args[1]
			}
			if msg.Attachments != nil && len(msg.Attachments) > 0 {
				imageUrl = msg.Attachments[0].URL
			}
			if imageUrl == "" {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
				return
			}

			data, err := helpers.NetGetUAWithError(imageUrl, helpers.DEFAULT_UA)
			helpers.Relax(err)

			ctx := context.Background()
			os.Setenv(
				"GOOGLE_APPLICATION_CREDENTIALS",
				helpers.GetConfig().Path("google.client_credentials_json_location").Data().(string),
			)

			visionClient, err := vision.NewImageAnnotatorClient(ctx)
			helpers.Relax(err)

			image, err := vision.NewImageFromReader(bytes.NewReader(data))
			helpers.Relax(err)

			labelsData, err := visionClient.DetectLabels(ctx, image, nil, 15)
			helpers.Relax(err)

			var labelsText string
			if labelsData != nil {
				for _, labelData := range labelsData {
					labelsText += labelData.GetDescription() + " " + strconv.FormatFloat(float64(labelData.GetScore()), 'f', 2, 64) + ", "
				}
				labelsText = strings.TrimSuffix(labelsText, ", ")
			}

			webData, err := visionClient.DetectWeb(ctx, image, nil)
			helpers.Relax(err)

			var webText string
			if webData != nil {
				webText += "Best Guess: "
				for _, webDataEntry := range webData.BestGuessLabels {
					webText += webDataEntry.GetLabel() + ", "
				}
				webText = strings.TrimSuffix(webText, ", ")
				webText += "\nWeb Entities: "
				for _, webDataEntry := range webData.WebEntities {
					webText += webDataEntry.GetDescription() + " " + strconv.FormatFloat(float64(webDataEntry.GetScore()), 'f', 2, 64) + ", "
				}
				webText = strings.TrimSuffix(webText, ", ")
			}

			/*
				cropHints, err := visionClient.CropHints(ctx, image, nil)
				helpers.Relax(err)
			*/

			safeData, err := visionClient.DetectSafeSearch(ctx, image, nil)
			helpers.Relax(err)

			_, err = helpers.SendMessage(msg.ChannelID, fmt.Sprintf("__**Result:**__\n"+
				"**Labels:** %s\n"+
				"**Web:** %s\n"+
				"**Safe Search:** %s\n",
				labelsText,
				webText,
				safeData.String()))
			helpers.Relax(err)
			return
		case "storage":
			session.ChannelTyping(msg.ChannelID)

			if len(args) < 2 {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
				return
			}

			info, err := helpers.RetrieveFileInformation(args[1])
			helpers.Relax(err)

			author, err := helpers.GetUser(info.UserID)
			if err != nil {
				author = new(discordgo.User)
				author.Username = "N/A"
				author.ID = info.UserID
			}

			file, err := helpers.RetrieveFile(args[1])
			helpers.Relax(err)

			_, err = helpers.SendFile(msg.ChannelID, info.Filename, bytes.NewReader(file), fmt.Sprintf(
				"`%s` (`%s`) `%s` by `%s#%s` (`#%s`)\nRetrieved %d times:",
				info.Filename, args[1], info.MimeType, author.Username, author.Discriminator, author.ID,
				info.RetrievedCount,
			))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			return
		}

		return
	})
	return
}
