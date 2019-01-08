package helpers

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/bwmarrin/discordgo"
)

const (
	LEFT_ARROW_EMOJI  = "⬅"
	RIGHT_ARROW_EMOJI = "➡"
	X_EMOJI           = "🇽"
	NAV_NUMBERS       = "🔢"

	FIELD_MESSAGE_TYPE = iota
	IMAGE_MESSAGE_TYPE
)

// map of messageID to pagedEmbedMessage
var pagedEmbededMessages map[string]*pagedEmbedMessage
var validReactions map[string]bool

type pagedEmbedMessage struct {
	files               []*discordgo.File
	fullEmbed           *discordgo.MessageEmbed
	totalNumOfPages     int
	currentPage         int
	fieldsPerPage       int
	color               int
	messageID           string
	channelID           string
	userId              string //user who triggered the message
	msgType             int
	waitingForPageInput bool
}

func init() {
	pagedEmbededMessages = make(map[string]*pagedEmbedMessage)

	validReactions = map[string]bool{
		LEFT_ARROW_EMOJI:  true,
		RIGHT_ARROW_EMOJI: true,
		X_EMOJI:           true,
		NAV_NUMBERS:       true,
	}

}

// will remove all reactions from all paged embed messages.
//  mainly used on bot uninit to clean embeds
func RemoveReactionsFromPagedEmbeds() {
	// TODO: sync group? Without blocking bot shutdown
	for _, pagedEmbed := range pagedEmbededMessages {
		go func(pagedEmbed *pagedEmbedMessage) {
			cache.GetSession().MessageReactionsRemoveAll(pagedEmbed.channelID, pagedEmbed.messageID)
		}(pagedEmbed)
	}
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
		waitingForPageInput: false,
		fullEmbed:           embed,
		channelID:           msg.ChannelID,
		currentPage:         1,
		fieldsPerPage:       fieldsPerPage,
		totalNumOfPages:     int(math.Ceil(float64(len(embed.Fields)) / float64(fieldsPerPage))),
		userId:              msg.Author.ID,
	}

	pagedMessage.setupAndSendFirstMessage()

	pagedEmbededMessages[pagedMessage.messageID] = pagedMessage
	return nil
}

// SendPagedImageMessage creates the paged image messages
func SendPagedImageMessage(msg *discordgo.Message, msgSend *discordgo.MessageSend, fieldsPerPage int) error {
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
		waitingForPageInput: false,
		fullEmbed:           msgSend.Embed,
		channelID:           msg.ChannelID,
		currentPage:         1,
		fieldsPerPage:       fieldsPerPage,
		totalNumOfPages:     len(msgSend.Files),
		files:               msgSend.Files,
		userId:              msg.Author.ID,
		msgType:             IMAGE_MESSAGE_TYPE,
	}

	pagedMessage.setupAndSendFirstMessage()

	pagedEmbededMessages[pagedMessage.messageID] = pagedMessage
	return nil
}

