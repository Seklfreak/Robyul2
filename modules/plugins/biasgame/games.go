package biasgame

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"
	"image"
	"image/draw"
	"image/png"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/nfnt/resize"
)

type BiasGame struct{}

type biasChoice struct {
	// info directly from google drive
	FileName       string
	DriveId        string
	WebViewLink    string
	WebContentLink string
	Gender         string

	// image
	BiasImages [][]byte

	// bias info
	BiasName  string
	GroupName string
}

type singleBiasGame struct {
	User             *discordgo.User
	ChannelID        string
	RoundLosers      []*biasChoice
	RoundWinners     []*biasChoice
	BiasQueue        []*biasChoice
	TopEight         []*biasChoice
	GameWinnerBias   *biasChoice
	IdolsRemaining   int
	LastRoundMessage *discordgo.Message
	ReadyForReaction bool   // used to make sure multiple reactions aren't counted
	Gender           string // girl, boy, mixed

	// a map of fileName => image array position. This is used to make sure that when a random image is selected for a game, that the same image is still used throughout the game
	GameImageIndex map[string]int
}

type multiBiasGame struct {
	CurrentRoundMessageId string // used to find game when reactions are added
	ChannelID             string
	RoundLosers           []*biasChoice
	RoundWinners          []*biasChoice
	BiasQueue             []*biasChoice
	TopEight              []*biasChoice
	GameWinnerBias        *biasChoice
	IdolsRemaining        int
	LastRoundMessage      *discordgo.Message
	Gender                string // girl, boy, mixed
	UserIdsInvolved       []string

	// a map of fileName => image array position. This is used to make sure that when a random image is selected for a game, that the same image is still used throughout the game
	GameImageIndex map[string]int
}

const (
	DRIVE_SEARCH_TEXT       = "\"%s\" in parents and (mimeType = \"image/gif\" or mimeType = \"image/jpeg\" or mimeType = \"image/png\" or mimeType = \"application/vnd.google-apps.folder\")"
	GIRLS_FOLDER_ID         = "1CIM6yrvZOKn_R-qWYJ6pISHyq-JQRkja"
	BOYS_FOLDER_ID          = "1psrhQQaV0kwPhAMtJ7LYT2SWgLoyDb-J"
	MISC_FOLDER_ID          = "1-HdvH5fiOKuZvPPVkVMILZxkjZKv9x_x"
	IMAGE_RESIZE_HEIGHT     = 150
	LEFT_ARROW_EMOJI        = "⬅"
	RIGHT_ARROW_EMOJI       = "➡"
	ARROW_FORWARD_EMOJI     = "▶"
	ARROW_BACKWARD_EMOJI    = "◀"
	ZERO_WIDTH_SPACE        = "\u200B"
	BOT_OWNER_ID            = "273639623324991489"
	MULTIPLAYER_ROUND_DELAY = 5
)

// used to stop commands from going through
//  before the game is ready after a bot restart
var gameIsReady = false

// misc images
var versesImage image.Image
var winnerBracket image.Image
var shadowBorder image.Image
var crown image.Image

// currently running single or multiplyer games
var currentSinglePlayerGames map[string]*singleBiasGame
var currentMultiPlayerGames []*multiBiasGame

// holds all available idols in the game
var allBiasChoices []*biasChoice

// game configs
var allowedGameSizes map[int]bool
var biasGameGenders map[string]string

// top 8 bracket
var bracketImageOffsets map[int]image.Point
var bracketImageResizeMap map[int]uint

