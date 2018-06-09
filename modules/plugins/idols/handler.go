package idols

// Init when the bot starts up
import (
	"regexp"
	"strings"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

// module struct
type Module struct{}

var gameGenders map[string]string

func (i *Module) Init(session *discordgo.Session) {
	go func() {
		defer helpers.Recover()

		// compile commonly used regex
		var err error
		alphaNumericRegex, err = regexp.Compile("[^a-zA-Z0-9]+")
		helpers.Relax(err)

		// load all idol images and information
		refreshIdols(false)

		// start loop to refresh idol cache
		startCacheRefreshLoop()

		// load aliases
		initAliases()

		// set up suggestions channel
		initSuggestionChannel()
	}()
}

// Uninit called when bot is shutting down
func (i *Module) Uninit(session *discordgo.Session) {

}

// Will validate if the passed command entered is used for this plugin
func (i *Module) Commands() []string {
	return []string{
		"idol",
	}
}

// Main Entry point for the plugin
func (i *Module) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermGames) {
		return
	}

	commandArgs := strings.Fields(content)

	if command == "idol" {

		switch commandArgs[0] {
		case "migrate-idols":
			helpers.RequireRobyulMod(msg, func() {
				migrateIdols(msg, content)
			})
		case "suggest":
			processImageSuggestion(msg, content)
		case "delete-image":
			helpers.RequireRobyulMod(msg, func() {
				deleteImage(msg, content)
			})
		case "update-image":
			helpers.RequireRobyulMod(msg, func() {
				updateImageInfo(msg, content)
			})
		case "update-group":

			helpers.RequireRobyulMod(msg, func() {
				updateGroupInfo(msg, content)
			})
		case "update":

			helpers.RequireRobyulMod(msg, func() {
				updateIdolInfoFromMsg(msg, content)
			})
		case "refresh-idols-old":
			helpers.RequireRobyulMod(msg, func() {
				newMessages, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.refresh.refresing"))
				helpers.Relax(err)
				refreshIdolsFromOld(true)

				cache.GetSession().ChannelMessageDelete(msg.ChannelID, newMessages[0].ID)
				helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.refresh.refresh-done"))
			})

		case "refresh-idols":

			helpers.RequireRobyulMod(msg, func() {
				newMessages, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.refresh.refresing"))
				helpers.Relax(err)
				refreshIdols(true)

				cache.GetSession().ChannelMessageDelete(msg.ChannelID, newMessages[0].ID)
				helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.biasgame.refresh.refresh-done"))
			})
		case "images", "image", "pic", "pics", "img", "imgs":

			showImagesForIdol(msg, content, false)
		case "image-ids", "images-ids", "pic-ids", "pics-ids":

			// shows images with object ids
			helpers.RequireRobyulMod(msg, func() {
				showImagesForIdol(msg, content, true)
			})
		case "list":

			listIdolsInGame(msg)
		case "alias":

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
		}
	} else if command == "sug-edit" { // edit is used for changing details of suggestions
		fieldToUpdate := commandArgs[0]
		fieldValue := strings.Join(commandArgs[1:], " ")
		updateSuggestionDetails(msg, fieldToUpdate, fieldValue)
	}

}

// Called whenever a reaction is added to any message
func (i *Module) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {
	defer helpers.Recover()
	if reaction == nil {
		return
	}

	// check if this was a reaction to a idol suggestion.
	checkSuggestionReaction(reaction)
}

///// Unused functions requried by ExtendedPlugin interface
func (i *Module) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {
}
func (i *Module) OnMessageDelete(msg *discordgo.MessageDelete, session *discordgo.Session) {
}
func (i *Module) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {
}
func (i *Module) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {
}
func (i *Module) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {
}
func (i *Module) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {
}
func (i *Module) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {
}
