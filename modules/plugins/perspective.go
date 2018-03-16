package plugins

import (
	"strings"

	"encoding/json"

	"fmt"

	"time"

	"sync"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type perspectiveAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next perspectiveAction)

type Perspective struct {
	googleApiKey            string
	guildsToCheck           []string
	messageResultsCache     map[string][]PerspectiveMessageResult // map[guildid-channelid-userid][]PerspectiveMessageResult
	messageResultsCacheLock sync.Mutex
}

type PerspectiveMessageResult struct {
	message *discordgo.Message
	result  PerspectiveMessageValues
}

type PerspectiveMessageValues struct {
	SevereToxicity float64
	Inflammatory   float64
	Obscene        float64
}

const (
	PerspectiveThresholdSevereToxicity = 0.60
	PerspectiveThresholdInflammatory   = 0.60
	PerspectiveThresholdObscene        = 0.70
	PerspectiveMessagesToEvaluate      = 3
	PerspectiveEndpointAnalyze         = "https://commentanalyzer.googleapis.com/v1alpha1/comments:analyze"
)

func (m *Perspective) Commands() []string {
	return []string{
		"perspective",
	}
}

// TODO: prevent multiple notifications for the same messages
// TODO: add timeout between specific user messages (don't notify for old messages)

func (m *Perspective) Init(session *discordgo.Session) {
	m.googleApiKey = helpers.GetConfig().Path("google.api_key").Data().(string)
	go func() {
		defer helpers.Recover()

		time.Sleep(60 * time.Second)
		err := m.cacheGuildsToCheck()
		helpers.Relax(err)
		m.logger().Info("initialised guilds to check cache")
	}()
}

func (m *Perspective) Uninit(session *discordgo.Session) {

}

func (m *Perspective) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	session.ChannelTyping(msg.ChannelID)

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := m.actionStart
	for action != nil {
		action = action(args, msg, &result)
	}
}

func (m *Perspective) actionStart(args []string, in *discordgo.Message, out **discordgo.MessageSend) perspectiveAction {
	cache.GetSession().ChannelTyping(in.ChannelID)

	if len(args) < 1 {
		*out = m.newMsg("bot.arguments.too-few")
		return m.actionFinish
	}

	switch args[0] {
	case "participate":
		return m.actionParticipate
	}

	return m.actionTest
}

// [p]perspective <message>
func (m *Perspective) actionTest(args []string, in *discordgo.Message, out **discordgo.MessageSend) perspectiveAction {
	message := strings.TrimSpace(strings.Replace(in.Content, strings.Split(in.Content, " ")[0], "", 1))

	start := time.Now()
	messageResults, err := m.analyze(message)
	helpers.Relax(err)
	took := time.Since(start)

	var severeToxicityWarning, inflammatoryWarning, obsceneWarning string
	if messageResults.SevereToxicity >= PerspectiveThresholdSevereToxicity {
		severeToxicityWarning = " ⚠"
	}
	if messageResults.Inflammatory >= PerspectiveThresholdInflammatory {
		inflammatoryWarning = " ⚠"
	}
	if messageResults.Obscene >= PerspectiveThresholdObscene {
		obsceneWarning = " ⚠"
	}

	*out = m.newMsg(fmt.Sprintf(
		"Severe Toxicity: %.2f%s, Inflammatory: %.2f%s, Obscene: %.2f%s\nTook %s",
		messageResults.SevereToxicity, severeToxicityWarning,
		messageResults.Inflammatory, inflammatoryWarning,
		messageResults.Obscene, obsceneWarning,
		took.String(),
	))
	return m.actionFinish
}

