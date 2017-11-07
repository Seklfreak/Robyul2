package plugins

import (
	"runtime/pprof"
	"strings"

	"bytes"

	"bufio"

	"fmt"

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
	helpers.RequireBotAdmin(msg, func() {
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
		}

		return
	})
	return
}
