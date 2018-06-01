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
	NUGUGAME_DEFULT_ROUND_DELAY  = 15
	NUGUGAME_ROUND_DELETE_DELAY  = 1 * time.Second
	CHECKMARK_EMOJI              = "✅"
)

var currentNuguGames []*nuguGame
var currentNuguGamesMutex sync.RWMutex

///////////////////
//   NUGU GAME   //
///////////////////

// startNuguGame will create and the start the nugu game for the user
func startNuguGame(msg *discordgo.Message, commandArgs []string) {
	log().Println("starting nugu game...")

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
		UUID:              newID.String(),
		User:              msg.Author,
		ChannelID:         msg.ChannelID,
		Gender:            gameGender,
		WaitingForMessage: false,
		RoundDelay:        NUGUGAME_DEFULT_ROUND_DELAY,
		IsMultigame:       isMulti,
		GameType:          gameType,
	}
	game.GameImageIndex = make(map[string]int)

	game.saveGame()
	game.sendRound()
}

// sendRound sends the next round in the game
func (g *nuguGame) sendRound() {
	log().Println("Sending nugu game round...")

	// if already waiting for user message, do not send the next round
	if g.WaitingForMessage == true {
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
	g.WaitingForMessage = true
	g.LastRoundMessage = fileSendMessage[0]
	g.watchForGuess()
}

// waitforguess will watch the users messages in the channel for correct guess
func (g *nuguGame) watchForGuess() {
	log().Println("waiting for nugu game guess...")

	go func() {
		defer helpers.Recover()

		// set up time out channel
		timeoutChan := make(chan int)
		go func() {
			time.Sleep(time.Second * g.RoundDelay)
			timeoutChan <- 0
		}()

		// watch for user input
		for {
			userInputChan := make(chan *discordgo.MessageCreate)
			cache.GetSession().AddHandlerOnce(func(_ *discordgo.Session, e *discordgo.MessageCreate) {
				userInputChan <- e
			})

			select {
			case userMsg := <-userInputChan:
				// confirm the message belongs to the user and is sent in the games channel
				if userMsg.ChannelID != g.ChannelID || (!g.IsMultigame && userMsg.Author.ID != g.User.ID) {
					continue
				}

				// if guess is correct add green check mark too it, save the correct guess, and send next round
				re := regexp.MustCompile("[^a-zA-Z0-9]+")

				userGuess := strings.ToLower(re.ReplaceAllString(userMsg.Content, ""))

				correctAnswer := strings.ToLower(re.ReplaceAllString(g.CurrentIdol.BiasName, ""))
				if g.GameType == "group" {
					correctAnswer = strings.ToLower(re.ReplaceAllString(g.CurrentIdol.GroupName, ""))
				}

				log().Printf("--- Guess given: %s, %s, %s, %s", userMsg.Content, userGuess, g.CurrentIdol.BiasName, correctAnswer)

				// check if the user guess contains the idols name
				if userGuess == correctAnswer {

					if g.IsMultigame {

						cache.GetSession().MessageReactionAdd(g.ChannelID, userMsg.ID, CHECKMARK_EMOJI)
					} else {
						go helpers.DeleteMessageWithDelay(userMsg.Message, NUGUGAME_ROUND_DELETE_DELAY)
					}

					g.CorrectIdols = append(g.CorrectIdols, g.CurrentIdol)
					g.WaitingForMessage = false
					g.sendRound()
					return
				}

				// do nothing if the user message doesn't match, they could just be talking...

			case <-timeoutChan:
				helpers.SendMessage(g.ChannelID, fmt.Sprintf("Game Over. The idol was: %s %s", g.CurrentIdol.GroupName, g.CurrentIdol.BiasName))
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

	currentNuguGames = append(currentNuguGames, g)
}

// deleteGame will delete the game from the current nugu games
func (g *nuguGame) deleteGame() {
	currentNuguGamesMutex.Lock()
	currentNuguGamesMutex.Unlock()

	for i, game := range currentNuguGames {
		if game.UUID == g.UUID {
			currentNuguGames = append(currentNuguGames[:i], currentNuguGames[i+1:]...)
		}
	}
}

// getNewRandomIdol will get a random idol for the game, respecting game options and not duplicating previous idols
func (g *nuguGame) getNewRandomIdol() *idols.Idol {
	var idol *idols.Idol
	var idolPool []*idols.Idol

	if true || !helpers.DEBUG_MODE {

		// if this isn't a mixed game then filter all choices by the gender
		if g.Gender != "mixed" {
			for _, bias := range idols.GetAllIdols() {
				if bias.Gender == g.Gender {
					idolPool = append(idolPool, bias)
				}
			}
		} else {
			idolPool = idols.GetAllIdols()
		}

	} else {
		for _, bias := range idols.GetAllIdols() {
			if bias.GroupName == "PRISTIN" || bias.GroupName == "CLC" || bias.GroupName == "TWICE" {
				idolPool = append(idolPool, bias)
			}
		}
	}

	// get random biases for the game
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

// getNuguGameByUserID will return the single player nugu game for the user if they have one in progress
func getNuguGameByUserID(userID string) *nuguGame {
	if userID == "" {
		return nil
	}

	var game *nuguGame

	currentNuguGamesMutex.RLock()
	for _, nuguGame := range currentNuguGames {
		if nuguGame.User != nil && userID == nuguGame.User.ID {
			game = nuguGame
		}
	}
	currentNuguGamesMutex.RUnlock()

	// if a game is found and is nil, delete it
	// if ok && game == nil {
	// 	currentNuguGamesMutex.Lock()
	// 	delete(currentNuguGames, userID)
	// 	currentNuguGamesMutex.Unlock()
	// }

	return game
}
