package helpers

import (
	"errors"
	"fmt"
	"math"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/bwmarrin/discordgo"
)

const (
	LEFT_ARROW_EMOJI  = "⬅"
	RIGHT_ARROW_EMOJI = "➡"
	X_EMOJI           = "🇽"
)

// map of messageID to pagedEmbedMessage
var pagedEmbededMessages map[string]*pagedEmbedMessage

type pagedEmbedMessage struct {
	messageID       string
	fullEmbed       *discordgo.MessageEmbed
	channelID       string
	totalNumOfPages int
	currentPage     int
	fieldsPerPage   int
	color           int
	userId          string //user who triggered the message
}

func init() {
	pagedEmbededMessages = make(map[string]*pagedEmbedMessage)
}

// GetPagedMessage will return the paged message if there is one, otherwill will return nil
func GetPagedMessage(messageID string) *pagedEmbedMessage {
	pagedMessaged, _ := pagedEmbededMessages[messageID]
	return pagedMessaged
}

// CreatePagedMessage creates the paged messages
func SendPagedMessage(msg *discordgo.Message, embed *discordgo.MessageEmbed, fieldsPerPage int) error {

	// if there aren't multiple fields to be paged through, or if the amount of fields is less than the requested fields per page
	//  just send a normal embed
	if len(embed.Fields) < 2 || len(embed.Fields) <= fieldsPerPage {
		SendEmbed(msg.ChannelID, embed)
		return nil
	}

	// fields per page can not be less than 1
	if fieldsPerPage < 1 {
		return errors.New("fieldsPerPage can not be less than 1")
	}

	// create paged message
	pagedMessage := &pagedEmbedMessage{
		fullEmbed:       embed,
		channelID:       msg.ChannelID,
		currentPage:     1,
		fieldsPerPage:   fieldsPerPage,
		totalNumOfPages: int(math.Ceil(float64(len(embed.Fields)) / float64(fieldsPerPage))),
		userId:          msg.Author.ID,
	}

	pagedMessage.setupAndSendFirstMessage()

	pagedEmbededMessages[pagedMessage.messageID] = pagedMessage
	return nil
}

// UpdateMessagePage will update the page based on the given direction and current page
//  reactions must be the left or right arrow
func (p *pagedEmbedMessage) UpdateMessagePage(reaction *discordgo.MessageReactionAdd) {
	// check for valid reaction
	if LEFT_ARROW_EMOJI != reaction.Emoji.Name && RIGHT_ARROW_EMOJI != reaction.Emoji.Name && X_EMOJI != reaction.Emoji.Name {
		return
	}

	// check if user who made the embed message is closing it
	if reaction.UserID == p.userId && X_EMOJI == reaction.Emoji.Name {
		fmt.Println("delete message")
		fmt.Println("user id: ", p.userId)
		delete(pagedEmbededMessages, reaction.MessageID)
		cache.GetSession().ChannelMessageDelete(p.channelID, p.messageID)
		return
	}

	// update current page based on direction
	if LEFT_ARROW_EMOJI == reaction.Emoji.Name {
		p.currentPage--
		if p.currentPage == 0 {
			p.currentPage = p.totalNumOfPages
		}
	}
	if RIGHT_ARROW_EMOJI == reaction.Emoji.Name {
		p.currentPage++
		if p.currentPage > p.totalNumOfPages {
			p.currentPage = 1
		}
	}

	tempEmbed := &discordgo.MessageEmbed{}
	*tempEmbed = *p.fullEmbed

	// get start and end fields based on current page and fields per page
	startField := (p.currentPage - 1) * p.fieldsPerPage
	endField := startField + p.fieldsPerPage
	if endField > len(p.fullEmbed.Fields) {
		endField = len(p.fullEmbed.Fields)
	}

	// updated stats
	tempEmbed.Fields = tempEmbed.Fields[startField:endField]
	tempEmbed.Footer = p.getEmbedFooter()
	EditEmbed(p.channelID, p.messageID, tempEmbed)

	// may return error due to permissions, don't need to catch it
	cache.GetSession().MessageReactionRemove(reaction.ChannelID, reaction.MessageID, reaction.Emoji.Name, reaction.UserID)
}

// setupAndSendFirstMessage
func (p *pagedEmbedMessage) setupAndSendFirstMessage() {

	// copy the embeded message so changes can be made to it
	tempEmbed := &discordgo.MessageEmbed{}
	*tempEmbed = *p.fullEmbed

	// set footer which will hold information about the page it is on
	tempEmbed.Footer = p.getEmbedFooter()

	// reduce fields to the fields per page
	tempEmbed.Fields = tempEmbed.Fields[:p.fieldsPerPage]

	sentMessage, err := SendEmbed(p.channelID, tempEmbed)
	if err != nil {
		// should probably handle this at some point lul
		return
	}
	p.messageID = sentMessage[0].ID

	cache.GetSession().MessageReactionAdd(p.channelID, p.messageID, LEFT_ARROW_EMOJI)
	cache.GetSession().MessageReactionAdd(p.channelID, p.messageID, RIGHT_ARROW_EMOJI)
	cache.GetSession().MessageReactionAdd(p.channelID, p.messageID, X_EMOJI)
}

// getEmbedFooter is a simlple helper function to return the footer for the embed message
func (p *pagedEmbedMessage) getEmbedFooter() *discordgo.MessageEmbedFooter {
	return &discordgo.MessageEmbedFooter{
		Text: fmt.Sprintf("Page: %d / %d", p.currentPage, p.totalNumOfPages),
	}
}
