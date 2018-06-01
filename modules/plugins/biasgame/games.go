package biasgame

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Seklfreak/Robyul2/modules/plugins/idols"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"

	"github.com/bwmarrin/discordgo"
	"github.com/nfnt/resize"
)

const (
	IMAGE_RESIZE_HEIGHT  = 150
	LEFT_ARROW_EMOJI     = "⬅"
	RIGHT_ARROW_EMOJI    = "➡"
	ARROW_FORWARD_EMOJI  = "▶"
	ARROW_BACKWARD_EMOJI = "◀"
)

// misc images
var versesImage image.Image
var winnerBracket image.Image
var shadowBorder image.Image

// currently running single or multiplayer games
var currentSinglePlayerGames map[string]*singleBiasGame
var currentSinglePlayerGamesMutex sync.RWMutex
var currentMultiPlayerGames []*multiBiasGame
var currentMultiPlayerGamesMutex sync.RWMutex

// game configs
var allowedGameSizes map[int]bool
var allowedMultiGameSizes map[int]bool

// top 8 bracket
var bracketImageOffsets map[int]image.Point
var bracketImageResizeMap map[int]uint

/////////////////////////////////
//    SINGLE GAME FUNCTIONS    //
/////////////////////////////////

// createSinglePlayerGame will setup a singleplayer game for the user
func createOrGetSinglePlayerGame(msg *discordgo.Message, commandArgs []string) *singleBiasGame {
	var singleGame *singleBiasGame

	// check if the user has a current game already going.
	// if so update the channel id for the game incase the user tried starting the game from another server
	if game := getSinglePlayerGameByUserID(msg.Author.ID); game != nil {

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
		var biasChoices []*idols.Idol
		gameGender := "mixed"
		gameSize := 32

		// validate game arguments
		if len(commandArgs) > 0 {
			for _, arg := range commandArgs {

				// gender check
				if gender, ok := gameGenders[arg]; ok == true {
					gameGender = gender
					continue
				}

				// game size check
				if requestedGameSize, err := strconv.Atoi(arg); err == nil {
					if _, ok := allowedGameSizes[requestedGameSize]; ok == true {

						gameSize = requestedGameSize
						continue
					} else {

						helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.game.invalid-game-size"))
						return nil
					}
				}

				// if a arg was passed that didn't match any check, send invalid args message
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				return nil
			}
		}

		// if this isn't a mixed game then filter all choices by the gender
		if gameGender != "mixed" {

			for _, bias := range idols.GetAllIdols() {
				if bias.Gender == gameGender {
					biasChoices = append(biasChoices, bias)
				}
			}
		} else {
			biasChoices = idols.GetAllIdols()
		}

		// confirm we have enough biases to choose from for the game size this should be
		if len(biasChoices) < gameSize {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.game.not-enough-idols"))
			return nil
		}

		// show a warning if the game size is >= 256, wait for confirm
		if gameSize >= 256 {

			if !helpers.ConfirmEmbed(msg.ChannelID, msg.Author, helpers.GetText("plugins.biasgame.game.size-warning"), "✅", "🚫") {
				return nil
			}

			// recheck if a game is still going on, see above
			if game := getSinglePlayerGameByUserID(msg.Author.ID); game != nil {
				game.ChannelID = msg.ChannelID
				return game
			}
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
		singleGame.saveGame()
	}

	return singleGame
}

// processVote is called when a valid reaction is added to a game
func (g *singleBiasGame) processVote(reaction *discordgo.MessageReactionAdd) {
	defer g.recoverGame()

	// check if reaction was added to the message of the game
	if g.ReadyForReaction == true && g.LastRoundMessage.ID == reaction.MessageID {

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

				g.finishSingleGame()
			} else {

				// save the last 8 for the chart
				if len(g.BiasQueue) == 8 {
					g.TopEight = g.BiasQueue
				}

				// Sleep a bit to allow other users to see what was chosen.
				// This creates conversation while the game is going and makes it a overall better experience
				//
				//   This will also allow me to call out and harshly judge players who don't choose nayoung <3
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
	defer g.recoverGame()

	// if a game only has one bias in queue, they are the winner and a round should not be attempted
	if len(g.BiasQueue) == 1 {
		g.finishSingleGame()
		return
	}

	// if a round message has been sent, delete before sending the next one
	if g.LastRoundMessage != nil {
		go cache.GetSession().ChannelMessageDelete(g.LastRoundMessage.ChannelID, g.LastRoundMessage.ID)
	}

	// get random images

	img1 := getSemiRandomIdolImage(g.BiasQueue[0], &g.GameImageIndex)
	img2 := getSemiRandomIdolImage(g.BiasQueue[1], &g.GameImageIndex)

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
	encoder.Encode(buf, makeVSImage(img1, img2))
	myReader := bytes.NewReader(buf.Bytes())

	// send round message
	fileSendMsg, err := helpers.SendFile(g.ChannelID, "combined_pic.png", myReader, messageString)
	if err != nil {

		if checkPermissionError(err, g.ChannelID) {
			helpers.SendMessage(g.ChannelID, helpers.GetText("bot.errors.no-file"))
		}

		return
	}

	// update game state
	g.LastRoundMessage = fileSendMsg[0]
	g.ReadyForReaction = true

	// add reactions
	cache.GetSession().MessageReactionAdd(g.ChannelID, fileSendMsg[0].ID, LEFT_ARROW_EMOJI)
	go cache.GetSession().MessageReactionAdd(g.ChannelID, fileSendMsg[0].ID, RIGHT_ARROW_EMOJI)
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

		ri := resize.Resize(0, resizeTo, getSemiRandomIdolImage(bias, &g.GameImageIndex), resize.Lanczos3)

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
	winnerMsgs, err := helpers.SendFile(g.ChannelID, "biasgame_winner.png", myReader, messageString)
	helpers.Relax(err)

	// if the winner is nayoung, add a nayoung emoji <3 <3 <3
	if strings.ToLower(g.GameWinnerBias.GroupName) == "pristin" && strings.ToLower(g.GameWinnerBias.BiasName) == "nayoung" {
		cache.GetSession().MessageReactionAdd(g.ChannelID, winnerMsgs[0].ID, getRandomNayoungEmoji()) // <3
	}
}

// finishSingleGame sends the winner message, records stats, and deletes game
func (g *singleBiasGame) finishSingleGame() {
	defer g.recoverGame()

	// delete from current games
	g.deleteGame()

	// used to make sure the game finish isn't triggered twice
	if g.GameWinnerBias != nil {
		return
	}

	g.GameWinnerBias = g.BiasQueue[0]
	g.sendWinnerMessage()

	// record game stats
	go func(g *singleBiasGame) {
		defer helpers.Recover()
		recordSingleGamesStats(g)
	}(g)
}

// recoverGame if a panic was caused during the game, delete from current games
func (g *singleBiasGame) recoverGame() {
	if r := recover(); r != nil {

		// delete from current games
		g.deleteGame()

		// re-panic so it gets handled and logged correctly
		panic(r)
	}
}

// saveGame saves the game to the currently running games
func (g *singleBiasGame) saveGame() {
	currentSinglePlayerGamesMutex.Lock()
	defer currentSinglePlayerGamesMutex.Unlock()

	currentSinglePlayerGames[g.User.ID] = g
}

// saveGame saves the game to the currently running games
func (g *singleBiasGame) deleteGame() {
	currentSinglePlayerGamesMutex.Lock()
	defer currentSinglePlayerGamesMutex.Unlock()

	delete(currentSinglePlayerGames, g.User.ID)
}

/////////////////////////////////
//     MULTI GAME FUNCTIONS    //
/////////////////////////////////

// startMultiPlayerGame will create and start a multiplayer game
func startMultiPlayerGame(msg *discordgo.Message, commandArgs []string) {

	// check if a multi game is already running in the current channel
	if game := getMultiPlayerGameByChannelID(msg.ChannelID); game != nil {

		// resume game if it was stopped
		if game.GameIsRunning == false {
			game.processMultiGame()
			return
		}

		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.game.multi-game-running"))
		return
	}

	commandArgs = commandArgs[1:]
	gameGender := "mixed"
	multiGameSize := 32

	// validate multi game options
	if len(commandArgs) > 0 {

		for _, arg := range commandArgs {

			// gender check
			if gender, ok := gameGenders[arg]; ok == true {
				gameGender = gender
				continue
			}

			// game size check
			if requestedGameSize, err := strconv.Atoi(arg); err == nil {
				if _, ok := allowedMultiGameSizes[requestedGameSize]; ok == true {

					multiGameSize = requestedGameSize
					continue
				} else {

					helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.game.invalid-game-size-multi"))
					return
				}
			}

			// if a arg was passed that didn't match any check, send invalid args message
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
			return
		}
	}

	var biasChoices []*idols.Idol

	// if this isn't a mixed game then filter all choices by the gender
	if gameGender != "mixed" {

		for _, bias := range idols.GetAllIdols() {
			if bias.Gender == gameGender {
				biasChoices = append(biasChoices, bias)
			}
		}
	} else {
		biasChoices = idols.GetAllIdols()
	}

	// confirm we have enough biases for a multiplayer game
	if len(biasChoices) < multiGameSize {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.game.not-enough-idols"))
		return
	}

	// create new game
	multiGame := &multiBiasGame{
		ChannelID:      msg.ChannelID,
		IdolsRemaining: multiGameSize,
		Gender:         gameGender,
		RoundDelay:     5,
		GameIsRunning:  true,
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
	multiGame.saveGame()

	multiGame.processMultiGame()
}

// sendMultiBiasGameRound sends the next round for the multi game
func (g *multiBiasGame) sendMultiBiasGameRound() error {
	if g == nil {
		return errors.New("Game is nil")
	}

	// if a round message has been sent, delete before sending the next one
	if g.LastRoundMessage != nil {
		cache.GetSession().ChannelMessageDelete(g.LastRoundMessage.ChannelID, g.LastRoundMessage.ID)
	}

	// get random images to use

	img1 := getSemiRandomIdolImage(g.BiasQueue[0], &g.GameImageIndex)
	img2 := getSemiRandomIdolImage(g.BiasQueue[1], &g.GameImageIndex)

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
	encoder.Encode(buf, makeVSImage(img1, img2))
	myReader := bytes.NewReader(buf.Bytes())

	// send round message
	fileSendMsg, err := helpers.SendFile(g.ChannelID, "combined_pic.png", myReader, messageString)

	if err != nil {

		// check if error is a permissions error, if not retry send the round
		if checkPermissionError(err, g.ChannelID) {

			helpers.SendMessage(g.ChannelID, helpers.GetText("bot.errors.no-file"))
			return errors.New("Could not send round")
		} else {

			for i := 0; i < 10; i++ {
				time.Sleep(time.Second * 1)

				myReader = bytes.NewReader(buf.Bytes())
				fileSendMsg, err = helpers.SendFile(g.ChannelID, "combined_pic.png", myReader, messageString)
				if err == nil {
					break
				}
			}

			// if after the retries an error still exists, return
			if err != nil {
				return err
			}
		}
	}

	// add reactions
	cache.GetSession().MessageReactionAdd(g.ChannelID, fileSendMsg[0].ID, LEFT_ARROW_EMOJI)
	cache.GetSession().MessageReactionAdd(g.ChannelID, fileSendMsg[0].ID, RIGHT_ARROW_EMOJI)

	// update game state
	g.CurrentRoundMessageId = fileSendMsg[0].ID
	g.LastRoundMessage = fileSendMsg[0]
	return nil
}

// start multi game loop. every 10 seconds count the number of arrow reactions. whichever side has most wins
func (g *multiBiasGame) processMultiGame() {

	for g.IdolsRemaining != 1 {

		// send next rounds and sleep
		err := g.sendMultiBiasGameRound()
		if err != nil {
			g.GameIsRunning = false
			return
		}
		time.Sleep(time.Second * time.Duration(g.RoundDelay))
		g.GameIsRunning = true

		// get current round message
		message, err := cache.GetSession().ChannelMessage(g.ChannelID, g.CurrentRoundMessageId)
		if err != nil {
			g.GameIsRunning = false
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

		// adjust next round delay based on amount of votes
		g.adjustRoundDelay(leftCount + rightCount)

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
			if winnerIndex == 1 {
				cache.GetSession().MessageReactionAdd(g.ChannelID, g.CurrentRoundMessageId, ARROW_FORWARD_EMOJI)
			} else {
				cache.GetSession().MessageReactionAdd(g.ChannelID, g.CurrentRoundMessageId, ARROW_BACKWARD_EMOJI)
			}
			time.Sleep(time.Millisecond * 1500)
		}

		g.IdolsRemaining--

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
	go func(g *multiBiasGame) {
		defer helpers.Recover()
		recordMultiGamesStats(g)
	}(g)

	g.deleteGame()
}

// removes game from current multi games
func (g *multiBiasGame) deleteGame() {
	currentMultiPlayerGamesMutex.Lock()
	currentMultiPlayerGamesMutex.Unlock()

	for i, game := range currentMultiPlayerGames {
		if game.CurrentRoundMessageId == g.CurrentRoundMessageId {
			currentMultiPlayerGames = append(currentMultiPlayerGames[:i], currentMultiPlayerGames[i+1:]...)
		}
	}
}

// saveGame save to currently running multi games
func (g *multiBiasGame) saveGame() {
	currentMultiPlayerGamesMutex.Lock()
	currentMultiPlayerGamesMutex.Unlock()

	currentMultiPlayerGames = append(currentMultiPlayerGames, g)
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

		ri := resize.Resize(0, resizeTo, getSemiRandomIdolImage(bias, &g.GameImageIndex), resize.Lanczos3)

		draw.Draw(bracketImage, ri.Bounds().Add(bracketImageOffsets[i]), ri, image.ZP, draw.Over)
	}

	// compress bracket image
	buf := new(bytes.Buffer)
	encoder := new(png.Encoder)
	encoder.CompressionLevel = -2 // -2 compression is best speed, -3 is best compression but end result isn't worth the slower encoding
	encoder.Encode(buf, bracketImage)
	myReader := bytes.NewReader(buf.Bytes())

	messageString := fmt.Sprintf("**Multi Game**\nWinner: %s %s!",
		g.GameWinnerBias.GroupName,
		g.GameWinnerBias.BiasName)

	// send message
	helpers.SendFile(g.ChannelID, "biasgame_multi_winner.png", myReader, messageString)
}

// adjusts the next round delay based on the amount of votes
//  will only adjust by 1 second at a time so changes are not drastic
func (g *multiBiasGame) adjustRoundDelay(lastRoundVoteCount int) {

	// remove bots reaction count
	lastRoundVoteCount -= 2

	// amount of votes => max delay
	voteCountDelayMap := map[int]int{
		0:  3,
		1:  4,
		2:  5,
		5:  6,
		12: 7,
	}

	// get desired delay based on vote count
	var targetDelay int
	for reqVoteCount, delay := range voteCountDelayMap {
		if lastRoundVoteCount >= reqVoteCount && delay > targetDelay {
			targetDelay = delay
		}
	}

	// if the new target delay is different than the current, adjust by 1 second
	if g.RoundDelay < targetDelay {
		g.RoundDelay++
	} else if g.RoundDelay > targetDelay {
		g.RoundDelay--
	}
}

///////////////////////
// UTILITY FUNCTIONS //
///////////////////////

// gets a specific single player game for a UserID
//  if the User currently has no game ongoing it will return nil
//  will delete the game if a nil game is found
func getSinglePlayerGameByUserID(userID string) *singleBiasGame {
	if userID == "" {
		return nil
	}

	currentSinglePlayerGamesMutex.RLock()
	game, ok := currentSinglePlayerGames[userID]
	currentSinglePlayerGamesMutex.RUnlock()

	// if a game is found and is nil, delete it
	if ok && game == nil {
		currentSinglePlayerGamesMutex.Lock()
		delete(currentSinglePlayerGames, userID)
		currentSinglePlayerGamesMutex.Unlock()
	}

	return game
}

// gets a specific multi player game for a channelID
//  returns nil if no games were found in the channel
func getMultiPlayerGameByChannelID(channelID string) *multiBiasGame {
	if channelID == "" {
		return nil
	}

	for _, game := range getCurrentMultiPlayerGames() {
		if game.ChannelID == channelID {
			return game
		}
	}

	return nil
}

// gets all currently ongoing single player games
func getCurrentSinglePlayerGames() map[string]*singleBiasGame {
	currentSinglePlayerGamesMutex.RLock()
	defer currentSinglePlayerGamesMutex.RUnlock()

	// copy data to prevent race conditions
	gamesCopy := make(map[string]*singleBiasGame)
	for key, value := range currentSinglePlayerGames {
		gamesCopy[key] = value
	}

	return gamesCopy
}

// gets all currently ongoing multi player games
func getCurrentMultiPlayerGames() []*multiBiasGame {
	currentMultiPlayerGamesMutex.RLock()
	defer currentMultiPlayerGamesMutex.RUnlock()

	// copy data to prevent race conditions
	gamesCopy := make([]*multiBiasGame, len(currentMultiPlayerGames))
	for i, value := range currentMultiPlayerGames {
		gamesCopy[i] = value
	}

	return gamesCopy
}

// startCacheRefreshLoop will refresh the cache for biasgames
func startCacheRefreshLoop() {
	bgLog().Info("Starting biasgame current games cache loop")
	go func() {
		defer helpers.Recover()

		for {
			time.Sleep(time.Second * 30)

			// save any currently running games
			err := setBiasGameCache("currentSinglePlayerGames", getCurrentSinglePlayerGames(), 0)
			helpers.Relax(err)
			bgLog().Infof("Cached %d singleplayer biasgames to redis", len(getCurrentSinglePlayerGames()))

			err = setBiasGameCache("currentMultiPlayerGames", getCurrentMultiPlayerGames(), 0)
			helpers.Relax(err)
			bgLog().Infof("Cached %d multiplayer biasgames to redis", len(getCurrentMultiPlayerGames()))
		}
	}()
}

// loadMiscImages handles loading other images besides the idol images
func loadMiscImages() {
	var crown image.Image

	validMiscImages := []string{
		"verses.png",
		"top-eight-bracket.png",
		"shadow-border.png",
		"crown.png",
	}

	miscImagesFolderPath := helpers.GetConfig().Path("assets_folder").Data().(string) + "biasgame/misc/"

	// load misc images
	for _, fileName := range validMiscImages {

		// check if file exists
		filePath := miscImagesFolderPath + fileName
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			helpers.Relax(err)
		}

		// open file and decode it
		file, err := os.Open(filePath)
		helpers.Relax(err)
		img, _, err := image.Decode(file)
		helpers.Relax(err)

		// resize misc images as needed
		switch fileName {
		case "verses.png":
			versesImage = resize.Resize(0, IMAGE_RESIZE_HEIGHT+30, img, resize.Lanczos3)
		case "shadow-border.png":
			shadowBorder = resize.Resize(0, IMAGE_RESIZE_HEIGHT+30, img, resize.Lanczos3)
		case "crown.png":
			crown = resize.Resize(IMAGE_RESIZE_HEIGHT/2, 0, img, resize.Lanczos3)
		case "top-eight-bracket.png":
			winnerBracket = img
		}
		bgLog().Infof("Loading biasgame misc image: %s", fileName)
	}

	// append crown to top eight
	bracketImage := image.NewRGBA(winnerBracket.Bounds())
	draw.Draw(bracketImage, winnerBracket.Bounds(), winnerBracket, image.Point{0, 0}, draw.Src)
	draw.Draw(bracketImage, crown.Bounds().Add(image.Pt(230, 5)), crown, image.ZP, draw.Over)
	winnerBracket = bracketImage.SubImage(bracketImage.Rect)
}

// makeVSImage will make the image that shows for rounds in the biasgame
func makeVSImage(img1, img2 image.Image) image.Image {
	// resize images if needed
	if img1.Bounds().Dy() != IMAGE_RESIZE_HEIGHT || img2.Bounds().Dy() != IMAGE_RESIZE_HEIGHT {
		img1 = resize.Resize(0, IMAGE_RESIZE_HEIGHT, img1, resize.Lanczos3)
		img2 = resize.Resize(0, IMAGE_RESIZE_HEIGHT, img2, resize.Lanczos3)
	}

	// give shadow border
	img1 = giveImageShadowBorder(img1, 15, 15)
	img2 = giveImageShadowBorder(img2, 15, 15)

	// combind images
	img1 = helpers.CombineTwoImages(img1, versesImage)
	return helpers.CombineTwoImages(img1, img2)
}