// Init when the bot starts up
func (b *BiasGame) Init(session *discordgo.Session) {
	defer helpers.Recover()

	// set global variables
	currentSinglePlayerGames = make(map[string]*singleBiasGame)
	allowedGameSizes = map[int]bool{
		10:  true, // for dev only, remove when game is live
		32:  true,
		64:  true,
		128: true,
		256: true,
	}
	biasGameGenders = map[string]string{
		"boy":   "boy",
		"boys":  "boy",
		"girl":  "girl",
		"girls": "girl",
		"mixed": "mixed",
	}
	// offsets of where bias images need to be placed on bracket image
	bracketImageOffsets = map[int]image.Point{
		14: image.Pt(182, 53),

		13: image.Pt(358, 271),
		12: image.Pt(81, 271),

		11: image.Pt(443, 409),
		10: image.Pt(305, 409),
		9:  image.Pt(167, 409),
		8:  image.Pt(29, 409),

		7: image.Pt(478, 517),
		6: image.Pt(419, 517),
		5: image.Pt(340, 517),
		4: image.Pt(281, 517),
		3: image.Pt(202, 517),
		2: image.Pt(143, 517),
		1: image.Pt(64, 517),
		0: image.Pt(5, 517),
	}
	bracketImageResizeMap = map[int]uint{
		14: 165,
		13: 90, 12: 90,
		11: 60, 10: 60, 9: 60, 8: 60,
	}

	// load all images and information
	refreshBiasChoices(false)
	loadMiscImages(false)
	startBiasCacheRefreshLoop()

	// set up suggestions channel
	initSuggestionChannel()

	// get any in progress games saved in cache and immediatly delete them
	getBiasGameCache("currentSinglePlayerGames", &currentSinglePlayerGames)
	getBiasGameCache("currentMultiPlayerGames", &currentMultiPlayerGames)

	// start any multi games
	for _, multiGame := range currentMultiPlayerGames {
		go multiGame.processMultiGame()
	}

	// spew.Dump(currentSinglePlayerGames)
	delBiasGameCache("currentSinglePlayerGames", "currentMultiPlayerGames")

	// this line should always be last in this function
	gameIsReady = true
}

// Uninit called when bot is shutting down
func (b *BiasGame) Uninit(session *discordgo.Session) {

	// save any currently running games
	if len(currentSinglePlayerGames) > 0 {
		setBiasGameCache("currentSinglePlayerGames", currentSinglePlayerGames, 0)
	}
	if len(currentMultiPlayerGames) > 0 {
		setBiasGameCache("currentMultiPlayerGames", currentMultiPlayerGames, 0)
	}
}

// Will validate if the passed command entered is used for this plugin
func (b *BiasGame) Commands() []string {
	return []string{
		"biasgame",
		"edit",
	}
}

// Main Entry point for the plugin
func (b *BiasGame) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	defer helpers.Recover()

	// images, suggestions, and stat set up are done async when bot starts up
	//   make sure game is ready before trying to process any commands
	if gameIsReady == false {
		helpers.SendMessage(msg.ChannelID, "biasgame.game.game-not-ready")
		return
	}

	// process text after the initial command
	commandArgs := strings.Fields(content)
	if command == "biasgame" {

		if len(commandArgs) == 0 {
			// start default bias game
			singleGame := createOrGetSinglePlayerGame(msg, "girl", 32)
			singleGame.sendBiasGameRound()

		} else if commandArgs[0] == "stats" {

			// stats
			displayBiasGameStats(msg, content)

		} else if commandArgs[0] == "rankings" {

			showSingleGameRankings(msg)

		} else if commandArgs[0] == "suggest" {

			// // create map of group => idols in group
			// groupIdolMap := make(map[string][]string)
			// for _, bias := range allBiasChoices {
			// 	groupIdolMap[bias.groupName] = append(groupIdolMap[bias.groupName], bias.biasName)
			// }

			ProcessImageSuggestion(msg, content)

		} else if commandArgs[0] == "current" {

			displayCurrentGameStats(msg)

		} else if commandArgs[0] == "multi" {

			startMultiPlayerGame(msg, commandArgs)

		} else if commandArgs[0] == "idols" {

			listIdolsInGame(msg)

		} else if commandArgs[0] == "refresh-images" {

			// check if the user is the bot owner
			if msg.Author.ID == BOT_OWNER_ID {

				newMessages, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.refresh.refresing"))
				helpers.Relax(err)
				refreshBiasChoices(true)

				cache.GetSession().ChannelMessageDelete(msg.ChannelID, newMessages[0].ID)
				helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.refresh.refresh-done"))
			} else {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.refresh.not-bot-owner"))
			}

		} else if gameSize, err := strconv.Atoi(commandArgs[0]); err == nil {

			// check if the game size the user wants is valid
			if allowedGameSizes[gameSize] {
				singleGame := createOrGetSinglePlayerGame(msg, "girl", gameSize)
				singleGame.sendBiasGameRound()
			} else {
				helpers.SendMessage(msg.ChannelID, "biasgame.game.invalid-game-size")
				return
			}

		} else if gameGender, ok := biasGameGenders[commandArgs[0]]; ok {

			// check if the game size the user wants is valid
			if len(commandArgs) == 2 {

				gameSize, _ := strconv.Atoi(commandArgs[1])
				if allowedGameSizes[gameSize] {
					singleGame := createOrGetSinglePlayerGame(msg, gameGender, gameSize)
					singleGame.sendBiasGameRound()
				} else {
					helpers.SendMessage(msg.ChannelID, "biasgame.game.invalid-game-size")
					return
				}
			} else {
				singleGame := createOrGetSinglePlayerGame(msg, gameGender, 32)
				singleGame.sendBiasGameRound()
			}

		}
	} else if command == "edit" { // edit is used for changing details of suggestions
		fieldToUpdate := commandArgs[0]
		fieldValue := strings.Join(commandArgs[1:], " ")
		UpdateSuggestionDetails(msg, fieldToUpdate, fieldValue)
	}
}

