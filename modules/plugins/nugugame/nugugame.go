package nugugame

import (
	"fmt"
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/globalsign/mgo/bson"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/modules/plugins/idols"
	"github.com/bwmarrin/discordgo"
)

const (
	NUGUGAME_IMAGE_RESIZE_HEIGHT = 200
	NUGUGAME_DEFULT_ROUND_DELAY  = 12
	NUGUGAME_ROUND_DELETE_DELAY  = 2 * time.Second
	CHECKMARK_EMOJI              = "✅"
	SINGLE_NUGUGAME_CACHE_KEY    = "currentSingleNugugames"
	MULTI_NUGUGAME_CACHE_KEY     = "currentMultiNugugames"
)

var currentNuguGames map[string]*nuguGame
var currentNuguGamesMutex sync.RWMutex
var alphaNumericRegex *regexp.Regexp

// startNuguGame Starts a new nugu game or resumes a game in cache
func startNuguGame(msg *discordgo.Message, commandArgs []string) {

	// if the user already has a game, do nothing
	if game := getNuguGameByUserID(msg.Author.ID); game != nil {
		helpers.SendMessage(msg.ChannelID, "You already have a current nugu game running, you must finish it before starting a new one.")
		return
	}

	// if the channel already has a game, do nothing
	if game := getNuguGamesByChannelID(msg.ChannelID); game != nil {
		helpers.SendMessage(msg.ChannelID, "Only one running nugu game is allowed per channel.")
		return
	}

	// default game settings
	gameGender := "mixed"
	isMulti := false
	gameType := "idol"
	gameDifficulty := "medium"
	lives := 3

	// validate game arguments and adjust game settings as needed
	if len(commandArgs) > 0 {
		for _, arg := range commandArgs {

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
				lives = difficultyLives[gameDifficulty]
				continue
			}

			// if a arg was passed that didn't match any check, send invalid args message
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
			return
		}
	}

	var game *nuguGame

	// check if a cached game exists
	if isMulti {
		game = getCachedMultiGame(msg.ChannelID)
	} else {
		game = getCachedSingleGame(msg.Author.ID)
	}

	// check if the user has a cached game, if not then make a new one based on their options
	if game == nil {
		game = &nuguGame{
			Gender:          gameGender,
			WaitingForGuess: false,
			IsMultigame:     isMulti,
			GameType:        gameType,
			Difficulty:      gameDifficulty,
			LivesRemaining:  lives,
		}
	}

	game.ChannelID = msg.ChannelID
	game.User = msg.Author
	game.GuessChannel = make(chan *discordgo.Message)
	game.GuessTimeoutTimer = time.NewTimer(NUGUGAME_DEFULT_ROUND_DELAY * time.Second)

	if game.UsersCorrectGuesses == nil {
		game.UsersCorrectGuesses = make(map[string][]bson.ObjectId)
	}

	game.saveGame()
	game.sendRound()
	game.watchForGuesses()
}

// sendRound sends the next round in the game
func (g *nuguGame) sendRound() {
	if g.WaitingForGuess == true {
		return
	}

	// delete last round message if there was one.
	if g.LastRoundMessage != nil && len(g.CorrectIdols) != 0 && g.CorrectIdols[len(g.CorrectIdols)-1].ID == g.CurrentIdol.ID {
		go helpers.DeleteMessageWithDelay(g.LastRoundMessage, NUGUGAME_ROUND_DELETE_DELAY)
	}

	// get a random idol to send round for
	g.CurrentIdol = g.getNewRandomIdol()

	// if current idol is nil assume we're out of usable idols and end the game
	if g.CurrentIdol == nil {

		// trigger timeout channel to finish game
		if !g.GuessTimeoutTimer.Stop() {
			<-g.GuessTimeoutTimer.C
		}
		g.GuessTimeoutTimer.Reset(time.Nanosecond)
		return
	}

	// Get the correct possible answers for this idol
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
	for i, correctAnswer := range correctAnswers {
		correctAnswers[i] = strings.ToLower(alphaNumericRegex.ReplaceAllString(correctAnswer, ""))
	}
	g.CorrectAnswers = correctAnswers

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
	if !g.GuessTimeoutTimer.Stop() {
		<-g.GuessTimeoutTimer.C
	}
	g.GuessTimeoutTimer.Reset(NUGUGAME_DEFULT_ROUND_DELAY * time.Second)
}

