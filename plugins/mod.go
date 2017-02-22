package plugins

import (
	"fmt"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	"regexp"
	"strings"
)

type Mod struct{}

func (m *Mod) Commands() []string {
	return []string{
		"cleanup",
	}
}

func (m *Mod) Init(session *discordgo.Session) {

}

func (m *Mod) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	helpers.RequireAdmin(msg, func() {
		regexMessageId := regexp.MustCompile(`^\d+$`)

		switch command {
		case "cleanup":
			args := strings.Split(content, " ")
			if len(args) > 0 {
				switch args[0] {
				case "after": // [p]cleanup after <after message id> [<until message id>]
					if len(args) < 2 {
						session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
						return
					} else {
						afterMessageId := args[1]
						untilMessageId := ""
						if regexMessageId.MatchString(afterMessageId) == false {
							session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
							return
						}
						if len(args) >= 3 {
							untilMessageId = args[2]
							if regexMessageId.MatchString(untilMessageId) == false {
								session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
								return
							}
						}
						messagesToDeleteAfter, _ := session.ChannelMessages(msg.ChannelID, 100, "", afterMessageId)
						messagesToDeleteBefore := []*discordgo.Message{}
						if untilMessageId != "" {
							messagesToDeleteBefore, _ = session.ChannelMessages(msg.ChannelID, 100, "", untilMessageId)
						}
						messagesToDeleteIds := []string{msg.ID}
						for _, messageToDelete := range messagesToDeleteAfter {
							isExcluded := false
							for _, messageBefore := range messagesToDeleteBefore {
								if messageToDelete.ID == messageBefore.ID {
									isExcluded = true
								}
							}
							if isExcluded == false {
								messagesToDeleteIds = append(messagesToDeleteIds, messageToDelete.ID)
							}
						}
						if len(messagesToDeleteIds) <= 10 {
							err := session.ChannelMessagesBulkDelete(msg.ChannelID, messagesToDeleteIds)
							if err != nil {
								session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf(helpers.GetTextF("plugins.mod.deleting-messages-failed"), err.Error()))
							}
						} else {
							if helpers.ConfirmEmbed(msg.ChannelID, msg.Author, helpers.GetTextF("plugins.mod.deleting-message-bulkdelete-confirm", len(messagesToDeleteIds)), "✅", "🚫") == true {
								err := session.ChannelMessagesBulkDelete(msg.ChannelID, messagesToDeleteIds)
								if err != nil {
									session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.mod.deleting-messages-failed", err.Error()))
									return
								}
							}
							return
						}
					}
				}
			}
		}

	})
}