// Called whenever a reaction is added to any message
func (b *BiasGame) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {
	defer helpers.Recover()

	if gameIsReady == false {
		return
	}

	// confirm the reaction was added to a message for one bias games
	if game, ok := currentSinglePlayerGames[reaction.UserID]; ok == true {
		game.processVote(reaction)
	}

	// check if the reaction was added to a paged message
	if pagedMessage := helpers.GetPagedMessage(reaction.MessageID); pagedMessage != nil {
		pagedMessage.UpdateMessagePage(reaction)
	}

	// check if this was a reaction to a idol suggestion.
	//  if it was accepted an image will be returned to be added to the biasChoices
	CheckSuggestionReaction(reaction)
}

/////////////////////////////////
//    SINGLE GAME FUNCTIONS    //
/////////////////////////////////

// createSinglePlayerGame will setup a singleplayer game for the user
func createOrGetSinglePlayerGame(msg *discordgo.Message, gameGender string, gameSize int) *singleBiasGame {
	var singleGame *singleBiasGame

	// check if the user has a current game already going.
	// if so update the channel id for the game incase the user tried starting the game from another server
	if game, ok := currentSinglePlayerGames[msg.Author.ID]; ok {

		// if the user already had a game going, let them know to avoid confusion if they
		//   tried starting another game a long time after the first
		//
		// -- not sure if i want this or not...
		// msg, err := helpers.SendMessage(msg.ChannelID, "biasgame.game.resuming-game")
		// if err == nil {
		// 	go utils.DeleteImageWithDelay(msg, time.Second*10)
		// }

		game.ChannelID = msg.ChannelID
		singleGame = game
	} else {
		var biasChoices []*biasChoice

		// if this isn't a mixed game then filter all choices by the gender
		if gameGender != "mixed" {

			for _, bias := range allBiasChoices {
				if bias.Gender == gameGender {
					biasChoices = append(biasChoices, bias)
				}
			}
		} else {
			biasChoices = allBiasChoices
		}

		// confirm we have enough biases to choose from for the game size this should be
		if len(biasChoices) < gameSize {
			helpers.SendMessage(msg.ChannelID, "biasgame.game.not-enough-idols")
			return nil
		}

		// create new game
		singleGame = &singleBiasGame{
			User:             msg.Author,
			ChannelID:        msg.ChannelID,
			IdolsRemaining:   gameSize,
			ReadyForReaction: false,
			Gender:           gameGender,
		}
		singleGame.GameImageIndex = make(map[string]int)

		// get random biases for the game
		usedIndexs := make(map[int]bool)
		for true {
			randomIndex := rand.Intn(len(biasChoices))

			if usedIndexs[randomIndex] == false {
				usedIndexs[randomIndex] = true
				singleGame.BiasQueue = append(singleGame.BiasQueue, biasChoices[randomIndex])

				if len(singleGame.BiasQueue) == gameSize {
					break
				}
			}
		}

		// save game to current running games
		currentSinglePlayerGames[msg.Author.ID] = singleGame
	}

	return singleGame
}