// waitforguess will watch the users messages in the channel for correct guess
func (g *nuguGame) watchForGuesses() {

	go func() {
		defer helpers.Recover()

		// watch for user input
		for {

			select {
			case userMsg := <-g.GuessChannel:

				// To help with async actions happening. g.currentIdol could
				// be set to nil when the game just ends and guesses are still
				// coming in
				currentIdol := g.CurrentIdol
				if currentIdol == nil {
					continue
				}

				// if guess is correct add green check mark too it, save the correct guess, and send next round
				userGuess := strings.ToLower(alphaNumericRegex.ReplaceAllString(userMsg.Content, ""))

				// check if the user guess contains the idols name
				for _, correctAnswer := range g.CorrectAnswers {

					if g.WaitingForGuess && userGuess == correctAnswer {
						g.WaitingForGuess = false

						if g.IsMultigame {

							// if the user guessed nayoung correctly, add cute nayoung emoji instead of a checkmark
							if strings.ToLower(currentIdol.GroupName) == "pristin" && strings.ToLower(currentIdol.Name) == "nayoung" {
								cache.GetSession().MessageReactionAdd(g.ChannelID, userMsg.ID, getRandomNayoungEmoji()) // <3

							} else {
								cache.GetSession().MessageReactionAdd(g.ChannelID, userMsg.ID, CHECKMARK_EMOJI)
							}

							g.UsersCorrectGuesses[userMsg.Author.ID] = append(g.UsersCorrectGuesses[userMsg.Author.ID], currentIdol.ID)

						} else {

							// if the user guessed nayoung correctly, add cute nayoung emoji instead of a checkmark
							if strings.ToLower(currentIdol.GroupName) == "pristin" && strings.ToLower(currentIdol.Name) == "nayoung" {
								cache.GetSession().MessageReactionAdd(g.ChannelID, userMsg.ID, getRandomNayoungEmoji()) // <3
								go helpers.DeleteMessageWithDelay(userMsg, 4*time.Second)

							} else {

								// Delete users guess if its a solo game, helps reduce spam
								go helpers.DeleteMessageWithDelay(userMsg, NUGUGAME_ROUND_DELETE_DELAY)
							}

						}

						g.CorrectIdols = append(g.CorrectIdols, currentIdol)
						g.sendRound()
						break
					}
				}

				// do nothing if the user message doesn't match, they could just be talking...

			case <-g.GuessTimeoutTimer.C:
				g.WaitingForGuess = false

				// check if they have lives remaining for the game
				if g.LivesRemaining > 1 && g.CurrentIdol != nil {
					g.GuessTimeoutTimer.Reset(NUGUGAME_DEFULT_ROUND_DELAY * time.Second)

					helpers.SendMessage(g.ChannelID, fmt.Sprintf("The idol was: %s %s", g.CurrentIdol.GroupName, g.CurrentIdol.Name))

					g.LivesRemaining--
					g.IncorrectIdols = append(g.IncorrectIdols, g.CurrentIdol)
					g.sendRound()

					break
				} else {

					// currentidol could be nil if game ran out of usable idols
					if g.CurrentIdol != nil {
						g.IncorrectIdols = append(g.IncorrectIdols, g.CurrentIdol)
					}

					g.finishGame()
					return
				}
			}
		}
	}()
}

