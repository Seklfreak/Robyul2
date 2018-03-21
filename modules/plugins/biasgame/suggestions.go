package biasgame

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"regexp"
	"strings"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/globalsign/mgo/bson"
	"github.com/mgutz/str"
	"github.com/nfnt/resize"
	"github.com/sethgrid/pester"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

const (
	IMAGE_SUGGESTION_CHANNEL = "424701783596728340"

	CHECKMARK_EMOJI    = "✅"
	X_EMOJI            = "❌"
	QUESTIONMARK_EMOJI = "❓"

	MAX_IMAGE_SIZE = 2000 // 2000x2000px
	MIN_IMAGE_SIZE = 150  // 150x150px
)

var suggestionQueue []*models.BiasGameSuggestionEntry
var suggestionEmbedMessageId string // id of the embed message where suggestions are accepted/denied
var genderFolderMap map[string]string
var exampleRoundPicId string
var suggestionQueueCountMessageId string

func initSuggestionChannel() {

	// when the bot starts, delete any past bot messages from the suggestion channel and make the embed
	var messagesToDelete []string
	messagesInChannel, _ := cache.GetSession().ChannelMessages(IMAGE_SUGGESTION_CHANNEL, 100, "", "", "")
	for _, msg := range messagesInChannel {
		messagesToDelete = append(messagesToDelete, msg.ID)
	}

	err := cache.GetSession().ChannelMessagesBulkDelete(IMAGE_SUGGESTION_CHANNEL, messagesToDelete)
	if err != nil {
		fmt.Println("Error deleting messages: ", err.Error())
	}

	// make a message on how to edit suggestions
	helpMessage := "```Editable Fields: name, group, gender, notes\nCommand: !edit {field} new field value...\n\nPlease add a note when denying suggestions.```"
	helpers.SendMessage(IMAGE_SUGGESTION_CHANNEL, helpMessage)

	// load unresolved suggestions and create the first embed
	loadUnresolvedSuggestions()
	updateSuggestionQueueCount()
	updateCurrentSuggestionEmbed()

	genderFolderMap = map[string]string{
		"boy":  BOYS_FOLDER_ID,
		"girl": GIRLS_FOLDER_ID,
	}
}

// processImageSuggestion
func ProcessImageSuggestion(msg *discordgo.Message, msgContent string) {
	invalidArgsMessage := "Invalid suggestion arguments. \n\n" +
		"Suggestion must be done with the following format:\n```!biasgame suggest [boy/girl] \"group name\" \"idol name\" [url to image]```\n" +
		"For Example:\n```!biasgame suggest girl \"PRISTIN\" \"Nayoung\" https://cdn.discordapp.com/attachments/420049316615553026/420056295618510849/unknown.png```\n\n"

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Panic: ", r)
			helpers.SendMessage(msg.ChannelID, invalidArgsMessage)
		}
	}()

	// ToArgv can panic, need to catch that
	suggestionArgs := str.ToArgv(msgContent)[1:]
	var suggestedImageUrl string

	// validate suggestion arg amount.
	if len(msg.Attachments) == 1 {
		if len(suggestionArgs) != 3 {
			helpers.SendMessage(msg.ChannelID, invalidArgsMessage)
			return
		}
		suggestedImageUrl = msg.Attachments[0].URL
	} else {
		if len(suggestionArgs) != 4 {
			helpers.SendMessage(msg.ChannelID, invalidArgsMessage)
			return
		}
		suggestedImageUrl = suggestionArgs[3]
	}

	// set gender to lowercase and check if its valid
	suggestionArgs[0] = strings.ToLower(suggestionArgs[0])
	if suggestionArgs[0] != "girl" && suggestionArgs[0] != "boy" {
		helpers.SendMessage(msg.ChannelID, invalidArgsMessage)
		return
	}

	// validate url image
	resp, err := pester.Get(suggestedImageUrl)
	if err != nil {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.suggestion.invalid-url"))
		return
	}
	defer resp.Body.Close()

	// make sure image is png or jpeg
	if resp.Header.Get("Content-type") != "image/png" && resp.Header.Get("Content-type") != "image/jpeg" {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.suggestion.not-png-or-jpeg"))
		return
	}

	// attempt to decode the image, if we can't there may be something wrong with the image submitted
	suggestedImage, _, errr := image.Decode(resp.Body)
	if errr != nil {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.suggestion.invalid-url"))
		fmt.Println("image decode error: ", err)
		return
	}

	// Check height and width are equal
	if suggestedImage.Bounds().Dy() != suggestedImage.Bounds().Dx() {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.suggestion.image-not-square"))
		return
	}

	// Validate size of image
	if suggestedImage.Bounds().Dy() > MAX_IMAGE_SIZE || suggestedImage.Bounds().Dy() < MIN_IMAGE_SIZE {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.suggestion.invalid-image-size"))
		return
	}

	// validate group and idol name have no double quotes or underscores
	if strings.ContainsAny(suggestionArgs[1]+suggestionArgs[2], "\"_") {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.suggestion.invalid-group-or-idol"))
		return
	}

	// send ty message
	fmt.Println(msg.Author.Mention())
	helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.biasgame.suggestion.thanks-for-suggestion", msg.Author.Mention()))

	// create suggetion
	suggestion := &models.BiasGameSuggestionEntry{
		UserID:     msg.Author.ID,
		ChannelID:  msg.ChannelID,
		Gender:     suggestionArgs[0],
		GrouopName: suggestionArgs[1],
		Name:       suggestionArgs[2],
		ImageURL:   suggestedImageUrl,
		GroupMatch: false,
		IdolMatch:  false,
	}
	checkIdolAndGroupExist(suggestion)

	// save suggetion to database and memory
	suggestionQueue = append(suggestionQueue, suggestion)
	helpers.MDbInsert(models.BiasGameSuggestionsTable, suggestion)
	updateSuggestionQueueCount()

	if len(suggestionQueue) == 1 || len(suggestionQueue) == 0 {

		updateCurrentSuggestionEmbed()

		// make a message and delete it immediatly. just to show that a new suggestion has come in
		msg, err := helpers.SendMessage(IMAGE_SUGGESTION_CHANNEL, "New Suggestion Ping")
		helpers.Relax(err)
		go helpers.DeleteMessageWithDelay(msg[0], time.Second*2)
	}

}

