package biasgame

// Init when the bot starts up
import (
	"image"
	"strconv"
	"strings"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

// module struct
type BiasGame struct{}

// used to stop commands from going through
//  before the game is ready after a bot restart
var moduleIsReady = false

func (b *BiasGame) Init(session *discordgo.Session) {
	go func() {
		defer helpers.Recover()

		// set global variables
		currentSinglePlayerGames = make(map[string]*singleBiasGame)
		allowedGameSizes = map[int]bool{
			32:   true,
			64:   true,
			128:  true,
			256:  true,
			512:  true,
			1024: true,
		}
		allowedMultiGameSizes = map[int]bool{
			32: true,
			64: true,
		}
		// allow games with the size of 10 in debug mode
		if helpers.DEBUG_MODE {
			allowedGameSizes[10] = true
			allowedMultiGameSizes[10] = true
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
		loadMiscImages()

		startCacheRefreshLoop()

		// get any in progress games saved in cache and immediatly delete them
		currentSinglePlayerGamesMutex.Lock()
		getBiasGameCache("currentSinglePlayerGames", &currentSinglePlayerGames)
		currentSinglePlayerGamesMutex.Unlock()
		currentMultiPlayerGamesMutex.Lock()
		getBiasGameCache("currentMultiPlayerGames", &currentMultiPlayerGames)
		currentMultiPlayerGamesMutex.Unlock()
		bgLog().Infof("restored %d singleplayer biasgames on launch", len(getCurrentSinglePlayerGames()))
		bgLog().Infof("restored %d multiplayer biasgames on launch", len(getCurrentMultiPlayerGames()))

		// start any multi games
		for _, multiGame := range getCurrentMultiPlayerGames() {
			go func(multiGame *multiBiasGame) {
				defer helpers.Recover()
				multiGame.processMultiGame()
			}(multiGame)
		}

		moduleIsReady = true

		// load aliases
		initAliases()

		// set up suggestions channel
		initSuggestionChannel()
	}()
}

// Uninit called when bot is shutting down
func (b *BiasGame) Uninit(session *discordgo.Session) {

	// save any currently running games
	err := setBiasGameCache("currentSinglePlayerGames", getCurrentSinglePlayerGames(), 0)
	helpers.Relax(err)

	err = setBiasGameCache("currentMultiPlayerGames", getCurrentMultiPlayerGames(), 0)
	helpers.Relax(err)

	bgLog().Infof("stored %d singleplayer biasgames on shutdown", len(getCurrentSinglePlayerGames()))
	bgLog().Infof("stored %d multiplayer biasgames on shutdown", len(getCurrentMultiPlayerGames()))
}

// Will validate if the passed command entered is used for this plugin
func (b *BiasGame) Commands() []string {
	return []string{
		"biasgame",
		"biasgame-edit",
	}
}

// Main Entry point for the plugin
func (b *BiasGame) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermGames) {
		return
	}

	// images, suggestions, and stat set up are done async when bot starts up
	//   make sure game is ready before trying to process any commands
	if moduleIsReady == false {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.game.game-not-ready"))
		return
	}

	// process text after the initial command
	commandArgs := strings.Fields(content)
	if command == "biasgame" {

		if len(commandArgs) == 0 {
			// start default bias game
			singleGame := createOrGetSinglePlayerGame(msg, commandArgs)
			singleGame.sendBiasGameRound()

		} else if commandArgs[0] == "group-stats" {

			displayGroupStats(msg, content)
		} else if commandArgs[0] == "idol-stats" {
			displayIdolStats(msg, content)

		} else if commandArgs[0] == "stats" {

			// stats
			displayBiasGameStats(msg, content)

		} else if isCommandAlias(commandArgs[0], "server-rankings") {

			showRankings(msg, commandArgs, true)

		} else if isCommandAlias(commandArgs[0], "rankings") {

			showRankings(msg, commandArgs, false)

		} else if commandArgs[0] == "suggest" {

			processImageSuggestion(msg, content)

		} else if commandArgs[0] == "migrate-drive-images" {

			helpers.RequireRobyulMod(msg, func() {
				runGoogleDriveMigration(msg)
			})

		} else if commandArgs[0] == "delete-image" {

			helpers.RequireRobyulMod(msg, func() {
				deleteBiasImage(msg, content)
			})

		} else if commandArgs[0] == "update-image" {

			helpers.RequireRobyulMod(msg, func() {
				updateImageInfo(msg, content)
			})

		} else if commandArgs[0] == "update-group" {

			helpers.RequireRobyulMod(msg, func() {
				updateGroupInfo(msg, content)
			})

		} else if commandArgs[0] == "update-stats" {

			helpers.RequireRobyulMod(msg, func() {
				updateGameStatsFromMsg(msg, content)
			})

		} else if commandArgs[0] == "update" {

			helpers.RequireRobyulMod(msg, func() {
				updateIdolInfoFromMsg(msg, content)
			})

		} else if isCommandAlias(commandArgs[0], "image-ids") {

			// shows images with object ids
			helpers.RequireRobyulMod(msg, func() {
				showImagesForIdol(msg, content, true)
			})

		} else if isCommandAlias(commandArgs[0], "images") {

			showImagesForIdol(msg, content, false)

		} else if commandArgs[0] == "alias" {

			if len(commandArgs) < 2 {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				return
			}

			switch commandArgs[1] {
			case "add":
				helpers.RequireRobyulMod(msg, func() {
					addGroupAlias(msg, content)
				})
				break
			case "list":
				listGroupAliases(msg)
				break
			case "delete", "del":
				helpers.RequireRobyulMod(msg, func() {
					deleteGroupAlias(msg, content)
				})
				break
			default:
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
			}

		} else if isCommandAlias(commandArgs[0], "current") {
			displayCurrentGameStats(msg)

		} else if isCommandAlias(commandArgs[0], "multi") {

			startMultiPlayerGame(msg, commandArgs)

		} else if commandArgs[0] == "idols" {

			listIdolsInGame(msg)

		} else if commandArgs[0] == "refresh-images" {

			helpers.RequireRobyulMod(msg, func() {
				newMessages, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.refresh.refresing"))
				helpers.Relax(err)
				refreshBiasChoices(true)

				cache.GetSession().ChannelMessageDelete(msg.ChannelID, newMessages[0].ID)
				helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.refresh.refresh-done"))
			})

		} else if _, err := strconv.Atoi(commandArgs[0]); err == nil {

			singleGame := createOrGetSinglePlayerGame(msg, commandArgs)
			singleGame.sendBiasGameRound()

		} else if _, ok := biasGameGenders[commandArgs[0]]; ok {

			singleGame := createOrGetSinglePlayerGame(msg, commandArgs)
			singleGame.sendBiasGameRound()
		}
	} else if command == "biasgame-edit" { // edit is used for changing details of suggestions
		fieldToUpdate := commandArgs[0]
		fieldValue := strings.Join(commandArgs[1:], " ")
		UpdateSuggestionDetails(msg, fieldToUpdate, fieldValue)
	}
}

