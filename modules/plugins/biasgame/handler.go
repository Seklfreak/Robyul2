package biasgame

// Init when the bot starts up
import (
	"fmt"
	"image"
	"strconv"
	"strings"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/shardmanager"
	"github.com/bwmarrin/discordgo"
)

// module struct
type Module struct{}

var gameGenders map[string]string

// used to stop commands from going through
//  before the game is ready after a bot restart
var moduleIsReady = false

func (m *Module) Init(session *shardmanager.Manager) {
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

		gameGenders = map[string]string{
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

	}()
}

// Uninit called when bot is shutting down
func (m *Module) Uninit(session *shardmanager.Manager) {

	// save any currently running games
	err := setBiasGameCache("currentSinglePlayerGames", getCurrentSinglePlayerGames(), 0)
	helpers.Relax(err)

	err = setBiasGameCache("currentMultiPlayerGames", getCurrentMultiPlayerGames(), 0)
	helpers.Relax(err)

	bgLog().Infof("stored %d singleplayer biasgames on shutdown", len(getCurrentSinglePlayerGames()))
	bgLog().Infof("stored %d multiplayer biasgames on shutdown", len(getCurrentMultiPlayerGames()))
}

// Will validate if the passed command entered is used for this plugin
func (m *Module) Commands() []string {
	return []string{
		"biasgame",
		"bg",
	}
}

// Main Entry point for the plugin
func (m *Module) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
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
	if command == "biasgame" || command == "bg" {

		if len(commandArgs) == 0 {
			// start default bias game
			singleGame := createOrGetSinglePlayerGame(msg, commandArgs)
			singleGame.sendBiasGameRound()

		} else if commandArgs[0] == "idols" {
			helpers.SendMessage(msg.ChannelID, fmt.Sprintf("This command has changed. Please use: `%sidol list`", helpers.GetPrefixForServer(msg.GuildID)))

		} else if commandArgs[0] == "images" {
			helpers.SendMessage(msg.ChannelID, fmt.Sprintf("This command has changed. Please use: `%sidol images \"Group Name\" \"Idol Name\"`", helpers.GetPrefixForServer(msg.GuildID)))

		} else if commandArgs[0] == "suggest" {
			helpers.SendMessage(msg.ChannelID, fmt.Sprintf("This command has changed. Please use: `%sidol suggest boy/girl \"Group Name\" \"Idol Name\" image url/attachment`", helpers.GetPrefixForServer(msg.GuildID)))

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

		} else if commandArgs[0] == "validate-stats" {

			helpers.RequireRobyulMod(msg, func() {
				validateStats(msg, commandArgs)
			})
		} else if commandArgs[0] == "migrate-games" {

			helpers.RequireRobyulMod(msg, func() {
				runGameMigration(msg, content)
			})
		} else if commandArgs[0] == "update-stats" {

			helpers.RequireRobyulMod(msg, func() {
				updateGameStatsFromMsg(msg, content)
			})

		} else if isCommandAlias(commandArgs[0], "current") {
			displayCurrentGameStats(msg)

		} else if isCommandAlias(commandArgs[0], "multi") {

			startMultiPlayerGame(msg, commandArgs)

		} else if _, err := strconv.Atoi(commandArgs[0]); err == nil {

			singleGame := createOrGetSinglePlayerGame(msg, commandArgs)
			singleGame.sendBiasGameRound()

		} else if _, ok := gameGenders[commandArgs[0]]; ok {

			singleGame := createOrGetSinglePlayerGame(msg, commandArgs)
			singleGame.sendBiasGameRound()
		}
	}
}

// Called whenever a reaction is added to any message
func (m *Module) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {
	defer helpers.Recover()
	if moduleIsReady == false || reaction == nil {
		return
	}

	// confirm the reaction was added to a message for one bias games
	if game := getSinglePlayerGameByUserID(reaction.UserID); game != nil {
		game.processVote(reaction)
	}
}

// holds aliases for commands
func isCommandAlias(input, targetCommand string) bool {
	// if input is already the same as target command no need to check aliases
	if input == targetCommand {
		return true
	}

	var aliasMap = map[string]string{
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
func (m *Module) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {
}
func (m *Module) OnMessageDelete(msg *discordgo.MessageDelete, session *discordgo.Session) {
}
func (m *Module) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {
}
func (m *Module) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {
}
func (m *Module) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {
}
func (m *Module) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {
}
func (m *Module) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {
}
