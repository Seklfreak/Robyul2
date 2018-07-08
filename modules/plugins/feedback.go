package plugins

import (
	"strings"

	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/VojtechVitek/go-trello"
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

type feedbackAction func(command string, args []string, in *discordgo.Message, out **discordgo.MessageSend) (next feedbackAction)

type Feedback struct{}

func (f *Feedback) Commands() []string {
	return []string{
		"feedback",
		"suggestion",
		"suggest",
		"issue",
		"bug",
	}
}

var (
	trelloClient          *trello.Client
	trelloListSuggestions *trello.List
	trelloListIssues      *trello.List
)

func (f *Feedback) Init(session *discordgo.Session) {
	var err error
	token := helpers.GetConfig().Path("trello.token").Data().(string)
	trelloClient, err = trello.NewAuthClient(
		helpers.GetConfig().Path("trello.key").Data().(string),
		&token)
	helpers.Relax(err)

	trelloListSuggestions, err = trelloClient.List(
		helpers.GetConfig().Path("trello.board-ids.suggestions").Data().(string))
	helpers.Relax(err)
	trelloListIssues, err = trelloClient.List(
		helpers.GetConfig().Path("trello.board-ids.issues").Data().(string))
	helpers.Relax(err)
}

func (f *Feedback) Uninit(session *discordgo.Session) {

}

func (f *Feedback) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermFeedback) {
		return
	}

	session.ChannelTyping(msg.ChannelID)

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := f.actionStart
	for action != nil {
		action = action(command, args, msg, &result)
	}
}

func (f *Feedback) actionStart(command string, args []string, in *discordgo.Message, out **discordgo.MessageSend) feedbackAction {
	cache.GetSession().ChannelTyping(in.ChannelID)

	if len(args) < 1 {
		*out = f.newMsg("plugins.feedback.arguments-too-few")
		return f.actionFinish
	}

	switch command {
	case "suggestion", "suggest", "feedback":
		if len(args) >= 1 && args[0] == "set-log" && helpers.IsRobyulMod(in.Author.ID) {
			return f.actionSetLog
		}
		return f.actionSuggestion
	case "issue", "bug":
		return f.actionIssue
	}

	*out = f.newMsg("bot.arguments.invalid")
	return f.actionFinish
}

func (f *Feedback) actionSuggestion(command string, args []string, in *discordgo.Message, out **discordgo.MessageSend) feedbackAction {
	parts := strings.Split(in.Content, command)
	content := strings.TrimSpace(strings.Join(parts[1:], command))

	cardName := content
	cardDesc := ""

	// check for description
	if strings.Contains(content, "|") {
		cardName = strings.Split(content, "|")[0]
		cardDesc = strings.Join(strings.Split(content, "|")[1:], "|")
	}

	card, err := trelloListSuggestions.AddCard(trello.Card{
		Name: strings.TrimSpace(cardName),
		Desc: strings.TrimSpace(cardDesc),
	})
	helpers.Relax(err)

	f.logger().Infof("user #%s submitted a suggestion: %s", in.Author.ID, content)

	// log suggestion to discord
	logChannelID, err := helpers.GetBotConfigString(models.FeedbackLogChannelKey)
	if err == nil && logChannelID != "" {
		_, err = helpers.SendEmbed(logChannelID, getLogEmbed("New Suggestion: "+cardName, cardDesc, card.Url, in.Author))
		helpers.RelaxLog(err)
	}

	*out = f.newMsg("plugins.feedback.suggestion-received")
	return f.actionFinish
}

func (f *Feedback) actionIssue(command string, args []string, in *discordgo.Message, out **discordgo.MessageSend) feedbackAction {
	parts := strings.Split(in.Content, command)
	content := strings.TrimSpace(strings.Join(parts[1:], command))

	cardName := content
	cardDesc := ""

	// check for description
	if strings.Contains(content, "|") {
		cardName = strings.Split(content, "|")[0]
		cardDesc = strings.Join(strings.Split(content, "|")[1:], "|")
	}

	card, err := trelloListIssues.AddCard(trello.Card{
		Name: strings.TrimSpace(cardName),
		Desc: strings.TrimSpace(cardDesc),
	})
	helpers.Relax(err)

	f.logger().Infof("user #%s submitted an issue: %s", in.Author.ID, content)

	// log issue to discord
	logChannelID, err := helpers.GetBotConfigString(models.FeedbackLogChannelKey)
	if err == nil && logChannelID != "" {
		_, err = helpers.SendEmbed(logChannelID, getLogEmbed("New Issue: "+cardName, cardDesc, card.Url, in.Author))
		helpers.RelaxLog(err)
	}

	*out = f.newMsg("plugins.feedback.issue-received")
	return f.actionFinish
}

func (f *Feedback) actionSetLog(command string, args []string, in *discordgo.Message, out **discordgo.MessageSend) feedbackAction {
	if !helpers.IsRobyulMod(in.Author.ID) {
		*out = f.newMsg("robyulmod.no_permission")
		return f.actionFinish
	}

	var err error
	var targetChannel *discordgo.Channel
	if len(args) >= 2 {
		targetChannel, err = helpers.GetChannelFromMention(in, args[1])
		helpers.Relax(err)
	}

	if targetChannel != nil && targetChannel.ID != "" {
		err = helpers.SetBotConfigString(models.FeedbackLogChannelKey, targetChannel.ID)
	} else {
		err = helpers.SetBotConfigString(models.FeedbackLogChannelKey, "")
	}

	*out = f.newMsg("plugins.feedback.setlog-success")
	return f.actionFinish
}

func getLogEmbed(title, description, url string, author *discordgo.User) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		URL:         url,
		Title:       title,
		Description: description,
		Timestamp:   time.Now().Format(time.RFC3339),
		Author: &discordgo.MessageEmbedAuthor{
			Name:    author.Username + "#" + author.Discriminator + " (#" + author.ID + ")",
			IconURL: author.AvatarURL(""),
		},
	}
}

func (f *Feedback) actionFinish(command string, args []string, in *discordgo.Message, out **discordgo.MessageSend) feedbackAction {
	_, err := helpers.SendComplex(in.ChannelID, *out)
	helpers.RelaxMessage(err, in.ChannelID, in.ID)

	return nil
}

func (f *Feedback) newMsg(content string) *discordgo.MessageSend {
	return &discordgo.MessageSend{Content: helpers.GetText(content)}
}

func (f *Feedback) logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "feedback")
}
