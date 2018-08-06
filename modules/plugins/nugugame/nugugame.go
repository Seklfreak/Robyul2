package nugugame

import (
	"fmt"
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"

	"github.com/globalsign/mgo/bson"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/modules/plugins/idols"
	"github.com/bwmarrin/discordgo"
	uuid "github.com/satori/go.uuid"
)

const (
	NUGUGAME_IMAGE_RESIZE_HEIGHT = 200
	NUGUGAME_DEFULT_ROUND_DELAY  = 12
	NUGUGAME_ROUND_DELETE_DELAY  = 2 * time.Second
	CHECKMARK_EMOJI              = "✅"
)

var currentNuguGames map[string][]*nuguGame
var currentNuguGamesMutex sync.RWMutex
var alphaNumericRegex *regexp.Regexp

///////////////////
//   NUGU GAME   //
///////////////////

// startNuguGame will create and the start the nugu game for the user
func startNuguGame(msg *discordgo.Message, commandArgs []string) {

	// if the user already has a game, do nothing
	if game := getNuguGameByUserID(msg.Author.ID); game != nil {
		helpers.SendMessage(msg.ChannelID, "You already have a current nugu game running, you must finish it before starting a new one.")
		return
	}

	// if the user already has a game, do nothing
	if game := getNuguGamesByChannelID(msg.ChannelID); game != nil {
		helpers.SendMessage(msg.ChannelID, "Only one running nugu game is allowed per channel.")
		return
	}

	// todo set this back to mixed
	gameGender := "mixed"
	isMulti := false
	gameType := "idol"
	gameDifficulty := "all"
	lives := 5

	// validate game arguments
	if len(commandArgs) > 0 {
		for _, arg := range commandArgs {

			// gender check
			if gender, ok := gameGenders[arg]; ok == true {
				gameGender = gender
				continue
			}

			if arg == "multi" {
				isMulti = true
				continue
			}

			if arg == "group" {
				gameType = "group"
				continue
			}

			if _, ok := idolsByDifficulty[arg]; ok {
				gameDifficulty = arg
				lives = difficultyLives[arg]
				continue
			}

			// if a arg was passed that didn't match any check, send invalid args message
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
			return
		}
	}

	// get unique id for game for deleting
	newID, err := uuid.NewV4()
	if err != nil {
		helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.errors.general", err.Error()))
		return
	}

	game := &nuguGame{
		UUID:            newID.String(),
		User:            msg.Author,
		ChannelID:       msg.ChannelID,
		Gender:          gameGender,
		WaitingForGuess: false,
		RoundDelay:      NUGUGAME_DEFULT_ROUND_DELAY,
		IsMultigame:     isMulti,
		GameType:        gameType,
		Difficulty:      gameDifficulty,
		LivesRemaining:  lives,
	}
	game.GuessChannel = make(chan *discordgo.Message)
	game.TimeoutChannel = time.NewTimer(NUGUGAME_DEFULT_ROUND_DELAY * time.Second)
	game.UsersCorrectGuesses = make(map[string][]bson.ObjectId)

	spew.Dump(game)

	game.saveGame()
	game.sendRound()

	// opens game channels and waits for guesses or timeout to be triggered
	game.watchForGuesses()
}

// sendRound sends the next round in the game
func (g *nuguGame) sendRound() {
	log().Println("Sending nugu game round...")

	// if already waiting for user message, do not send the next round
	if g.WaitingForGuess == true {
		return
	}

	// delete last round message if there was one
	if g.LastRoundMessage != nil && len(g.CorrectIdols) != 0 && g.CorrectIdols[len(g.CorrectIdols)-1].ID == g.CurrentIdol.ID {
		go helpers.DeleteMessageWithDelay(g.LastRoundMessage, NUGUGAME_ROUND_DELETE_DELAY)
	}

	// get a random idol to send round for
	g.CurrentIdol = g.getNewRandomIdol()

	// if current idol is nil assume we're out of usable idols and end hte game
	if g.CurrentIdol == nil {

		// trigger timeout channel to finish game
		if !g.TimeoutChannel.Stop() {
			<-g.TimeoutChannel.C
		}
		g.TimeoutChannel.Reset(time.Nanosecond)
		return
	}

	// get an image for the current idol and resize it
	idolImage := g.CurrentIdol.GetResizedRandomImage(NUGUGAME_IMAGE_RESIZE_HEIGHT)
	idolImage = giveImageShadowBorder(idolImage, 20, 20)

	roundMessage := "What is the idols name?"
	if g.GameType == "group" {
		roundMessage = "What is the idols group name?"
	}
	if !g.IsMultigame {
		roundMessage = fmt.Sprintf("**@%s**\nCurrent Score: %d\nLives Remaining: %d\n%s", g.User.Username, len(g.CorrectIdols), g.LivesRemaining, roundMessage)
	} else {
		roundMessage = fmt.Sprintf("**Multi Game**\nCurrent Score: %d\nLives Remaining: %d\n%s", len(g.CorrectIdols), g.LivesRemaining, roundMessage)
	}

	// send round message
	fileSendMessage, err := helpers.SendFile(g.ChannelID, "idol_image.png", helpers.ImageToReader(idolImage), roundMessage)
	if err != nil {
		if checkPermissionError(err, g.ChannelID) {
			helpers.SendMessage(g.ChannelID, helpers.GetText("bot.errors.no-file"))
		}
		return
	}

	// update game state
	g.WaitingForGuess = true
	g.LastRoundMessage = fileSendMessage[0]

	// clear timeout channel and reset timer
	if !g.TimeoutChannel.Stop() {
		<-g.TimeoutChannel.C
	}
	g.TimeoutChannel.Reset(NUGUGAME_DEFULT_ROUND_DELAY * time.Second)
}