// CheckSuggestionReaction will check if the reaction was added to a suggestion message
func CheckSuggestionReaction(reaction *discordgo.MessageReactionAdd) {
	var userResponseMessage string

	// check if the reaction added was valid
	if CHECKMARK_EMOJI != reaction.Emoji.Name && X_EMOJI != reaction.Emoji.Name {
		return
	}

	// check if the reaction was added to the suggestion embed message
	if reaction.MessageID == suggestionEmbedMessageId {
		if len(suggestionQueue) == 0 {
			return
		}

		cs := suggestionQueue[0]

		// update current page based on direction
		if CHECKMARK_EMOJI == reaction.Emoji.Name {

			// send processing image message
			msg, err := helpers.SendMessage(IMAGE_SUGGESTION_CHANNEL, "Uploading image to google drive...")
			if err == nil {
				defer cache.GetSession().ChannelMessageDelete(IMAGE_SUGGESTION_CHANNEL, msg[0].ID)
			}

			// make call to get suggestion image
			res, err := pester.Get(cs.ImageURL)
			if err != nil {
				msg, _ := helpers.SendMessage(IMAGE_SUGGESTION_CHANNEL, helpers.GetText("plugins.biasgame.suggestion.could-not-decode"))
				go helpers.DeleteMessageWithDelay(msg[0], time.Second*15)
				return
			}

			approvedImage, err := helpers.DecodeImage(res.Body)
			if err != nil {
				msg, _ := helpers.SendMessage(IMAGE_SUGGESTION_CHANNEL, helpers.GetText("plugins.biasgame.suggestion.could-not-decode"))
				go helpers.DeleteMessageWithDelay(msg[0], time.Second*15)
				return
			}

			buf := new(bytes.Buffer)
			encoder := new(png.Encoder)
			encoder.CompressionLevel = -2 // -2 compression is best speed
			encoder.Encode(buf, approvedImage)
			myReader := bytes.NewReader(buf.Bytes())

			// upload image to google drive
			go func() {

				file_meta := &drive.File{Name: fmt.Sprintf("%s_%s.png", cs.GrouopName, cs.Name), Parents: []string{genderFolderMap[cs.Gender]}}
				approvedFiles, err := cache.GetGoogleDriveService().Files.Create(file_meta).Media(myReader).Fields(googleapi.Field("name, id, parents, webViewLink, webContentLink")).Do()
				if err != nil {
					fmt.Println("error: ", err.Error())
					return
				}
				addDriveFileToAllBiases(approvedFiles)
			}()

			// set image accepted image
			userResponseMessage = fmt.Sprintf("**Bias Game Suggestion Approved** <:SeemsBlob:422158571115905034>\nIdol: %s %s\nImage: <%s>", cs.GrouopName, cs.Name, cs.ImageURL)
			cs.Status = "approved"

		} else if X_EMOJI == reaction.Emoji.Name {

			// image was denied
			userResponseMessage = fmt.Sprintf("**Bias Game Suggestion Denied** <:NotLikeBlob:422163995869315082>\nIdol: %s %s\nImage: <%s>", cs.GrouopName, cs.Name, cs.ImageURL)
			cs.Status = "denied"
		}

		// update db record
		cs.ProcessedByUserId = reaction.UserID
		cs.LastModifiedOn = time.Now()
		go helpers.MDbUpsertID(models.BiasGameSuggestionsTable, cs.ID, cs)

		// send a message to the user who suggested the image
		dmChannel, err := cache.GetSession().UserChannelCreate(cs.UserID)
		if err == nil {
			// set notes if there are any
			if cs.Notes != "" {
				userResponseMessage += "\nNotes: " + cs.Notes
			}
			go helpers.SendMessage(dmChannel.ID, userResponseMessage)
		}

		// delete first suggestion and process queue again
		suggestionQueue = suggestionQueue[1:]
		go updateCurrentSuggestionEmbed()
	}

	return
}