// processVote is called when a valid reaction is added to a game
func (g *singleBiasGame) processVote(reaction *discordgo.MessageReactionAdd) {

	// check if reaction was added to the message of the game
	if g.LastRoundMessage.ID == reaction.MessageID && g.ReadyForReaction == true {

		winnerIndex := 0
		loserIndex := 0
		validReaction := false

		// check if the reaction added to the message was a left or right arrow
		if LEFT_ARROW_EMOJI == reaction.Emoji.Name {
			winnerIndex = 0
			loserIndex = 1
			validReaction = true
		} else if RIGHT_ARROW_EMOJI == reaction.Emoji.Name {
			winnerIndex = 1
			loserIndex = 0
			validReaction = true
		}

		if validReaction == true {
			g.ReadyForReaction = false
			g.IdolsRemaining--

			// record winners and losers for stats
			g.RoundLosers = append(g.RoundLosers, g.BiasQueue[loserIndex])
			g.RoundWinners = append(g.RoundWinners, g.BiasQueue[winnerIndex])

			// add winner to end of bias queue and remove first two
			g.BiasQueue = append(g.BiasQueue, g.BiasQueue[winnerIndex])
			g.BiasQueue = g.BiasQueue[2:]

			// if there is only one bias left, they are the winner
			if len(g.BiasQueue) == 1 {

				g.GameWinnerBias = g.BiasQueue[0]
				g.sendWinnerMessage()

				// record game stats
				go recordSingleGamesStats(g)

				// end the g. delete from current games
				delete(currentSinglePlayerGames, g.User.ID)

			} else {

				// save the last 8 for the chart
				if len(g.BiasQueue) == 8 {
					g.TopEight = g.BiasQueue
				}

				// Sleep a time bit to allow other users to see what was chosen.
				// This creates conversation while the game is going and makes it a overall better experience
				//
				//   This will also allow me to call out and harshly judge players who don't choose nayoung.
				time.Sleep(time.Second / 5)

				g.sendBiasGameRound()
			}

		}
	}
}

// sendBiasGameRound will send the message for the round
func (g *singleBiasGame) sendBiasGameRound() {
	if g == nil {
		return
	}

	// if a round message has been sent, delete before sending the next one
	if g.LastRoundMessage != nil {
		go cache.GetSession().ChannelMessageDelete(g.LastRoundMessage.ChannelID, g.LastRoundMessage.ID)
	}

	// combine first bias image with the "vs" image, then combine that image with 2nd bias image
	img1 := g.BiasQueue[0].getRandomBiasImage(&g.GameImageIndex)
	img2 := g.BiasQueue[1].getRandomBiasImage(&g.GameImageIndex)

	img1 = giveImageShadowBorder(img1, 15, 15)
	img2 = giveImageShadowBorder(img2, 15, 15)

	img1 = helpers.CombineTwoImages(img1, versesImage)
	finalImage := helpers.CombineTwoImages(img1, img2)

	// create round message
	messageString := fmt.Sprintf("**@%s**\nIdols Remaining: %d\n%s %s vs %s %s",
		g.User.Username,
		g.IdolsRemaining,
		g.BiasQueue[0].GroupName,
		g.BiasQueue[0].BiasName,
		g.BiasQueue[1].GroupName,
		g.BiasQueue[1].BiasName)

	// encode the combined image and compress it
	buf := new(bytes.Buffer)
	encoder := new(png.Encoder)
	encoder.CompressionLevel = -2
	encoder.Encode(buf, finalImage)
	myReader := bytes.NewReader(buf.Bytes())

	// send round message
	fileSendMsg, err := helpers.SendFile(g.ChannelID, "combined_pic.png", myReader, messageString)
	if err != nil {
		return
	}

	// add reactions
	cache.GetSession().MessageReactionAdd(g.ChannelID, fileSendMsg.ID, LEFT_ARROW_EMOJI)
	go cache.GetSession().MessageReactionAdd(g.ChannelID, fileSendMsg.ID, RIGHT_ARROW_EMOJI)

	// update game state
	g.LastRoundMessage = fileSendMsg
	g.ReadyForReaction = true
}