// Called whenever a reaction is added to any message
func (b *BiasGame) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {
	defer helpers.Recover()
	if moduleIsReady == false || reaction == nil {
		return
	}

	// confirm the reaction was added to a message for one bias games
	if game := getSinglePlayerGameByUserID(reaction.UserID); game != nil {
		game.processVote(reaction)
	}

	// check if this was a reaction to a idol suggestion.
	//  if it was accepted an image will be returned to be added to the biasChoices
	CheckSuggestionReaction(reaction)
}

// holds aliases for commands
func isCommandAlias(input, targetCommand string) bool {
	// if input is already the same as target command no need to check aliases
	if input == targetCommand {
		return true
	}

	var aliasMap = map[string]string{
		"images": "images",
		"image":  "images",
		"pic":    "images",
		"pics":   "images",
		"img":    "images",
		"imgs":   "images",

		"image-ids":  "image-ids",
		"images-ids": "image-ids",
		"pic-ids":    "image-ids",
		"pics-ids":   "image-ids",

		"rankings": "rankings",
		"ranking":  "rankings",
		"rank":     "rankings",
		"ranks":    "rankings",

		"current": "current",
		"cur":     "current",

		"multi":       "multi",
		"multiplayer": "multi",

		"server-rankings": "server-rankings",
		"server-ranking":  "server-rankings",
		"server-ranks":    "server-rankings",
		"server-rank":     "server-rankings",
	}

	if attemptedCommand, ok := aliasMap[input]; ok {
		return attemptedCommand == targetCommand
	}

	return false
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