// UpdateSuggestionDetails
func UpdateSuggestionDetails(msg *discordgo.Message, fieldToUpdate string, value string) {
	if msg.ChannelID != IMAGE_SUGGESTION_CHANNEL {
		return
	}

	if len(suggestionQueue) == 0 {
		return
	}

	go helpers.DeleteMessageWithDelay(msg, time.Second)

	cs := suggestionQueue[0]
	fieldToUpdate = strings.ToLower(fieldToUpdate)

	switch fieldToUpdate {
	case "name":
		cs.Name = value
	case "group":
		cs.GrouopName = value
	case "gender":
		cs.Gender = value
	case "notes":
		cs.Notes = value
	default:
		return
	}

	// save changes and update embed message
	helpers.MDbUpsertID(models.BiasGameSuggestionsTable, cs.ID, cs)
	updateCurrentSuggestionEmbed()
}

// updateCurrentSuggestionEmbed will re-render the embed message with the current suggestion if one exists
func updateCurrentSuggestionEmbed() {
	var embed *discordgo.MessageEmbed
	var msgSend *discordgo.MessageSend

	if exampleRoundPicId != "" {
		go cache.GetSession().ChannelMessageDelete(IMAGE_SUGGESTION_CHANNEL, exampleRoundPicId)
	}

	if len(suggestionQueue) == 0 {

		embed = &discordgo.MessageEmbed{
			Color: 0x0FADED, // blueish
			Author: &discordgo.MessageEmbedAuthor{
				Name: "No suggestions in queue",
			},
		}

		msgSend = &discordgo.MessageSend{Embed: embed}

	} else {
		// current suggestion
		cs := suggestionQueue[0]
		checkIdolAndGroupExist(cs)

		res, err := pester.Get(cs.ImageURL)
		if err != nil {
			fmt.Println("get error: ", err.Error())
			return
		}

		suggestedImage, imgErr := helpers.DecodeImage(res.Body)
		if imgErr != nil {
			return
		}

		resizedImage := resize.Resize(0, IMAGE_RESIZE_HEIGHT, suggestedImage, resize.Lanczos3)

		img1 := giveImageShadowBorder(resizedImage, 15, 15)
		img2 := giveImageShadowBorder(resizedImage, 15, 15)

		img1 = helpers.CombineTwoImages(img1, versesImage)
		finalImage := helpers.CombineTwoImages(img1, img2)

		buf := new(bytes.Buffer)
		encoder := new(png.Encoder)
		encoder.CompressionLevel = -2 // -2 compression is best speed, -3 is best compression but end result isn't worth the slower encoding
		encoder.Encode(buf, finalImage)
		myReader := bytes.NewReader(buf.Bytes())

		// get info of user who suggested image
		suggestedBy, err := cache.GetSession().User(cs.UserID)

		// get guild and channel info it was suggested from
		suggestedFromText := "No Guild Info"
		suggestedFromCh, err := cache.GetSession().Channel(cs.ChannelID)
		suggestedFrom, err := cache.GetSession().Guild(suggestedFromCh.GuildID)
		if err == nil {
			suggestedFromText = fmt.Sprintf("%s | #%s", suggestedFrom.Name, suggestedFromCh.Name)
		}

		// if the group name and idol name were matched show a checkmark, otherwise show a question mark
		groupNameDisplay := "Group Name"
		if cs.GroupMatch == true {
			groupNameDisplay += " " + CHECKMARK_EMOJI
		} else {
			groupNameDisplay += " " + QUESTIONMARK_EMOJI
		}
		idolNameDisplay := "Idol Name"
		if cs.IdolMatch == true {
			idolNameDisplay += " " + CHECKMARK_EMOJI
		} else {
			idolNameDisplay += " " + QUESTIONMARK_EMOJI
		}

		// check if notes are set, if not then display no notes entered.
		//  discord embeds can't have empty field values
		notesValue := cs.Notes
		if notesValue == "" {
			notesValue = "*No notes entered*"
		}

		embed = &discordgo.MessageEmbed{
			Color: 0x0FADED, // blueish
			// Author: &discordgo.MessageEmbedAuthor{
			// 	Name: fmt.Sprintf("Suggestions in queue: %d", len(suggestionQueue)),
			// },
			Image: &discordgo.MessageEmbedImage{
				URL: "attachment://example_round.png",
			},
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   idolNameDisplay,
					Value:  cs.Name,
					Inline: true,
				},
				{
					Name:   groupNameDisplay,
					Value:  cs.GrouopName,
					Inline: true,
				},
				{
					Name:   "Gender",
					Value:  cs.Gender,
					Inline: true,
				},
				{
					Name:   "Suggested By",
					Value:  fmt.Sprintf("%s", suggestedBy.Mention()),
					Inline: true,
				},
				{
					Name:   "Suggested From",
					Value:  suggestedFromText,
					Inline: true,
				},
				{
					Name:   "Timestamp",
					Value:  cs.ID.Time().Format("Jan 2, 2006 3:04pm (MST)"),
					Inline: true,
				},
				{
					Name:   "Notes",
					Value:  notesValue,
					Inline: true,
				},
				{
					Name:   "Image URL",
					Value:  cs.ImageURL,
					Inline: true,
				},
			},
		}

		msgSend = &discordgo.MessageSend{
			Files: []*discordgo.File{{
				Name:   "example_round.png",
				Reader: myReader,
			}},
			Embed: embed,
		}
	}

	// send or edit embed message
	var embedMsg *discordgo.Message
	cache.GetSession().ChannelMessageDelete(IMAGE_SUGGESTION_CHANNEL, suggestionEmbedMessageId)
	embedMsg, _ = cache.GetSession().ChannelMessageSendComplex(IMAGE_SUGGESTION_CHANNEL, msgSend)
	suggestionEmbedMessageId = embedMsg.ID
	updateSuggestionQueueCount()
	// if suggestionEmbedMessageId == "" {
	// embedMsg, _ = utils.SendEmbed(IMAGE_SUGGESTION_CHANNEL, embed)
	// } else {
	// embedMsg, _ = cache.GetSession().ChannelMessageEditComplex(m)
	// embedMsg, _ = utils.EditEmbed(IMAGE_SUGGESTION_CHANNEL, suggestionEmbedMessageId, embed)
	// }

	// delete any reactions on message and then reset them if there's another suggestion in queue
	cache.GetSession().MessageReactionsRemoveAll(IMAGE_SUGGESTION_CHANNEL, embedMsg.ID)
	if len(suggestionQueue) > 0 {
		cache.GetSession().MessageReactionAdd(IMAGE_SUGGESTION_CHANNEL, embedMsg.ID, CHECKMARK_EMOJI)
		cache.GetSession().MessageReactionAdd(IMAGE_SUGGESTION_CHANNEL, embedMsg.ID, X_EMOJI)
	}
}

