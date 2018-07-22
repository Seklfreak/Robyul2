package nugugame

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"time"

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
		// todo: maybe send a message here letting the user know they have a game going?
		log().Warnln("nugu game found for user...")
		return
	}

	// todo set this back to mixed
	// gameGender := "mixed"
	gameGender := "girl"
	isMulti := false
	gameType := "idol"

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
	}
	game.GameImageIndex = make(map[string]int)
	game.GuessChannel = make(chan *discordgo.Message)
	game.TimeoutChannel = time.NewTimer(NUGUGAME_DEFULT_ROUND_DELAY * time.Second)

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
	if g.LastRoundMessage != nil {
		go helpers.DeleteMessageWithDelay(g.LastRoundMessage, NUGUGAME_ROUND_DELETE_DELAY)
	}

	// get a random idol to send round for
	g.CurrentIdol = g.getNewRandomIdol()

	// get an image for the current idol and resize it
	idolImage := g.CurrentIdol.GetResizedRandomImage(NUGUGAME_IMAGE_RESIZE_HEIGHT)

	roundMessage := "What is the idols name?"
	if g.GameType == "group" {
		roundMessage = "What is the idols group name?"
	}
	if !g.IsMultigame {
		roundMessage = fmt.Sprintf("**@%s**\nCurrent Score: %d\n%s", g.User.Username, len(g.CorrectIdols), roundMessage)
	} else {
		roundMessage = fmt.Sprintf("**Multi Game**\nCurrent Score: %d\n%s", len(g.CorrectIdols), roundMessage)
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
}

// waitforguess will watch the users messages in the channel for correct guess
func (g *nuguGame) watchForGuesses() {
	log().Println("waiting for nugu game guess...")

	go func() {
		defer helpers.Recover()

		// watch for user input
		for {

			select {
			case userMsg := <-g.GuessChannel:
				log().Infoln("User Message: ", userMsg)

				// if guess is correct add green check mark too it, save the correct guess, and send next round
				userGuess := strings.ToLower(alphaNumericRegex.ReplaceAllString(userMsg.Content, ""))

				var correctAnswers []string
				if g.GameType == "group" {
					correctAnswers = []string{g.CurrentIdol.GroupName}

					// add aliases as acceptable answers
					if hasAliases, aliases := idols.GetAlisesForGroup(g.CurrentIdol.GroupName); hasAliases {
						correctAnswers = append(correctAnswers, aliases...)
					}
				} else {
					correctAnswers = []string{g.CurrentIdol.Name}
					correctAnswers = append(correctAnswers, g.CurrentIdol.NameAliases...)
				}

				log().Printf("--- Guess given: %s, %s, %s, %s", userMsg.Content, userGuess, g.CurrentIdol.Name, correctAnswers)

				// check if the user guess contains the idols name
				for _, correctAnswer := range correctAnswers {

					correctAnswer = strings.ToLower(alphaNumericRegex.ReplaceAllString(correctAnswer, ""))
					if userGuess == correctAnswer && g.WaitingForGuess {
						g.WaitingForGuess = false

						if g.IsMultigame {

							cache.GetSession().MessageReactionAdd(g.ChannelID, userMsg.ID, CHECKMARK_EMOJI)
						} else {
							go helpers.DeleteMessageWithDelay(userMsg, NUGUGAME_ROUND_DELETE_DELAY)
						}

						g.CorrectIdols = append(g.CorrectIdols, g.CurrentIdol)
						g.sendRound()

						// clear timeout channel and reset timer
						if !g.TimeoutChannel.Stop() {
							<-g.TimeoutChannel.C
						}
						g.TimeoutChannel.Reset(NUGUGAME_DEFULT_ROUND_DELAY * time.Second)
						break
					}
				}

				// do nothing if the user message doesn't match, they could just be talking...

			case <-g.TimeoutChannel.C:
				helpers.SendMessage(g.ChannelID, fmt.Sprintf("Game Over. The idol was: %s %s", g.CurrentIdol.GroupName, g.CurrentIdol.Name))
				g.deleteGame()
				return
			}
		}
	}()
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

	for _, games := range currentNuguGames {
		for i, game := range games {
			if game.UUID == g.UUID {
				currentNuguGames[game.ChannelID] = append(currentNuguGames[game.ChannelID][:i], currentNuguGames[game.ChannelID][i+1:]...)
			}
		}
	}
}

// getNewRandomIdol will get a random idol for the game, respecting game options and not duplicating previous idols
func (g *nuguGame) getNewRandomIdol() *idols.Idol {
	var idol *idols.Idol
	var idolPool []*idols.Idol

	if !helpers.DEBUG_MODE {

		// if this isn't a mixed game then filter all choices by the gender
		if g.Gender != "mixed" {
			for _, bias := range idols.GetActiveIdols() {
				if bias.Gender == g.Gender {
					idolPool = append(idolPool, bias)
				}
			}
		} else {
			idolPool = idols.GetActiveIdols()
		}

	} else {
		testGroups := []string{
			"Pristin",
			"CLC",
			"TWICE",
			"Apink",
			"BLΛƆKPIИK",
			"Red Velvet",
		}

		for _, bias := range idols.GetActiveIdols() {
			for _, group := range testGroups {
				if bias.GroupName == group {
					idolPool = append(idolPool, bias)
				}
			}
		}
	}

	// get random idol for the game
RandomIdolLoop:
	for true {
		randomIdol := idolPool[rand.Intn(len(idolPool))]

		// if the random idol found matches one the game has had previous then skip it
		for _, previousGuesses := range g.CorrectIdols {
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