// sendWinnerMessage creates the top eight brackent sends the winning message to the user
func (g *singleBiasGame) sendWinnerMessage() {

	// if a round message has been sent, delete before sending the next one
	if g.LastRoundMessage != nil {
		cache.GetSession().ChannelMessageDelete(g.LastRoundMessage.ChannelID, g.LastRoundMessage.ID)
	}

	// get last 7 from winners array and combine with topEight array
	winners := g.RoundWinners[len(g.RoundWinners)-7 : len(g.RoundWinners)]
	bracketInfo := append(g.TopEight, winners...)

	// create final image with the bounds of the winner bracket
	bracketImage := image.NewRGBA(winnerBracket.Bounds())
	draw.Draw(bracketImage, winnerBracket.Bounds(), winnerBracket, image.Point{0, 0}, draw.Src)

	// populate winner brackent image
	for i, bias := range bracketInfo {

		// adjust images sizing according to placement
		resizeTo := uint(50)

		if newResizeVal, ok := bracketImageResizeMap[i]; ok {
			resizeTo = newResizeVal
		}

		ri := resize.Resize(0, resizeTo, bias.getRandomBiasImage(&g.GameImageIndex), resize.Lanczos3)

		draw.Draw(bracketImage, ri.Bounds().Add(bracketImageOffsets[i]), ri, image.ZP, draw.Over)
	}

	// compress bracket image
	buf := new(bytes.Buffer)
	encoder := new(png.Encoder)
	encoder.CompressionLevel = -2 // -2 compression is best speed, -3 is best compression but end result isn't worth the slower encoding
	encoder.Encode(buf, bracketImage)
	myReader := bytes.NewReader(buf.Bytes())

	messageString := fmt.Sprintf("%s\nWinner: %s %s!",
		g.User.Mention(),
		g.GameWinnerBias.GroupName,
		g.GameWinnerBias.BiasName)

	// send message
	helpers.SendFile(g.ChannelID, "biasgame_winner.png", myReader, messageString)
}

/////////////////////////////////
//     MULTI GAME FUNCTIONS    //
/////////////////////////////////