func updateSuggestionQueueCount() {
	// update suggestion count message
	if suggestionQueueCountMessageId == "" {
		msg, err := cache.GetSession().ChannelMessageSend(IMAGE_SUGGESTION_CHANNEL, fmt.Sprintf("Suggestions in queue: %d", len(suggestionQueue)))
		if err == nil {
			suggestionQueueCountMessageId = msg.ID
		}
	} else {
		cache.GetSession().ChannelMessageEdit(IMAGE_SUGGESTION_CHANNEL, suggestionQueueCountMessageId, fmt.Sprintf("Suggestions in queue: %d", len(suggestionQueue)))
	}
}

// loadUnresolvedSuggestions
func loadUnresolvedSuggestions() {
	queryParams := bson.M{}

	queryParams["status"] = ""

	helpers.MDbIter(helpers.MdbCollection(models.BiasGameSuggestionsTable).Find(queryParams)).All(&suggestionQueue)
}

// does a loose comparison of the suggested idols and idols already in the game.
func checkIdolAndGroupExist(sug *models.BiasGameSuggestionEntry) {

	// create map of group => idols in group
	groupIdolMap := make(map[string][]string)
	for _, bias := range allBiasChoices {
		groupIdolMap[bias.GroupName] = append(groupIdolMap[bias.GroupName], bias.BiasName)
	}

	// check if the group suggested matches a current group. do loose comparison
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	for k, v := range groupIdolMap {
		curGroup := strings.ToLower(reg.ReplaceAllString(k, ""))
		sugGroup := strings.ToLower(reg.ReplaceAllString(sug.GrouopName, ""))

		// if groups match, set the suggested group to the current group
		if curGroup == sugGroup {
			sug.GroupMatch = true
			sug.GrouopName = k

			// check if the idols name matches
			for _, idolName := range v {
				curName := strings.ToLower(reg.ReplaceAllString(idolName, ""))
				sugName := strings.ToLower(reg.ReplaceAllString(sug.Name, ""))

				if curName == sugName {
					sug.IdolMatch = true
					sug.Name = idolName
					break
				}
			}
			break
		}
	}
}