// [p]perspective participate [<#channel or channel id>]
func (m *Perspective) actionParticipate(args []string, in *discordgo.Message, out **discordgo.MessageSend) perspectiveAction {
	// TODO: remove robyul mod check in the future
	if !helpers.IsRobyulMod(in.Author.ID) {
		*out = m.newMsg("robyulmod.no_permission")
		return m.actionFinish
	}
	if !helpers.IsAdmin(in) {
		*out = m.newMsg("admin.no_permission")
		return m.actionFinish
	}

	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)

	settings := helpers.GuildSettingsGetCached(channel.GuildID)

	if settings.PerspectiveIsParticipating && len(args) < 2 {
		// disable perspective checking
		settings.PerspectiveIsParticipating = false
		settings.PerspectiveChannelID = ""
		*out = m.newMsg("plugins.perspective.participation-disabled")
	} else {
		// enable perspective checking
		if len(args) < 2 {
			*out = m.newMsg("bot.arguments.too-few")
			return m.actionFinish
		}

		targetChannel, err := helpers.GetChannelFromMention(in, args[1])
		helpers.Relax(err)

		settings.PerspectiveIsParticipating = true
		settings.PerspectiveChannelID = targetChannel.ID
		*out = m.newMsg("plugins.perspective.participation-enabled")
	}

	err = helpers.GuildSettingsSet(channel.GuildID, settings)
	helpers.Relax(err)

	err = m.cacheGuildsToCheck()
	helpers.Relax(err)

	return m.actionFinish
}

func (m *Perspective) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {
	// ignore bots
	if msg.Author.Bot {
		return
	}
	channel, err := helpers.GetChannel(msg.ChannelID)
	if err != nil {
		return
	}
	// ignore commands
	prefix := helpers.GetPrefixForServer(channel.GuildID)
	if prefix != "" {
		if strings.HasPrefix(content, prefix) {
			return
		}
	}
	// ignore music bot prefixes
	if strings.HasPrefix(content, "__") || strings.HasPrefix(content, "//") {
		return
	}
	// ignore guilds that aren't participating
	var participating bool
	for _, guildToCheck := range m.guildsToCheck {
		if guildToCheck == channel.GuildID {
			participating = true
			break
		}
	}
	if !participating {
		return
	}
	// analyze
	messageResult, err := m.analyze(msg.Content)
	helpers.Relax(err)
	// add message + results to cache
	m.addMessageToCache(channel.GuildID, channel.ID, msg.Author.ID, msg, messageResult)
	// calculate means
	meanResults := m.calculatedCachedMessagesMean(channel.GuildID, channel.ID, msg.Author.ID)
	// debug
	//m.logger().Debugf("Severe Toxicity: %.2f, Inflammatory: %.2f, Obscene: %.2f, message: %s, mean (last %d): Severe Toxicity: %.2f, Inflammatory: %.2f, Obscene: %.2f",
	//	messageResult.SevereToxicity, messageResult.Inflammatory, messageResult.Obscene, msg.Content,
	//	PerspectiveMessagesToEvaluate, meanResults.SevereToxicity, meanResults.Inflammatory, meanResults.Obscene,
	//)
	// check threshold
	if meanResults.SevereToxicity >= PerspectiveThresholdSevereToxicity ||
		meanResults.Inflammatory >= PerspectiveThresholdInflammatory ||
		meanResults.Obscene >= PerspectiveThresholdObscene {
		// send warning
		err = m.sendWarning(channel.GuildID, channel.ID, msg.Author.ID, meanResults)
		helpers.RelaxLog(err)
	}
}

func (m *Perspective) addMessageToCache(guildID, channelID, userID string, msg *discordgo.Message, results PerspectiveMessageValues) {
	m.messageResultsCacheLock.Lock()
	defer m.messageResultsCacheLock.Unlock()
	// init variables
	key := guildID + "-" + channelID + "-" + userID
	newEntry := PerspectiveMessageResult{
		message: msg,
		result:  results,
	}
	// initialize slice if needed
	if m.messageResultsCache == nil {
		m.messageResultsCache = make(map[string][]PerspectiveMessageResult, 0)
	}
	if _, ok := m.messageResultsCache[key]; !ok || m.messageResultsCache[key] == nil {
		m.messageResultsCache[key] = make([]PerspectiveMessageResult, 0)
	}
	// add to slice
	m.messageResultsCache[key] = append(m.messageResultsCache[key], newEntry)
	// truncate slice if too big
	if len(m.messageResultsCache[key]) > PerspectiveMessagesToEvaluate {
		m.messageResultsCache[key] = m.messageResultsCache[key][len(m.messageResultsCache[key])-PerspectiveMessagesToEvaluate:]
	}
}

