package modules

import (
    "github.com/bwmarrin/discordgo"
    "git.lukas.moe/sn0w/Karen/ratelimits"
    "git.lukas.moe/sn0w/Karen/helpers"
    "git.lukas.moe/sn0w/Karen/cache"
    "git.lukas.moe/sn0w/Karen/metrics"
    "fmt"
    "git.lukas.moe/sn0w/Karen/logger"
    "strconv"
)

// Init warms the caches and initializes the plugins
func Init(session *discordgo.Session) {
    pluginCount := len(PluginList)
    triggerCount := len(TriggerPluginList)
    pluginCache = make(map[string]*Plugin)
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

    logger.INFO.L(
        "modules",
        "Initializer finished. Loaded " + strconv.Itoa(len(PluginList)) + " plugins and " + strconv.Itoa(len(TriggerPluginList)) + " triggers",
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
        (*ref).Response(trigger, content)
    }
}