// UpdateMessagePage will update the page based on the given direction and current page
//  reactions must be the left or right arrow
func (p *pagedEmbedMessage) UpdateMessagePage(reaction *discordgo.MessageReactionAdd) {

	// check for valid reaction
	if !validReactions[reaction.Emoji.Name] || reaction.UserID != p.userId {
		return
	}

	// check if user who made the embed message is closing it
	if X_EMOJI == reaction.Emoji.Name {
		delete(pagedEmbededMessages, reaction.MessageID)
		cache.GetSession().ChannelMessageDelete(p.channelID, p.messageID)
		return
	}

	// check if user wants to navigate to a specific page
	if NAV_NUMBERS == reaction.Emoji.Name {
		if p.waitingForPageInput {
			return
		}

		// if the page entered is valid and not already the current page, then update
		if page, err := p.getUserInputPage(); err == nil && p.currentPage != page {
			p.currentPage = page
		} else {
			return
		}
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
	if p.msgType == IMAGE_MESSAGE_TYPE {
		// image embeds can't be edited, need to delete and remate it
		cache.GetSession().ChannelMessageDelete(p.channelID, p.messageID)

		// if fields were sent with image embed, handle those
		if len(p.fullEmbed.Fields) > 0 {

			// get start and end fields based on current page and fields per page
			startField := (p.currentPage - 1) * p.fieldsPerPage
			endField := startField + p.fieldsPerPage
			if endField > len(p.fullEmbed.Fields) {
				endField = len(p.fullEmbed.Fields)
			}

			tempEmbed.Fields = tempEmbed.Fields[startField:endField]
		}

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
			Files: []*discordgo.File{{
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

	if p.msgType == IMAGE_MESSAGE_TYPE {

		// if fields were sent with image embed, handle those
		if len(p.fullEmbed.Fields) > 0 {

			// get start and end fields based on current page and fields per page
			startField := (p.currentPage - 1) * p.fieldsPerPage
			endField := startField + p.fieldsPerPage
			if endField > len(p.fullEmbed.Fields) {
				endField = len(p.fullEmbed.Fields)
			}

			tempEmbed.Fields = tempEmbed.Fields[startField:endField]
		}

		var buf bytes.Buffer
		newReader := io.TeeReader(p.files[p.currentPage-1].Reader, &buf)
		p.files[p.currentPage-1].Reader = &buf

		tempEmbed.Image.URL = fmt.Sprintf("attachment://%s", p.files[p.currentPage-1].Name)
		sentMessage, err = SendComplex(p.channelID, &discordgo.MessageSend{
			Embed: tempEmbed,
			Files: []*discordgo.File{{
				Name:   p.files[p.currentPage-1].Name,
				Reader: newReader,
			}},
		})
		if p.hasError(err) {
			return
		}

	} else {
		// reduce fields to the fields per page
		tempEmbed.Fields = tempEmbed.Fields[:p.fieldsPerPage]

		sentMessage, err = SendEmbed(p.channelID, tempEmbed)
		if p.hasError(err) {
			return
		}
	}

	p.messageID = sentMessage[0].ID
	p.addReactionsToMessage()
}

// getEmbedFooter is a simlple helper function to return the footer for the embed message
func (p *pagedEmbedMessage) getEmbedFooter() *discordgo.MessageEmbedFooter {
	var footerText string

	// check if embed had a footer, if so attach to page count
	if p.fullEmbed.Footer != nil && p.fullEmbed.Footer.Text != "" {
		footerText = fmt.Sprintf("Page: %d / %d | %s", p.currentPage, p.totalNumOfPages, p.fullEmbed.Footer.Text)
	} else {
		footerText = fmt.Sprintf("Page: %d / %d", p.currentPage, p.totalNumOfPages)
	}

	return &discordgo.MessageEmbedFooter{Text: footerText}
}

func (p *pagedEmbedMessage) addReactionsToMessage() {
	cache.GetSession().MessageReactionAdd(p.channelID, p.messageID, LEFT_ARROW_EMOJI)
	cache.GetSession().MessageReactionAdd(p.channelID, p.messageID, RIGHT_ARROW_EMOJI)

	if p.totalNumOfPages > 5 {
		cache.GetSession().MessageReactionAdd(p.channelID, p.messageID, NAV_NUMBERS)
	}

	cache.GetSession().MessageReactionAdd(p.channelID, p.messageID, X_EMOJI)
}

// getUserInputPage waits for the user to enter a page
func (p *pagedEmbedMessage) getUserInputPage() (int, error) {
	queryMsg, err := SendMessage(p.channelID, "<@"+p.userId+"> Which page would you like to open? <:blobidea:317047867036663809>")
	if err != nil {
		return 0, err
	}

	defer cache.GetSession().ChannelMessageDelete(queryMsg[0].ChannelID, queryMsg[0].ID)

	timeoutChan := make(chan int)
	go func() {
		time.Sleep(time.Second * 45)
		timeoutChan <- 0
	}()

	p.waitingForPageInput = true
	for {
		select {
		case userMsg := <-waitForUserMessage():

			// check for user who opened embed
			if userMsg.Author.ID != p.userId {
				continue
			}

			// get page number from user text
			re := regexp.MustCompile("[0-9]+")
			if userEnteredNum, err := strconv.Atoi(re.FindString(userMsg.Content)); err == nil {

				// delete user message and remove reaction
				go cache.GetSession().ChannelMessageDelete(userMsg.ChannelID, userMsg.ID)
				go cache.GetSession().MessageReactionRemove(p.channelID, p.messageID, NAV_NUMBERS, p.userId)

				p.waitingForPageInput = false
				if userEnteredNum > 0 && userEnteredNum <= p.totalNumOfPages {

					return userEnteredNum, nil
				} else {
					return 0, errors.New("Page out of embed message range")
				}
			}
		case <-timeoutChan:
			go cache.GetSession().MessageReactionRemove(p.channelID, p.messageID, NAV_NUMBERS, p.userId)
			p.waitingForPageInput = false
			return 0, errors.New("Timed out")
		}
	}
}

// simple helper to check error, returns true if an error occured
//  helps with checking specifically for permissions errors
func (p *pagedEmbedMessage) hasError(err error) bool {
	if err == nil {
		return false
	}

	// delete from current embeds
	delete(pagedEmbededMessages, p.messageID)

	// check if error is a permissions error
	if err, ok := err.(*discordgo.RESTError); ok && err.Message.Code == discordgo.ErrCodeMissingPermissions {
		if p.msgType == IMAGE_MESSAGE_TYPE {
			SendMessage(p.channelID, GetText("bot.errors.no-embed-or-file"))
		} else {
			SendMessage(p.channelID, GetText("bot.errors.no-embed"))
		}
	} else {
		Relax(err)
	}

	return true
}

func waitForUserMessage() chan *discordgo.MessageCreate {
	out := make(chan *discordgo.MessageCreate)
	cache.GetSession().AddHandlerOnce(func(_ *discordgo.Session, e *discordgo.MessageCreate) {
		out <- e
	})
	return out
}