func (m *Perspective) calculatedCachedMessagesMean(guildID, channelID, userID string) (meanResults PerspectiveMessageValues) {
	m.messageResultsCacheLock.Lock()
	defer m.messageResultsCacheLock.Unlock()
	// init variables
	key := guildID + "-" + channelID + "-" + userID
	// check if cache exists
	if _, ok := m.messageResultsCache[key]; !ok || m.messageResultsCache[key] == nil || len(m.messageResultsCache[key]) < 1 {
		return meanResults
	}
	// cache big enough?
	if len(m.messageResultsCache[key]) < PerspectiveMessagesToEvaluate {
		return meanResults
	}
	// calculate means
	for _, cachedMessage := range m.messageResultsCache[key] {
		meanResults.SevereToxicity += cachedMessage.result.SevereToxicity
		meanResults.Inflammatory += cachedMessage.result.Inflammatory
		meanResults.Obscene += cachedMessage.result.Obscene
	}
	meanResults.SevereToxicity = meanResults.SevereToxicity / float64(len(m.messageResultsCache[key]))
	meanResults.Inflammatory = meanResults.Inflammatory / float64(len(m.messageResultsCache[key]))
	meanResults.Obscene = meanResults.Obscene / float64(len(m.messageResultsCache[key]))
	return meanResults
}

func (m *Perspective) sendWarning(guildID, channelID, userID string, avgResults PerspectiveMessageValues) (err error) {
	settings := helpers.GuildSettingsGetCached(guildID)

	if !settings.PerspectiveIsParticipating || settings.PerspectiveChannelID == "" {
		return errors.New("guild is not participating")
	}

	m.messageResultsCacheLock.Lock()
	defer m.messageResultsCacheLock.Unlock()
	// init variables
	key := guildID + "-" + channelID + "-" + userID
	// check if cache exists
	if _, ok := m.messageResultsCache[key]; !ok || m.messageResultsCache[key] == nil || len(m.messageResultsCache[key]) < 1 {
		return nil
	}

	author := m.messageResultsCache[key][0].message.Author
	lastMessageTimestamp, err := m.messageResultsCache[key][len(m.messageResultsCache[key])-1].message.Timestamp.Parse()
	if err != nil {
		lastMessageTimestamp = time.Now()
	}

	warningEmbed := &discordgo.MessageEmbed{
		Title:       "detected messages by user that possibly requires action",
		Description: "",
		Timestamp:   lastMessageTimestamp.Format(time.RFC3339), // TODO: is last message timestamp?
		Color:       helpers.GetDiscordColorFromHex("ffb80a"),  // orange
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Avg Severe Toxicity: %.2f, Inflammatory: %.2f, Obscene: %.2f | ",
				avgResults.SevereToxicity, avgResults.Inflammatory, avgResults.Obscene) +
				helpers.GetText("plugins.perspective.embed-footer"),
			IconURL: helpers.GetText("plugins.perspective.embed-footer-imageurl"),
		},
		Author: &discordgo.MessageEmbedAuthor{
			Name:    author.Username + "#" + author.Discriminator + " (#" + author.ID + ")",
			IconURL: author.AvatarURL("64"),
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: "Channel", Value: "<#" + channelID + ">", Inline: false,
			},
		},
	}

	var severeToxicityWarning, inflammatoryWarning, obsceneWarning string
	for _, cachedMessage := range m.messageResultsCache[key] {
		severeToxicityWarning = "✅"
		inflammatoryWarning = "✅"
		obsceneWarning = "✅"
		if cachedMessage.result.SevereToxicity >= PerspectiveThresholdSevereToxicity {
			severeToxicityWarning = "⚠"
		}
		if cachedMessage.result.Inflammatory >= PerspectiveThresholdInflammatory {
			inflammatoryWarning = "⚠"
		}
		if cachedMessage.result.Obscene >= PerspectiveThresholdObscene {
			obsceneWarning = "⚠"
		}
		messageTimestamp, err := cachedMessage.message.Timestamp.Parse()
		if err != nil {
			messageTimestamp = time.Now()
		}
		warningEmbed.Description += severeToxicityWarning + inflammatoryWarning + obsceneWarning + " `" + messageTimestamp.Format(time.ANSIC) + " UTC`: `" + cachedMessage.message.Content + "`\n"
	}
	warningEmbed.Description += "`Severe Toxicity` / `Inflammatory` / `Obscene`"

	_, err = helpers.SendEmbed(settings.PerspectiveChannelID, warningEmbed)
	return err
}