// waitforguess will watch the users messages in the channel for correct guess
func (g *nuguGame) watchForGuesses() {
	log().Println("waiting for nugu game guess...")

	go func() {
		defer helpers.Recover()

		// watch for user input
		for {
			log().Infoln("loop")

			select {
			case userMsg := <-g.GuessChannel:
				log().Infoln("User Message: ", userMsg)

				// to help with async actions happening. g.currentIdol could be set to nil
				currentIdol := g.CurrentIdol
				if currentIdol == nil {
					continue
				}

				// if guess is correct add green check mark too it, save the correct guess, and send next round
				userGuess := strings.ToLower(alphaNumericRegex.ReplaceAllString(userMsg.Content, ""))

				var correctAnswers []string
				if g.GameType == "group" {
					correctAnswers = []string{currentIdol.GroupName}

					// add aliases as acceptable answers
					if hasAliases, aliases := idols.GetAlisesForGroup(currentIdol.GroupName); hasAliases {
						correctAnswers = append(correctAnswers, aliases...)
					}
				} else {
					correctAnswers = []string{currentIdol.Name}
					correctAnswers = append(correctAnswers, currentIdol.NameAliases...)
				}

				log().Printf("--- Guess given: %s, %s, %s, %s", userMsg.Content, userGuess, currentIdol.Name, correctAnswers)

				// check if the user guess contains the idols name
				for _, correctAnswer := range correctAnswers {

					correctAnswer = strings.ToLower(alphaNumericRegex.ReplaceAllString(correctAnswer, ""))
					if userGuess == correctAnswer && g.WaitingForGuess {
						g.WaitingForGuess = false

						if g.IsMultigame {

							cache.GetSession().MessageReactionAdd(g.ChannelID, userMsg.ID, CHECKMARK_EMOJI)
							g.UsersCorrectGuesses[userMsg.Author.ID] = append(g.UsersCorrectGuesses[userMsg.Author.ID], currentIdol.ID)

						} else {
							go helpers.DeleteMessageWithDelay(userMsg, NUGUGAME_ROUND_DELETE_DELAY)
						}

						g.CorrectIdols = append(g.CorrectIdols, currentIdol)
						g.sendRound()
						break
					}
				}

				// do nothing if the user message doesn't match, they could just be talking...

			case <-g.TimeoutChannel.C:
				g.WaitingForGuess = false

				// check if they have lives remaining for the game
				if g.LivesRemaining > 1 && g.CurrentIdol != nil {
					/*msgs, err := */ helpers.SendMessage(g.ChannelID, fmt.Sprintf("The idol was: %s %s", g.CurrentIdol.GroupName, g.CurrentIdol.Name))
					// helpers.Relax(err)
					// go helpers.DeleteMessageWithDelay(msgs[0], NUGUGAME_ROUND_DELETE_DELAY)

					g.LivesRemaining--
					g.IncorrectIdols = append(g.IncorrectIdols, g.CurrentIdol)
					g.TimeoutChannel.Reset(NUGUGAME_DEFULT_ROUND_DELAY * time.Second)
					g.sendRound()

					break
				} else {
					log().Infoln("done.")
					g.finishGame()
					return
				}
			}
		}
	}()
}