// startMultiPlayerGame will create and start a multiplayer game
func startMultiPlayerGame(msg *discordgo.Message, commandArgs []string) {
	fmt.Println("starting multi game")

	// check if a multi game is already running in the current channel
	for _, game := range currentMultiPlayerGames {
		if game.ChannelID == msg.ChannelID {
			helpers.SendMessage(msg.ChannelID, "biasgame.game.multi-game-running")
			return
		}
	}

	var gameGender string
	var ok bool

	// if command args are at least 2, check if the 2nd arg is valid gender
	if len(commandArgs) >= 2 {

		if gameGender, ok = biasGameGenders[commandArgs[1]]; ok == false {
			// todo: some message probably
			return
		}
	} else {

		// set gender to girl
		gameGender = "girl"
	}

	var biasChoices []*biasChoice

	// if this isn't a mixed game then filter all choices by the gender
	if gameGender != "mixed" {

		for _, bias := range allBiasChoices {
			if bias.Gender == gameGender {
				biasChoices = append(biasChoices, bias)
			}
		}
	} else {
		biasChoices = allBiasChoices
	}

	// confirm we have enough biases for a multiplayer game
	if len(biasChoices) < 32 {
		helpers.SendMessage(msg.ChannelID, "biasgame.game.not-enough-idols")
		return
	}

	// create new game
	multiGame := &multiBiasGame{
		ChannelID:      msg.ChannelID,
		IdolsRemaining: 32,
		Gender:         gameGender,
	}
	multiGame.GameImageIndex = make(map[string]int)

	// get random biases for the game
	usedIndexs := make(map[int]bool)
	for true {
		randomIndex := rand.Intn(len(biasChoices))

		if usedIndexs[randomIndex] == false {
			usedIndexs[randomIndex] = true
			multiGame.BiasQueue = append(multiGame.BiasQueue, biasChoices[randomIndex])

			if len(multiGame.BiasQueue) == multiGame.IdolsRemaining {
				break
			}
		}
	}

	// save game to current running games
	currentMultiPlayerGames = append(currentMultiPlayerGames, multiGame)

	multiGame.processMultiGame()
}

// sendMultiBiasGameRound sends the next round for the multi game
func (g *multiBiasGame) sendMultiBiasGameRound() {
	if g == nil {
		return
	}

	// if a round message has been sent, delete before sending the next one
	if g.LastRoundMessage != nil {
		cache.GetSession().ChannelMessageDelete(g.LastRoundMessage.ChannelID, g.LastRoundMessage.ID)
	}

	// combine first bias image with the "vs" image, then combine that image with 2nd bias image
	img1 := g.BiasQueue[0].getRandomBiasImage(&g.GameImageIndex)
	img2 := g.BiasQueue[1].getRandomBiasImage(&g.GameImageIndex)

	img1 = giveImageShadowBorder(img1, 15, 15)
	img2 = giveImageShadowBorder(img2, 15, 15)

	img1 = helpers.CombineTwoImages(img1, versesImage)
	finalImage := helpers.CombineTwoImages(img1, img2)

	// create round message
	messageString := fmt.Sprintf("**Multi Game**\nIdols Remaining: %d\n%s %s vs %s %s",
		g.IdolsRemaining,
		g.BiasQueue[0].GroupName,
		g.BiasQueue[0].BiasName,
		g.BiasQueue[1].GroupName,
		g.BiasQueue[1].BiasName)

	// encode the combined image and compress it
	buf := new(bytes.Buffer)
	encoder := new(png.Encoder)
	encoder.CompressionLevel = -2 // -2 compression is best speed, -3 is best compression but end result isn't worth the slower encoding
	encoder.Encode(buf, finalImage)
	myReader := bytes.NewReader(buf.Bytes())

	// send round message
	fileSendMsg, err := helpers.SendFile(g.ChannelID, "combined_pic.png", myReader, messageString)
	if err != nil {
		return
	}

	// add reactions
	cache.GetSession().MessageReactionAdd(g.ChannelID, fileSendMsg.ID, LEFT_ARROW_EMOJI)
	cache.GetSession().MessageReactionAdd(g.ChannelID, fileSendMsg.ID, RIGHT_ARROW_EMOJI)

	// update game state
	g.CurrentRoundMessageId = fileSendMsg.ID
	g.LastRoundMessage = fileSendMsg
}

