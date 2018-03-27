package modules

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/generator"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/modules/plugins/levels"
	"github.com/Seklfreak/Robyul2/ratelimits"
	"github.com/bwmarrin/discordgo"
)

// Init warms the caches and initializes the plugins
func Init(session *discordgo.Session) {
	checkDuplicateCommands()

	pluginCount := len(PluginList)
	extendedPluginCount := len(PluginExtendedList)
	pluginCache = make(map[string]*Plugin)
	extendedPluginCache = make(map[string]*ExtendedPlugin)

	logTemplate := "[PLUG] %s reacts to [ %s]"
	listeners := ""

	for i := 0; i < pluginCount; i++ {
		ref := &PluginList[i]

		for _, cmd := range (*ref).Commands() {
			pluginCache[cmd] = ref
			listeners += cmd + " "
		}

		cache.GetLogger().WithField("module", "modules").Info(fmt.Sprintf(
			logTemplate,
			helpers.Typeof(*ref),
			listeners,
		))
		listeners = ""

		(*ref).Init(session)
	}

	listeners = ""
	logTemplate = "[EXTENDED-PLUG] %s reacts to [ %s]"
	for i := 0; i < extendedPluginCount; i++ {
		ref := &PluginExtendedList[i]

		for _, cmd := range (*ref).Commands() {
			extendedPluginCache[cmd] = ref
			listeners += cmd + " "
		}

		cache.GetLogger().WithField("module", "modules").Info(fmt.Sprintf(
			logTemplate,
			helpers.Typeof(*ref),
			listeners,
		))
		listeners = ""

		if helpers.Typeof(*ref) == "*Levels" {
			generator.SetProfileGenerator((*ref).(*levels.Levels))
		}

		(*ref).Init(session)
	}

	pluginCommands := make([]string, 0)
	for k := range pluginCache {
		pluginCommands = append(pluginCommands, k)
	}
	cache.SetPluginList(pluginCommands)
	extendedPluginCommands := make([]string, 0)
	for k := range extendedPluginCache {
		extendedPluginCommands = append(extendedPluginCommands, k)
	}
	cache.SetPluginExtendedList(extendedPluginCommands)

	cache.GetLogger().WithField("module", "modules").Info(
		"modules",
		"Initializer finished. Loaded "+strconv.Itoa(len(PluginList))+" plugins and "+strconv.Itoa(len(PluginExtendedList))+" extended plugins",
	)
}

// Uninit deintializes the plugins
func Uninit(session *discordgo.Session) {
	extendedPluginCount := len(PluginExtendedList)
	extendedPluginCache = make(map[string]*ExtendedPlugin)

	logTemplate := "[EXTENDED-PLUG] %s deintializing…"
	for i := 0; i < extendedPluginCount; i++ {
		ref := &PluginExtendedList[i]

		for _, cmd := range (*ref).Commands() {
			extendedPluginCache[cmd] = ref
		}

		cache.GetLogger().WithField("module", "modules").Info(fmt.Sprintf(
			logTemplate,
			helpers.Typeof(*ref),
		))

		(*ref).Uninit(session)
	}

	cache.GetLogger().WithField("module", "modules").Info(
		"modules",
		"Uninit finished. Unitialized "+strconv.Itoa(len(PluginExtendedList))+" extended plugins",
	)
}

// command - The command that triggered this execution
// content - The content without command
// msg     - The message object
// session - The discord session
func CallBotPlugin(command string, content string, msg *discordgo.Message) {
	// Defer a recovery in case anything panics
	defer helpers.RecoverDiscord(msg)

	// Consume a key for this action
	ratelimits.Container.Drain(1, msg.Author.ID)

	// Track metrics
	metrics.CommandsExecuted.Add(1)

	// Call the module
	if ref, ok := pluginCache[command]; ok {
		(*ref).Action(command, content, msg, cache.GetSession())
	}
	// call the extended module
	if ref, ok := extendedPluginCache[command]; ok {
		(*ref).Action(command, content, msg, cache.GetSession())
	}
}

func CallExtendedPlugin(content string, msg *discordgo.Message) {
	defer helpers.Recover()

	for _, extendedPlugin := range PluginExtendedList {
		extendedPlugin.OnMessage(strings.TrimSpace(content), msg, cache.GetSession())
	}
	//go safePluginExtendedCall(strings.TrimSpace(content), msg, plug)
}

func CallExtendedPluginOnMessageDelete(message *discordgo.MessageDelete) {
	defer helpers.Recover()

	for _, extendedPlugin := range PluginExtendedList {
		extendedPlugin.OnMessageDelete(message, cache.GetSession())
	}
}

func CallExtendedPluginOnGuildMemberAdd(member *discordgo.Member) {
	defer helpers.Recover()

	// Iterate over all plugins
	for _, extendedPlugin := range PluginExtendedList {
		extendedPlugin.OnGuildMemberAdd(member, cache.GetSession())
	}
}
func CallExtendedPluginOnGuildMemberRemove(member *discordgo.Member) {
	defer helpers.Recover()

	// Iterate over all plugins
	for _, extendedPlugin := range PluginExtendedList {
		extendedPlugin.OnGuildMemberRemove(member, cache.GetSession())
	}
}
func CallExtendedPluginOnReactionAdd(reaction *discordgo.MessageReactionAdd) {
	defer helpers.Recover()

	// Iterate over all plugins
	for _, extendedPlugin := range PluginExtendedList {
		extendedPlugin.OnReactionAdd(reaction, cache.GetSession())
	}
}
func CallExtendedPluginOnReactionRemove(reaction *discordgo.MessageReactionRemove) {
	defer helpers.Recover()

	// Iterate over all plugins
	for _, extendedPlugin := range PluginExtendedList {
		extendedPlugin.OnReactionRemove(reaction, cache.GetSession())
	}
}
func CallExtendedPluginOnGuildBanAdd(user *discordgo.GuildBanAdd) {
	defer helpers.Recover()

	// Iterate over all plugins
	for _, extendedPlugin := range PluginExtendedList {
		extendedPlugin.OnGuildBanAdd(user, cache.GetSession())
	}
}
func CallExtendedPluginOnGuildBanRemove(user *discordgo.GuildBanRemove) {
	defer helpers.Recover()

	// Iterate over all plugins
	for _, extendedPlugin := range PluginExtendedList {
		extendedPlugin.OnGuildBanRemove(user, cache.GetSession())
	}
}

func checkDuplicateCommands() {
	cmds := make(map[string]string)

	for _, plug := range PluginList {
		for _, cmd := range plug.Commands() {
			t := helpers.Typeof(plug)

			if occupant, ok := cmds[cmd]; ok {
				cache.GetLogger().WithField("module", "modules").Info("Failed to load " + t + " because '" + cmd + "' was already registered by " + occupant)
				os.Exit(1)
			}

			cmds[cmd] = t
		}
	}
}
