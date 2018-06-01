package nugugame

// Init when the bot starts up
import (
	"strings"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

// module struct
type Module struct{}

var gameGenders map[string]string

func (m *Module) Init(session *discordgo.Session) {
	go func() {

		gameGenders = map[string]string{
			"boy":   "boy",
			"boys":  "boy",
			"girl":  "girl",
			"girls": "girl",
			"mixed": "mixed",
		}
	}()
}

// Uninit called when bot is shutting down
func (m *Module) Uninit(session *discordgo.Session) {

}

// Will validate if the passed command entered is used for this plugin
func (m *Module) Commands() []string {
	return []string{
		"nugugame",
	}
}

// Main Entry point for the plugin
func (m *Module) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermGames) {
		return
	}

	// process text after the initial command
	commandArgs := strings.Fields(content)

	if command == "nugugame" {
		startNuguGame(msg, commandArgs)
	}
}

///// Unused functions requried by ExtendedPlugin interface
func (m *Module) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {
}
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