// start multi game loop. every 10 seconds count the number of arrow reactions. whichever side has most wins
func (g *multiBiasGame) processMultiGame() {

	for g.IdolsRemaining != 1 {

		// send next rounds and sleep
		g.sendMultiBiasGameRound()
		time.Sleep(time.Second * MULTIPLAYER_ROUND_DELAY)

		// get current round message
		message, err := cache.GetSession().ChannelMessage(g.ChannelID, g.CurrentRoundMessageId)
		if err != nil {
			fmt.Println("multi game error: ", err.Error())
			return
		}

		leftCount := 0
		rightCount := 0

		// check which reaction has most votes
		for _, reaction := range message.Reactions {
			// ignore reactions not from bot
			if reaction.Me == false {
				continue
			}

			if reaction.Emoji.Name == LEFT_ARROW_EMOJI {
				leftCount = reaction.Count
			}
			if reaction.Emoji.Name == RIGHT_ARROW_EMOJI {
				rightCount = reaction.Count
			}
		}

		winnerIndex := 0
		loserIndex := 0
		randomWin := false

		// check if the reaction added to the message was a left or right arrow
		if leftCount > rightCount {
			winnerIndex = 0
			loserIndex = 1
		} else if leftCount < rightCount {
			winnerIndex = 1
			loserIndex = 0
		} else {
			// if votes are even, choose one at random
			randomNumber := rand.Intn(100)
			randomWin = true
			// fmt.Println("random number: ", randomNumber)
			if randomNumber >= 50 {
				winnerIndex = 1
				loserIndex = 0
			} else {
				winnerIndex = 0
				loserIndex = 1
			}
		}

		// if a random winner was chosen, display an arrow indication who the random winner was
		if randomWin == true {
			cache.GetSession().MessageReactionsRemoveAll(g.ChannelID, g.CurrentRoundMessageId)
			if winnerIndex == 1 {
				cache.GetSession().MessageReactionAdd(g.ChannelID, g.CurrentRoundMessageId, ARROW_FORWARD_EMOJI)
			} else {
				cache.GetSession().MessageReactionAdd(g.ChannelID, g.CurrentRoundMessageId, ARROW_BACKWARD_EMOJI)
			}
			time.Sleep(time.Millisecond * 1500)
		}

		g.IdolsRemaining--

		// fmt.Println("round winner: ", g.BiasQueue[winnerIndex].groupName, g.BiasQueue[winnerIndex].biasName)

		// record winners and losers for stats
		g.RoundLosers = append(g.RoundLosers, g.BiasQueue[loserIndex])
		g.RoundWinners = append(g.RoundWinners, g.BiasQueue[winnerIndex])

		// add winner to end of bias queue and remove first two
		g.BiasQueue = append(g.BiasQueue, g.BiasQueue[winnerIndex])
		g.BiasQueue = g.BiasQueue[2:]

		// save the last 8 for the chart
		if len(g.BiasQueue) == 8 {
			g.TopEight = g.BiasQueue
		}
	}

	g.GameWinnerBias = g.BiasQueue[0]
	g.sendWinnerMessage()

	// record game stats
	go recordMultiGamesStats(g)

	// delete multi game from current multi games
	for i, game := range currentMultiPlayerGames {
		if game.CurrentRoundMessageId == g.CurrentRoundMessageId {
			currentMultiPlayerGames = append(currentMultiPlayerGames[:i], currentMultiPlayerGames[i+1:]...)
		}
	}
}

// sendWinnerMessage creates the top eight brackent sends the winning message to the user
//
//  note: i realize this function is the exact same as the single game version,
//         but im going to choose to keep these and seporate functions to make any
//         future changes to the games easier
func (g *multiBiasGame) sendWinnerMessage() {

	// if a round message has been sent, delete before sending the next one
	if g.LastRoundMessage != nil {
		cache.GetSession().ChannelMessageDelete(g.LastRoundMessage.ChannelID, g.LastRoundMessage.ID)
	}

	// get last 7 from winners array and combine with topEight array
	winners := g.RoundWinners[len(g.RoundWinners)-7 : len(g.RoundWinners)]
	bracketInfo := append(g.TopEight, winners...)

	// create final image with the bounds of the winner bracket
	bracketImage := image.NewRGBA(winnerBracket.Bounds())
	draw.Draw(bracketImage, winnerBracket.Bounds(), winnerBracket, image.Point{0, 0}, draw.Src)

	// populate winner brackent image
	for i, bias := range bracketInfo {

		// adjust images sizing according to placement
		resizeTo := uint(50)

		if newResizeVal, ok := bracketImageResizeMap[i]; ok {
			resizeTo = newResizeVal
		}

		ri := resize.Resize(0, resizeTo, bias.getRandomBiasImage(&g.GameImageIndex), resize.Lanczos3)

		draw.Draw(bracketImage, ri.Bounds().Add(bracketImageOffsets[i]), ri, image.ZP, draw.Over)
	}

	// compress bracket image
	buf := new(bytes.Buffer)
	encoder := new(png.Encoder)
	encoder.CompressionLevel = -2 // -2 compression is best speed, -3 is best compression but end result isn't worth the slower encoding
	encoder.Encode(buf, bracketImage)
	myReader := bytes.NewReader(buf.Bytes())

	messageString := fmt.Sprintf("\nWinner: %s %s!",
		g.GameWinnerBias.GroupName,
		g.GameWinnerBias.BiasName)

	// send message
	helpers.SendFile(g.ChannelID, "biasgame_multi_winner.png", myReader, messageString)
}