// finishGame will send the final message and delete the game
func (g *nuguGame) finishGame() {
	go recordNuguGame(g)
	g.deleteGame()

	// if there is a current idol set, the user missed it and should be printed out what the idol was
	missedIdolMessage := "\nAll idols for this difficulty have been used."
	if g.CurrentIdol != nil {
		missedIdolMessage = fmt.Sprintf("\nThe idol was: %s %s", g.CurrentIdol.GroupName, g.CurrentIdol.Name)
	}

	var finalMessage string
	if !g.IsMultigame {
		finalMessage = fmt.Sprintf("**@%s** Game Over!%s\nFinal Score: %d", g.User.Username, missedIdolMessage, len(g.CorrectIdols))

	} else {
		finalMessage = fmt.Sprintf("**Multi Game** Game Over!%s\nFinal Score: %d", missedIdolMessage, len(g.CorrectIdols))

		if len(g.UsersCorrectGuesses) > 0 {
			finalMessage += "\n__User Scores__"

			// get all scores in array so they can be sorted
			var userScores []int
			for _, idolIds := range g.UsersCorrectGuesses {
				userScores = append(userScores, len(idolIds))
			}
			sort.Sort(sort.Reverse(sort.IntSlice(userScores)))

			// loop through user scores highest to lowest and append them to final message
			for _, userScore := range userScores {

				for userId, idolIds := range g.UsersCorrectGuesses {

					if len(idolIds) == userScore {

						// get user name
						user, err := helpers.GetUser(userId)
						var userName string
						if err != nil || user == nil {
							userName = "*Unknown*"
						} else {
							userName = user.Username
						}

						finalMessage += fmt.Sprintf("\n%s: %d", userName, userScore)

						// remove user so they don't get printed twice if their score matches someone else
						delete(g.UsersCorrectGuesses, userId)
						continue
					}
				}
			}
		}

	}

	helpers.SendMessage(g.ChannelID, finalMessage)
}

// saveGame saves the nugu game to the current running games
func (g *nuguGame) saveGame() {
	currentNuguGamesMutex.Lock()
	defer currentNuguGamesMutex.Unlock()

	currentNuguGames[g.ChannelID] = append(currentNuguGames[g.ChannelID], g)
}

// deleteGame will delete the game from the current nugu games
func (g *nuguGame) deleteGame() {
	currentNuguGamesMutex.Lock()
	defer currentNuguGamesMutex.Unlock()
	delete(currentNuguGames, g.ChannelID)
}

// getNewRandomIdol will get a random idol for the game, respecting game options and not duplicating previous idols
func (g *nuguGame) getNewRandomIdol() *idols.Idol {
	var idol *idols.Idol
	var idolPool []*idols.Idol

	idolIds := getNugugameIdolsByDifficulty(g.Difficulty)
	if len(idolIds) > 0 {
		for _, idolID := range idolIds {
			idolForGame := idols.GetMatchingIdolById(bson.ObjectIdHex(idolID))

			if idolForGame != nil && idolForGame.Deleted == false {
				idolPool = append(idolPool, idolForGame)
			}
		}
	} else {
		idolPool = idols.GetActiveIdols()
	}

	// if this isn't a mixed game then filter all choices by the gender
	if g.Gender != "mixed" {
		var tempIdolPool []*idols.Idol
		for _, bias := range idolPool {
			if bias.Gender == g.Gender {
				tempIdolPool = append(tempIdolPool, bias)
			}
		}
		idolPool = tempIdolPool
	}

	// if there are no more unused idols, end the game
	if len(idolPool) == len(append(g.CorrectIdols, g.IncorrectIdols...)) {
		return nil
	}

	// get random idol for the game
RandomIdolLoop:
	for true {
		randomIdol := idolPool[rand.Intn(len(idolPool))]

		// if the random idol found matches one the game has had previous then skip it
		for _, previousGuesses := range append(g.CorrectIdols, g.IncorrectIdols...) {
			if previousGuesses.NameAndGroup == randomIdol.NameAndGroup {
				continue RandomIdolLoop
			}
		}

		idol = randomIdol
		break
	}

	return idol
}

///////////////////////
// UTILITY FUNCTIONS //
///////////////////////

// getAllNugugames thread safe get for all games
func getAllNuguGames() map[string][]*nuguGame {
	currentNuguGamesMutex.RLock()
	defer currentNuguGamesMutex.RUnlock()
	return currentNuguGames
}

// getNuguGamesByChannelID thread safe get for all games in a channel
func getNuguGamesByChannelID(channelID string) []*nuguGame {
	currentNuguGamesMutex.RLock()
	defer currentNuguGamesMutex.RUnlock()
	return currentNuguGames[channelID]
}

// getNuguGameByUserID will return the single player nugu game for the user if they have one in progress
func getNuguGameByUserID(userID string) *nuguGame {
	if userID == "" {
		return nil
	}

	var game *nuguGame

	currentNuguGamesMutex.RLock()
	for _, games := range currentNuguGames {
		for _, nuguGame := range games {
			if nuguGame.User != nil && userID == nuguGame.User.ID {
				game = nuguGame
			}
		}
	}
	currentNuguGamesMutex.RUnlock()
	return game
}