func (m *Perspective) analyze(message string) (results PerspectiveMessageValues, err error) {
	// TODO: strip emoji
	requestData := &PerspectiveRequest{}
	requestData.Comment.Text = message
	requestData.Languages = []string{"en"}

	marshalled, err := json.Marshal(requestData)
	if err != nil {
		return results, err
	}

	resultData, err := helpers.NetPostUAWithError(
		PerspectiveEndpointAnalyze+"?key="+m.googleApiKey,
		string(marshalled),
		helpers.DEFAULT_UA,
	)
	metrics.PerspectiveApiRequests.Add(1)
	if err != nil {
		return results, err
	}

	var response PerspectiveResponse
	err = json.Unmarshal(resultData, &response)
	if err != nil {
		return results, err
	}

	return PerspectiveMessageValues{
		SevereToxicity: response.AttributeScores.SevereToxicity.SummaryScore.Value,
		Inflammatory:   response.AttributeScores.Inflammatory.SummaryScore.Value,
		Obscene:        response.AttributeScores.Obscene.SummaryScore.Value,
	}, nil
}

func (m *Perspective) cacheGuildsToCheck() (err error) {
	newGuildsToCheck := make([]string, 0)
	for _, guild := range cache.GetSession().State.Guilds {
		settings := helpers.GuildSettingsGetCached(guild.ID)
		if settings.PerspectiveIsParticipating {
			newGuildsToCheck = append(newGuildsToCheck, guild.ID)
		}
	}

	m.guildsToCheck = newGuildsToCheck

	return nil
}

func (m *Perspective) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) perspectiveAction {
	_, err := helpers.SendComplex(in.ChannelID, *out)
	helpers.Relax(err)

	return nil
}

func (m *Perspective) OnMessageDelete(msg *discordgo.MessageDelete, session *discordgo.Session) {

}

func (m *Perspective) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {

}

func (m *Perspective) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {

}

func (m *Perspective) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {

}

func (m *Perspective) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {

}

func (m *Perspective) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {

}

func (m *Perspective) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}

func (m *Perspective) newMsg(content string) *discordgo.MessageSend {
	return &discordgo.MessageSend{Content: helpers.GetText(content)}
}

func (m *Perspective) Relax(err error) {
	if err != nil {
		panic(err)
	}
}

func (m *Perspective) logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "perspective")
}

type PerspectiveRequest struct {
	Comment struct {
		Text string `json:"text"`
	} `json:"comment"`
	Languages           []string `json:"languages"`
	RequestedAttributes struct {
		SevereToxicity struct{} `json:"SEVERE_TOXICITY"`
		Inflammatory   struct{} `json:"INFLAMMATORY"`
		Obscene        struct{} `json:"OBSCENE"`
	} `json:"requestedAttributes"`
}

type PerspectiveResponse struct {
	AttributeScores struct {
		SevereToxicity struct {
			SummaryScore struct {
				Value float64 `json:"value"`
				Type  string  `json:"type"`
			} `json:"summaryScore"`
		} `json:"SEVERE_TOXICITY"`
		Inflammatory struct {
			SummaryScore struct {
				Value float64 `json:"value"`
				Type  string  `json:"type"`
			} `json:"summaryScore"`
		} `json:"INFLAMMATORY"`
		Obscene struct {
			SummaryScore struct {
				Value float64 `json:"value"`
				Type  string  `json:"type"`
			} `json:"summaryScore"`
		} `json:"OBSCENE"`
	} `json:"attributeScores"`
}