//////////////////////////////////
//     BIAS CHOICE FUNCTIONS    //
//////////////////////////////////

// will return a random image for the bias,
//  if an image has already been chosen for the given game and bias thenit will use that one
func (b *biasChoice) getRandomBiasImage(gameImageIndex *map[string]int) image.Image {
	var imageIndex int

	// check if a random image for the idol has already been chosen for this game
	//  also make sure that biasimages array contains the index. it may have been changed due to a refresh from googledrive
	if imagePos, ok := (*gameImageIndex)[b.FileName]; ok && len(b.BiasImages) > imagePos {
		imageIndex = imagePos
	} else {
		imageIndex = rand.Intn(len(b.BiasImages))
		(*gameImageIndex)[b.FileName] = imageIndex
	}

	img, _, err := image.Decode(bytes.NewReader(b.BiasImages[imageIndex]))
	helpers.Relax(err)
	return img
}

///// MISC HELPER FUNCTIONS

// giveImageShadowBorder give the round image a shadow border
func giveImageShadowBorder(img image.Image, offsetX int, offsetY int) image.Image {
	rgba := image.NewRGBA(shadowBorder.Bounds())
	draw.Draw(rgba, shadowBorder.Bounds(), shadowBorder, image.Point{0, 0}, draw.Src)
	draw.Draw(rgba, img.Bounds().Add(image.Pt(offsetX, offsetY)), img, image.ZP, draw.Over)
	return rgba.SubImage(rgba.Rect)
}

// bgLog is just a small helper function for logging in the biasgame
func bgLog() *logrus.Entry {
	return cache.GetLogger().WithField("module", "biasgame")
}

// getBiasGameCache
func getBiasGameCache(key string, data interface{}) error {
	// get cache with given key
	cacheResult, err := cache.GetRedisClient().Get(fmt.Sprintf("robyul2-discord:biasgame:%s", key)).Bytes()
	if err != nil || err == redis.Nil {
		return err
	}

	// if the datas type is already []byte then set it to cache instead of unmarshal
	switch data.(type) {
	case []byte:
		data = cacheResult
		return nil
	}

	json.Unmarshal(cacheResult, data)
	return nil
}

// setBiasGameCache
func setBiasGameCache(key string, data interface{}, time time.Duration) error {
	marshaledData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	cache.GetRedisClient().Set(fmt.Sprintf("robyul2-discord:biasgame:%s", key), marshaledData, time)
	return nil
}

// delBiasGameCache
func delBiasGameCache(keys ...string) {
	for _, key := range keys {

		cache.GetRedisClient().Del(fmt.Sprintf("robyul2-discord:biasgame:%s", key))
	}
}

///// Unused functions requried by ExtendedPlugin interface
func (b *BiasGame) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {
}
func (b *BiasGame) OnMessageDelete(msg *discordgo.MessageDelete, session *discordgo.Session) {
}
func (b *BiasGame) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {
}
func (b *BiasGame) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {
}
func (b *BiasGame) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {
}
func (b *BiasGame) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {
}
func (b *BiasGame) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {
}
