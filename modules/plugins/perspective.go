package plugins

import (
	"strings"

	"encoding/json"

	"fmt"

	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type perspectiveAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next perspectiveAction)

type Perspective struct {
	googleApiKey  string
	guildsToCheck []string
}

const (
	PerspectiveThreshold = 0.75
)

func (m *Perspective) Commands() []string {
	return []string{
		"perspective",
	}
}

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
	defer helpers.Recover()

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
	severeToxicity, inflammatory, obscene, err := m.analyze(message)
	helpers.Relax(err)
	took := time.Since(start)

	var severeToxicityWarning, inflammatoryWarning, obsceneWarning string
	if severeToxicity >= PerspectiveThreshold {
		severeToxicityWarning = " ⚠"
	}
	if inflammatory >= PerspectiveThreshold {
		inflammatoryWarning = " ⚠"
	}
	if obscene >= PerspectiveThreshold {
		obsceneWarning = " ⚠"
	}

	*out = m.newMsg(fmt.Sprintf(
		"Severe Toxicity: %.2f%s, Inflammatory: %.2f%s, Obscene %.2f%s\nTook %s",
		severeToxicity, severeToxicityWarning,
		inflammatory, inflammatoryWarning,
		obscene, obsceneWarning,
		took.String(),
	))
	return m.actionFinish
}

// [p]perspective participate [<#channel or channel id>]
func (m *Perspective) actionParticipate(args []string, in *discordgo.Message, out **discordgo.MessageSend) perspectiveAction {
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
	severeToxicity, inflammatory, obscene, err := m.analyze(msg.Content)
	helpers.Relax(err)
	// check threshold
	if severeToxicity < PerspectiveThreshold &&
		inflammatory < PerspectiveThreshold &&
		obscene < PerspectiveThreshold {
		return
	}
	// debug
	m.logger().Debugf("Severe Toxicity: %.2f, Inflammatory: %.2f, Obscene %.2f, message: %s",
		severeToxicity, inflammatory, obscene, msg.Content,
	)
	// send warning
	err = m.sendWarning(channel.GuildID, msg, severeToxicity, inflammatory, obscene)
	helpers.RelaxLog(err)
}

func (m *Perspective) sendWarning(guildID string, message *discordgo.Message, severeToxicity, inflammatory, obscene float64) (err error) {
	settings := helpers.GuildSettingsGetCached(guildID)

	if !settings.PerspectiveIsParticipating || settings.PerspectiveChannelID == "" {
		return errors.New("guild is not participating")
	}

	timestamp, err := message.Timestamp.Parse()
	if err != nil {
		timestamp = time.Now()
	}

	severeToxicityWarning := "✅ "
	inflammatoryWarning := "✅ "
	obsceneWarning := "✅ "
	if severeToxicity >= PerspectiveThreshold {
		severeToxicityWarning = "⚠ "
	}
	if inflammatory >= PerspectiveThreshold {
		inflammatoryWarning = "⚠ "
	}
	if obscene >= PerspectiveThreshold {
		obsceneWarning = "⚠ "
	}

	warningEmbed := &discordgo.MessageEmbed{
		Title:       "detected a message that possibly requires action",
		Description: message.Content,
		Timestamp:   timestamp.Format(time.RFC3339),
		Color:       helpers.GetDiscordColorFromHex("ffb80a"), // orange
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.perspective.embed-footer"),
			IconURL: helpers.GetText("plugins.perspective.embed-footer-imageurl"),
		},
		Author: &discordgo.MessageEmbedAuthor{
			Name:    message.Author.Username + "#" + message.Author.Discriminator + " (#" + message.Author.ID + ")",
			IconURL: message.Author.AvatarURL("64"),
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Severe Toxicity",
				Value:  fmt.Sprintf("%s%.2f", severeToxicityWarning, severeToxicity),
				Inline: true,
			},
			{
				Name:   "Inflammatory",
				Value:  fmt.Sprintf("%s%.2f", inflammatoryWarning, inflammatory),
				Inline: true,
			},
			{
				Name:   "Obscene",
				Value:  fmt.Sprintf("%s%.2f", obsceneWarning, obscene),
				Inline: true,
			},
			{
				Name: "Channel", Value: "<#" + message.ChannelID + ">", Inline: false,
			},
		},
	}

	_, err = helpers.SendEmbed(settings.PerspectiveChannelID, warningEmbed)
	return err
}

func (m *Perspective) analyze(message string) (severeToxicity, inflammatory, obscene float64, err error) {
	// TODO: strip emoji
	requestData := &PerspectiveRequest{}
	requestData.Comment.Text = message
	requestData.Languages = []string{"en"}

	marshalled, err := json.Marshal(requestData)
	if err != nil {
		return 0, 0, 0, err
	}

	resultData, err := helpers.NetPostUAWithError(
		"https://commentanalyzer.googleapis.com/v1alpha1/comments:analyze?key="+m.googleApiKey,
		string(marshalled),
		helpers.DEFAULT_UA,
	)
	metrics.PerspectiveApiRequests.Add(1)
	if err != nil {
		return 0, 0, 0, err
	}

	var response PerspectiveResponse
	err = json.Unmarshal(resultData, &response)
	if err != nil {
		return 0, 0, 0, err
	}

	return response.AttributeScores.SevereToxicity.SummaryScore.Value,
		response.AttributeScores.Inflammatory.SummaryScore.Value,
		response.AttributeScores.Obscene.SummaryScore.Value,
		nil
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