// finishGame will send the final message, record stats, and delete the game
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

			// used to make sure the same user isn't printed twice if their score matches another
			usedUserIds := make(map[string]bool)

			// loop through user scores highest to lowest and append them to final message
			for _, userScore := range userScores {

				for userId, idolIds := range g.UsersCorrectGuesses {

					if len(idolIds) == userScore && !usedUserIds[userId] {

						// get user name
						var userName string
						user, err := helpers.GetUser(userId)
						if err != nil || user == nil {
							userName = "*Unknown*"
						} else {
							userName = user.Username
						}

						finalMessage += fmt.Sprintf("\n%s: %d", userName, userScore)

						usedUserIds[userId] = true
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
	currentNuguGames[g.ChannelID] = g
}

// deleteGame will delete the game from the current nugu games
func (g *nuguGame) deleteGame() {
	currentNuguGamesMutex.Lock()
	delete(currentNuguGames, g.ChannelID)
	currentNuguGamesMutex.Unlock()

	if g.IsMultigame {
		delModuleCache(MULTI_NUGUGAME_CACHE_KEY + g.ChannelID)
	} else {
		delModuleCache(SINGLE_NUGUGAME_CACHE_KEY + g.User.ID)
	}
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
	usedIdols := append(g.CorrectIdols, g.IncorrectIdols...)
	if len(idolPool) == len(usedIdols) {
		return nil
	}

	// get random idol for the game
RandomIdolLoop:
	for true {
		randomIdol := idolPool[rand.Intn(len(idolPool))]

		// if the random idol found matches one the game has had previous then skip it
		for _, previousGuesses := range usedIdols {
			if previousGuesses.NameAndGroup == randomIdol.NameAndGroup {
				continue RandomIdolLoop
			}
		}

		idol = randomIdol
		break
	}

	return idol
}

// quitNuguGame allows a user to quit a nugugame early
func quitNuguGame(msg *discordgo.Message, commandArgs []string) {
	var game *nuguGame

	// confirm the user has a game
	if game = getNuguGameByUserID(msg.Author.ID); game == nil {
		return
	}

	if game.IsMultigame == true {
		helpers.SendMessage(msg.ChannelID, "You may not quit multi games early.")
		return
	}

	// manually set lives left to 1 and force the guess timeout channel to end
	// which will cause another life to be lost and the game to end
	game.LivesRemaining = 1
	if !game.GuessTimeoutTimer.Stop() {
		<-game.GuessTimeoutTimer.C
	}
	game.GuessTimeoutTimer.Reset(time.Nanosecond)
}

// skipNuguGame allows a user to skip a nugugame round
func skipNuguGame(msg *discordgo.Message, commandArgs []string) {
	var game *nuguGame

	// confirm the user has a game
	if game = getNuguGameByUserID(msg.Author.ID); game == nil {
		return
	}

	if game.IsMultigame == true {
		helpers.SendMessage(msg.ChannelID, "You may not skip multi game rounds.")
		return
	}

	// trigger timeout channel to finish round
	if !game.GuessTimeoutTimer.Stop() {
		<-game.GuessTimeoutTimer.C
	}
	game.GuessTimeoutTimer.Reset(time.Nanosecond)
}

///////////////////////
// UTILITY FUNCTIONS //
///////////////////////

// getAllNugugames thread safe get for all games
func getAllNuguGames() map[string]*nuguGame {
	currentNuguGamesMutex.RLock()
	defer currentNuguGamesMutex.RUnlock()
	return currentNuguGames
}

// getNuguGamesByChannelID thread safe get for all games in a channel
func getNuguGamesByChannelID(channelID string) *nuguGame {
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
	for _, nuguGame := range currentNuguGames {
		if nuguGame.User != nil && userID == nuguGame.User.ID {
			game = nuguGame
		}
	}
	currentNuguGamesMutex.RUnlock()
	return game
}

// getCachedGameByUserId will get a cached game from redis for the userid
func getCachedSingleGame(userId string) *nuguGame {

	var cachedNugugame nuguGameForCache
	err := getModuleCache(SINGLE_NUGUGAME_CACHE_KEY+userId, &cachedNugugame)
	if err != nil || cachedNugugame.UserId != userId {
		return nil
	}

	return convertCachedNugugame(cachedNugugame)
}

// getCachedGameByUserId will get a cached game from redis for the userid
func getCachedMultiGame(channelId string) *nuguGame {
	var cachedNugugame nuguGameForCache
	err := getModuleCache(MULTI_NUGUGAME_CACHE_KEY+channelId, &cachedNugugame)
	if err != nil || cachedNugugame.ChannelID != channelId {
		return nil
	}

	return convertCachedNugugame(cachedNugugame)
}

// converts cached nugugame to a real game
func convertCachedNugugame(cachedNugugame nuguGameForCache) *nuguGame {
	currentIdol := idols.GetMatchingIdolById(cachedNugugame.CurrentIdolId)
	if currentIdol == nil {
		return nil
	}

	realNugugame := &nuguGame{
		ChannelID:           cachedNugugame.ChannelID,
		CurrentIdol:         currentIdol,
		Gender:              cachedNugugame.Gender,
		GameType:            cachedNugugame.GameType,
		IsMultigame:         cachedNugugame.IsMultigame,
		Difficulty:          cachedNugugame.Difficulty,
		LivesRemaining:      cachedNugugame.LivesRemaining,
		UsersCorrectGuesses: cachedNugugame.UsersCorrectGuesses,
	}

	for _, idolId := range cachedNugugame.CorrectIdols {
		idol := idols.GetMatchingIdolById(idolId)
		if idol == nil {
			return nil
		}

		realNugugame.CorrectIdols = append(realNugugame.CorrectIdols, idol)
	}

	for _, idolId := range cachedNugugame.IncorrectIdols {
		idol := idols.GetMatchingIdolById(idolId)
		if idol == nil {
			return nil
		}

		realNugugame.IncorrectIdols = append(realNugugame.IncorrectIdols, idol)
	}

	return realNugugame
}

// converts actual games to the cached version for smaller size and to avoid
// issues with dataypes (like the chan)
func convertNugugameToCached(nugugames map[string]*nuguGame) map[string]nuguGameForCache {

	// userid => cached game
	cachedGames := make(map[string]nuguGameForCache)

	for _, game := range nugugames {

		// only need to save games that have actual correct guesses recorded
		if len(game.CorrectIdols) == 0 || game.CurrentIdol == nil {
			continue
		}

		cachedGame := nuguGameForCache{
			UserId:              game.User.ID,
			ChannelID:           game.ChannelID,
			Gender:              game.Gender,
			GameType:            game.GameType,
			IsMultigame:         game.IsMultigame,
			Difficulty:          game.Difficulty,
			LivesRemaining:      game.LivesRemaining,
			CurrentIdolId:       game.CurrentIdol.ID,
			UsersCorrectGuesses: game.UsersCorrectGuesses,
		}

		for _, idol := range game.CorrectIdols {
			cachedGame.CorrectIdols = append(cachedGame.CorrectIdols, idol.ID)
		}

		for _, idol := range game.IncorrectIdols {
			cachedGame.IncorrectIdols = append(cachedGame.IncorrectIdols, idol.ID)
		}

		cachedGames[cachedGame.UserId] = cachedGame
	}

	return cachedGames
}

// startCacheRefreshLoop will refresh the cache for nugugames
func startCacheRefreshLoop() {
	log().Info("Starting nugugame current games cache loop")

	go func() {
		defer helpers.Recover()

		for {
			time.Sleep(time.Second * 30)
			cacheNugugames()
		}
	}()
}

func cacheNugugames() {
	// save any currently running games
	cachedGames := convertNugugameToCached(getAllNuguGames())
	for userId, game := range cachedGames {

		if game.IsMultigame {

			// setting 3 day limit because i don't want people to just randomly stumble across a really old multi game in some channel
			err := setModuleCache(MULTI_NUGUGAME_CACHE_KEY+game.ChannelID, game, time.Hour*72)
			helpers.Relax(err)
		} else {
			err := setModuleCache(SINGLE_NUGUGAME_CACHE_KEY+userId, game, 0)
			helpers.Relax(err)
		}
	}
	log().Infof("Cached %d nugugames to redis", len(cachedGames))
}
