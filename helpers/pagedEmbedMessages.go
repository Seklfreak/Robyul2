package helpers

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"

	"sync"

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
	files           []*discordgo.File
	fullEmbed       *discordgo.MessageEmbed
	totalNumOfPages int
	currentPage     int
	fieldsPerPage   int
	color           int
	messageID       string
	channelID       string
	userId          string //user who triggered the message
	msgType         string // "image" - will cause the embed to page through the files instead of fields
}

func init() {
	pagedEmbededMessages = make(map[string]*pagedEmbedMessage)
}

// will remove all reactions from all paged embed messages.
//  mainly used on bot uninit to clean embeds
func RemoveReactionsFromPagedEmbeds() {
	var syncGroup sync.WaitGroup
	for _, pagedEmbed := range pagedEmbededMessages {
		syncGroup.Add(1)
		go func() {
			cache.GetSession().MessageReactionsRemoveAll(pagedEmbed.channelID, pagedEmbed.messageID)
			syncGroup.Done()
		}()
	}
	// make sure all reactions get removed before bot exits
	syncGroup.Wait()
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

// SendPagedImageMessage creates the paged image messages
func SendPagedImageMessage(msg *discordgo.Message, msgSend *discordgo.MessageSend) error {
	if msgSend.Embed == nil {
		return errors.New("parameter msgSend must contain an embed")
	}

	// make sure the image url is set to the name of the first file incease it wasn't set
	msgSend.Embed.Image.URL = fmt.Sprintf("attachment://%s", msgSend.Files[0].Name)

	// check if there are multiple files, not just send it normally
	if len(msgSend.Files) < 2 {
		SendComplex(msg.ChannelID, msgSend)
		return nil
	}

	// create paged message
	pagedMessage := &pagedEmbedMessage{
		fullEmbed:       msgSend.Embed,
		channelID:       msg.ChannelID,
		currentPage:     1,
		fieldsPerPage:   1,
		totalNumOfPages: len(msgSend.Files),
		files:           msgSend.Files,
		userId:          msg.Author.ID,
		msgType:         "image",
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

	// updated stats
	if p.msgType == "image" {
		// image embeds can't be edited, need to delete and remate it
		cache.GetSession().ChannelMessageDelete(p.channelID, p.messageID)

		// we need to split and reset the reader since a reader can only be used once
		var buf bytes.Buffer
		newReader := io.TeeReader(p.files[p.currentPage-1].Reader, &buf)
		p.files[p.currentPage-1].Reader = &buf

		// change the url of the embed to point to the new image
		tempEmbed.Image.URL = fmt.Sprintf("attachment://%s", p.files[p.currentPage-1].Name)

		// update footer and send message
		tempEmbed.Footer = p.getEmbedFooter()
		sentMessage, _ := SendComplex(p.channelID, &discordgo.MessageSend{
			Embed: tempEmbed,
			Files: []*discordgo.File{&discordgo.File{
				Name:   p.files[p.currentPage-1].Name,
				Reader: newReader,
			}},
		})

		// update map with new message id since
		originalmsgID := p.messageID
		p.messageID = sentMessage[0].ID
		p.addReactionsToMessage()
		pagedEmbededMessages[sentMessage[0].ID] = p
		delete(pagedEmbededMessages, originalmsgID)
	} else {

		// get start and end fields based on current page and fields per page
		startField := (p.currentPage - 1) * p.fieldsPerPage
		endField := startField + p.fieldsPerPage
		if endField > len(p.fullEmbed.Fields) {
			endField = len(p.fullEmbed.Fields)
		}

		tempEmbed.Fields = tempEmbed.Fields[startField:endField]
		tempEmbed.Footer = p.getEmbedFooter()
		EditEmbed(p.channelID, p.messageID, tempEmbed)

		cache.GetSession().MessageReactionRemove(reaction.ChannelID, reaction.MessageID, reaction.Emoji.Name, reaction.UserID)
	}

}

// setupAndSendFirstMessage
func (p *pagedEmbedMessage) setupAndSendFirstMessage() {
	var sentMessage []*discordgo.Message
	var err error

	// copy the embeded message so changes can be made to it
	tempEmbed := &discordgo.MessageEmbed{}
	*tempEmbed = *p.fullEmbed

	// set footer which will hold information about the page it is on
	tempEmbed.Footer = p.getEmbedFooter()

	if p.msgType == "image" {

		var buf bytes.Buffer
		newReader := io.TeeReader(p.files[p.currentPage-1].Reader, &buf)
		p.files[p.currentPage-1].Reader = &buf

		tempEmbed.Image.URL = fmt.Sprintf("attachment://%s", p.files[p.currentPage-1].Name)
		sentMessage, _ = SendComplex(p.channelID, &discordgo.MessageSend{
			Embed: tempEmbed,
			Files: []*discordgo.File{&discordgo.File{
				Name:   p.files[p.currentPage-1].Name,
				Reader: newReader,
			}},
		})

	} else {
		// reduce fields to the fields per page
		tempEmbed.Fields = tempEmbed.Fields[:p.fieldsPerPage]

		sentMessage, err = SendEmbed(p.channelID, tempEmbed)
	}

	if err != nil {
		// should probably handle this at some point lul
		return
	}
	p.messageID = sentMessage[0].ID

	p.addReactionsToMessage()
}

// getEmbedFooter is a simlple helper function to return the footer for the embed message
func (p *pagedEmbedMessage) getEmbedFooter() *discordgo.MessageEmbedFooter {
	return &discordgo.MessageEmbedFooter{
		Text: fmt.Sprintf("Page: %d / %d", p.currentPage, p.totalNumOfPages),
	}
}

func (p *pagedEmbedMessage) addReactionsToMessage() {
	cache.GetSession().MessageReactionAdd(p.channelID, p.messageID, LEFT_ARROW_EMOJI)
	cache.GetSession().MessageReactionAdd(p.channelID, p.messageID, RIGHT_ARROW_EMOJI)
	cache.GetSession().MessageReactionAdd(p.channelID, p.messageID, X_EMOJI)
}
