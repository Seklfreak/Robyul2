package modules

import (
    "github.com/bwmarrin/discordgo"
    "github.com/Seklfreak/Robyul2/ratelimits"
    "github.com/Seklfreak/Robyul2/helpers"
    "github.com/Seklfreak/Robyul2/cache"
    "github.com/Seklfreak/Robyul2/metrics"
    "fmt"
    "github.com/Seklfreak/Robyul2/logger"
    "strconv"
    "strings"
    "os"
)

// Init warms the caches and initializes the plugins
func Init(session *discordgo.Session) {
    checkDuplicateCommands()

    pluginCount := len(PluginList)
    extendedPluginCount := len(PluginExtendedList)
    triggerCount := len(TriggerPluginList)
    pluginCache = make(map[string]*Plugin)
    extendedPluginCache = make(map[string]*ExtendedPlugin)
    triggerCache = make(map[string]*TriggerPlugin)

    logTemplate := "[PLUG] %s reacts to [ %s]"
    listeners := ""

    for i := 0; i < pluginCount; i++ {
        ref := &PluginList[i]

        for _, cmd := range (*ref).Commands() {
            pluginCache[cmd] = ref
            listeners += cmd + " "
        }

        logger.INFO.L("modules", fmt.Sprintf(
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

        logger.INFO.L("modules", fmt.Sprintf(
            logTemplate,
            helpers.Typeof(*ref),
            listeners,
        ))
        listeners = ""

        (*ref).Init(session)
    }

    logTemplate = "[TRIG] %s gets triggered by [ %s]"
    for i := 0; i < triggerCount; i++ {
        ref := &TriggerPluginList[i]

        for _, trigger := range (*ref).Triggers() {
            triggerCache[trigger] = ref
            listeners += trigger + " "
        }

        logger.INFO.L("modules", fmt.Sprintf(
            logTemplate,
            helpers.Typeof(*ref),
            listeners,
        ))
        listeners = ""
    }

    pluginCommands := make([]string, 0, len(pluginCache))
    for k := range pluginCache {
        pluginCommands = append(pluginCommands, k)
    }
    cache.SetPluginList(pluginCommands)
    extendedPluginCommands := make([]string, 0, len(extendedPluginCache))
    for k := range pluginCache {
        extendedPluginCommands = append(extendedPluginCommands, k)
    }
    cache.SetPluginExtendedList(extendedPluginCommands)
    triggerCommands := make([]string, 0, len(triggerCache))
    for k := range pluginCache {
        triggerCommands = append(triggerCommands, k)
    }
    cache.SetTriggerPluginList(triggerCommands)

    logger.INFO.L(
        "modules",
        "Initializer finished. Loaded "+strconv.Itoa(len(PluginList))+" plugins and "+strconv.Itoa(len(TriggerPluginList))+" triggers",
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

// msg     - The message that triggered the execution
// session - The discord session
func CallTriggerPlugin(trigger string, content string, msg *discordgo.Message) {
    // Defer a recovery in case anything panics
    defer helpers.RecoverDiscord(msg)

    // Consume a key for this action
    ratelimits.Container.Drain(1, msg.Author.ID)

    // Redirect trigger
    if ref, ok := triggerCache[trigger]; ok {
        cache.GetSession().ChannelMessageSend(
            msg.ChannelID,
            (*ref).Response(trigger, content),
        )
    }
}

func CallExtendedPlugin(content string, msg *discordgo.Message) {
    defer helpers.RecoverDiscord(msg)

    for _, extendedPlugin := range PluginExtendedList {
        extendedPlugin.OnMessage(strings.TrimSpace(content), msg, cache.GetSession())
    }
    //go safePluginExtendedCall(strings.TrimSpace(content), msg, plug)
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

func checkDuplicateCommands() {
    cmds := make(map[string]string)

    for _, plug := range PluginList {
        for _, cmd := range plug.Commands() {
            t := helpers.Typeof(plug)

            if occupant, ok := cmds[cmd]; ok {
                logger.ERROR.L("modules", "Failed to load "+t+" because '"+cmd+"' was already registered by "+occupant)
                os.Exit(1)
            }

            cmds[cmd] = t
        }
    }

    for _, trig := range TriggerPluginList {
        for _, cmd := range trig.Triggers() {
            t := helpers.Typeof(trig)

            if occupant, ok := cmds[cmd]; ok {
                logger.ERROR.L("modules", "Failed to load "+t+" because '"+cmd+"' was already registered by "+occupant)
                os.Exit(1)
            }

            cmds[cmd] = t
        }
    }
}
