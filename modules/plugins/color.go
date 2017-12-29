package plugins

import (
	"strings"

	"bytes"
	"fmt"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/ungerik/go-cairo"
)

type Color struct{}

func (c *Color) Commands() []string {
	return []string{
		"color",
		"colour",
	}
}

const (
	PicSize = 200
)

func (c *Color) Init(session *discordgo.Session) {

}

func (c *Color) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermColor) {
		return
	}

	session.ChannelTyping(msg.ChannelID)

	args := strings.Fields(content)
	if len(args) <= 0 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
		return
	}

	colorText := args[0]
	if !strings.HasPrefix(colorText, "#") {
		colorText = "#" + colorText
	}

	color, err := colorful.Hex(colorText)
	if err != nil {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
		return
	}

	surface := cairo.NewSurface(cairo.FORMAT_ARGB32, PicSize, PicSize)
	surface.SetSourceRGB(color.R, color.G, color.B)
	surface.Rectangle(0, 0, float64(PicSize), float64(PicSize))
	surface.Fill()
	pngBytes, _ := surface.WriteToPNGStream()

	_, err = helpers.SendComplex(
		msg.ChannelID, &discordgo.MessageSend{
			Content: fmt.Sprintf("<@%s> Color `%s`", msg.Author.ID, color.Hex()),
			Files: []*discordgo.File{
				{
					Name:   color.Hex() + ".png",
					Reader: bytes.NewReader(pngBytes),
				},
			},
		})
	helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
}
