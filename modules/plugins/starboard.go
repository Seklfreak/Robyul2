package plugins

import (
	"errors"
	"strings"

	"mvdan.cc/xurls"

	"time"

	"fmt"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	rethink "github.com/gorethink/gorethink"
)

type starboardAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next starboardAction)

type Starboard struct{}

var (
	StarboardEmojis = [...]string{"⭐"}
)

func (s *Starboard) Commands() []string {
	return []string{
		"starboard",
		"sb",
	}
}

func (s *Starboard) Init(session *discordgo.Session) {

}

func (s *Starboard) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	defer helpers.Recover()

	session.ChannelTyping(msg.ChannelID)

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := s.actionStart
	for action != nil {
		action = action(args, msg, &result)
	}
}

func (s *Starboard) actionStart(args []string, in *discordgo.Message, out **discordgo.MessageSend) starboardAction {
	cache.GetSession().ChannelTyping(in.ChannelID)

	if len(args) < 1 {
		*out = s.newMsg("bot.arguments.too-few")
		return s.actionFinish
	}

	switch args[0] {
	case "starrers":
		return s.actionStarrers
	case "top":
		return s.actionTop
	case "status":
		return s.actionStatus
	case "set":
		return s.actionSet
	}

	*out = s.newMsg("bot.arguments.invalid")
	return s.actionFinish
}

func (s *Starboard) actionTop(args []string, in *discordgo.Message, out **discordgo.MessageSend) starboardAction {
	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)

	topEntries, err := s.getTopStarboardEntries(channel.GuildID)
	if err != nil {
		if strings.Contains(err.Error(), "no starboard entries") {
			*out = s.newMsg(helpers.GetText("plguins.starboard.top-no-entries"))
			return s.actionFinish
		} else {
			helpers.Relax(err)
		}
	}

	embed, err := s.getTopMessagesEmbed(topEntries)
	helpers.Relax(err)
	*out = &discordgo.MessageSend{Embed: embed}
	return s.actionFinish
}

func (s *Starboard) actionStarrers(args []string, in *discordgo.Message, out **discordgo.MessageSend) starboardAction {
	if len(args) < 2 {
		*out = s.newMsg(helpers.GetText("bot.arguments.too-few"))
		return s.actionFinish
	}

	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)

	starboardEntry, err := s.getStarboardEntry(channel.GuildID, args[1])
	if err != nil {
		if strings.Contains(err.Error(), "no starboard entry") {
			*out = s.newMsg(helpers.GetText("bot.arguments.invalid"))
			return s.actionFinish
		}
		helpers.Relax(err)
	}

	embed := s.getStarrersEmbed(starboardEntry)
	*out = &discordgo.MessageSend{Embed: embed}
	return s.actionFinish
}

func (s *Starboard) actionStatus(args []string, in *discordgo.Message, out **discordgo.MessageSend) starboardAction {
	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)

	guildSettings := helpers.GuildSettingsGetCached(channel.GuildID)

	if guildSettings.StarboardChannelID != "" {
		*out = s.newMsg(helpers.GetTextF("plugins.starboard.status-set", guildSettings.StarboardChannelID))
	} else {
		*out = s.newMsg(helpers.GetText("plugins.starboard.status-none"))
	}
	return s.actionFinish
}

func (s *Starboard) actionSet(args []string, in *discordgo.Message, out **discordgo.MessageSend) starboardAction {
	if !helpers.IsMod(in) {
		*out = s.newMsg(helpers.GetText("mod.no_permission"))
		return s.actionFinish
	}

	channel, err := helpers.GetChannel(in.ChannelID)
	helpers.Relax(err)

	guildSettings := helpers.GuildSettingsGetCached(channel.GuildID)

	if len(args) < 2 {
		if guildSettings.StarboardChannelID != "" {
			guildSettings.StarboardChannelID = ""
			err = helpers.GuildSettingsSet(channel.GuildID, guildSettings)
			helpers.Relax(err)

			*out = s.newMsg(helpers.GetText("plugins.starboard.reset-success"))
			return s.actionFinish
		} else {
			*out = s.newMsg(helpers.GetText("plugins.starboard.status-none"))
		}
		return s.actionFinish
	}

	targetChannel, err := helpers.GetChannelFromMention(in, args[1])
	if err != nil {
		if strings.Contains(err.Error(), "Channel not found") {
			*out = s.newMsg(helpers.GetText("bot.arguments.invalid"))
			return s.actionFinish
		}
		helpers.Relax(err)
	}
	guildSettings.StarboardChannelID = targetChannel.ID
	err = helpers.GuildSettingsSet(channel.GuildID, guildSettings)
	helpers.Relax(err)

	*out = s.newMsg(helpers.GetTextF("plugins.starboard.set-success", guildSettings.StarboardChannelID))
	return s.actionFinish
}

func (s *Starboard) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) starboardAction {
	_, err := cache.GetSession().ChannelMessageSendComplex(in.ChannelID, *out)
	helpers.Relax(err)

	return nil
}

func (s *Starboard) newMsg(content string) *discordgo.MessageSend {
	return &discordgo.MessageSend{Content: helpers.GetText(content)}
}

func (s *Starboard) Relax(err error) {
	if err != nil {
		panic(err)
	}
}

func (s *Starboard) logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "starboard")
}

func (s *Starboard) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {

}

func (s *Starboard) OnMessageDelete(msg *discordgo.MessageDelete, session *discordgo.Session) {
	go func() {
		defer helpers.Recover()

		channel, err := helpers.GetChannel(msg.ChannelID)
		helpers.Relax(err)

		starboardEntry, err := s.getStarboardEntry(channel.GuildID, msg.ID)
		if err != nil {
			return
		}

		s.deleteStarboardEntry(starboardEntry)

		err = cache.GetSession().ChannelMessageDelete(
			starboardEntry.StarboardMessageChannelID, starboardEntry.StarboardMessageID)
		helpers.Relax(err)
	}()
}

func (s *Starboard) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {

}

func (s *Starboard) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {

}

func (s *Starboard) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {
	go func() {
		defer helpers.Recover()

		isStarboardEmoji := false
		for _, starboardEmoji := range StarboardEmojis {
			if reaction.MessageReaction.Emoji.Name == starboardEmoji {
				isStarboardEmoji = true
			}
		}

		// stop if no starboard emoji
		if isStarboardEmoji == false {
			return
		}

		user, err := helpers.GetUser(reaction.UserID)
		helpers.Relax(err)

		// stop if reaction is by a bot
		if user.Bot {
			return
		}

		channel, err := helpers.GetChannel(reaction.ChannelID)
		helpers.Relax(err)

		settings := helpers.GuildSettingsGetCached(channel.GuildID)

		// stop if no starboard channel set
		if settings.StarboardChannelID == "" {
			return
		}

		message, err := cache.GetSession().State.Message(reaction.ChannelID, reaction.MessageID)
		if err != nil {
			message, err = cache.GetSession().ChannelMessage(reaction.ChannelID, reaction.MessageID)
		}
		helpers.Relax(err)

		// stop if user is reacting to own message
		if message.Author.ID == reaction.UserID {
			return
		}

		// stop if no message and no attachment
		if message.Content == "" && len(message.Attachments) <= 0 {
			return
		}

		err = s.AddStar(channel.GuildID, message, reaction.UserID)
		helpers.Relax(err)
	}()
}

func (s *Starboard) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {
	go func() {
		defer helpers.Recover()

		isStarboardEmoji := false
		for _, starboardEmoji := range StarboardEmojis {
			if reaction.MessageReaction.Emoji.Name == starboardEmoji {
				isStarboardEmoji = true
			}
		}

		// stop if no starboard emoji
		if isStarboardEmoji == false {
			return
		}

		user, err := helpers.GetUser(reaction.UserID)
		helpers.Relax(err)

		// stop if reaction is by a bot
		if user.Bot {
			return
		}

		channel, err := helpers.GetChannel(reaction.ChannelID)
		helpers.Relax(err)

		settings := helpers.GuildSettingsGetCached(channel.GuildID)

		// stop if no starboard channel set
		if settings.StarboardChannelID == "" {
			return
		}

		message, err := cache.GetSession().State.Message(reaction.ChannelID, reaction.MessageID)
		if err != nil {
			message, err = cache.GetSession().ChannelMessage(reaction.ChannelID, reaction.MessageID)
		}
		helpers.Relax(err)

		// stop if user is reacting to own message
		if message.Author.ID == reaction.UserID {
			return
		}

		err = s.RemoveStar(channel.GuildID, message, reaction.UserID)
		helpers.Relax(err)
	}()
}

func (s *Starboard) AddStar(guildID string, msg *discordgo.Message, starUserID string) error {
	starboardEntry, err := s.getStarboardEntry(guildID, msg.ID)
	if err != nil {
		urls := make([]string, 0)
		for _, attachment := range msg.Attachments {
			urls = append(urls, attachment.URL)
		}
		embedImage := ""
		if len(msg.Embeds) > 0 {
			for _, embed := range msg.Embeds {
				if embed.Video != nil && embed.Video.URL != "" {
					embedImage = embed.Video.URL
				}
				if embed.Image != nil && embed.Image.URL != "" {
					embedImage = embed.Image.URL
				}
				if embed.Thumbnail != nil && embed.Thumbnail.URL != "" {
					embedImage = embed.Thumbnail.URL
				}
			}
		}

		if strings.Contains(err.Error(), "no starboard entry") {
			starboardEntry, err = s.createStarboardEntry(
				guildID,
				msg.ID,
				msg.ChannelID,
				msg.Author.ID,
				msg.Content,
				urls,
				embedImage,
			)
			helpers.Relax(err)
		} else {
			return err
		}
	}

	err = s.incrementStarboardEntry(&starboardEntry, starUserID)
	if err != nil {
		return err
	}

	return s.PostOrUpdateDiscordMessage(starboardEntry)
}

func (s *Starboard) RemoveStar(guildID string, msg *discordgo.Message, starUserID string) error {
	starboardEntry, err := s.getStarboardEntry(guildID, msg.ID)
	if err != nil {
		if strings.Contains(err.Error(), "no starboard entry") {
			return nil
		} else {
			return err
		}
	}

	deleted, err := s.decrementStarboardEntry(&starboardEntry, starUserID)
	helpers.Relax(err)

	if starboardEntry.StarboardMessageID != "" && starboardEntry.StarboardMessageChannelID != "" {
		if deleted {
			err = cache.GetSession().ChannelMessageDelete(
				starboardEntry.StarboardMessageChannelID, starboardEntry.StarboardMessageID)
			helpers.Relax(err)
		} else {
			return s.PostOrUpdateDiscordMessage(starboardEntry)
		}
	}
	return nil
}

func (s *Starboard) PostOrUpdateDiscordMessage(starEntry models.StarEntry) error {
	settings := helpers.GuildSettingsGetCached(starEntry.GuildID)
	if settings.StarboardChannelID == "" {
		return nil
	}

	authorName := "N/A"
	authorDP := ""
	author, err := helpers.GetGuildMember(starEntry.GuildID, starEntry.AuthorID)
	if err == nil && author != nil && author.User != nil {
		authorDP = author.User.AvatarURL("256")
		authorName = author.User.Username
		if author.Nick != "" {
			authorName += " ~ " + author.Nick
		}
	}

	channelName := ""
	channel, err := helpers.GetChannel(starEntry.ChannelID)
	if err == nil && channel != nil {
		channelName = channel.Name
	}

	content := starEntry.MessageContent
	for _, url := range starEntry.MessageAttachmentURLs {
		content += "\n" + url
	}

	starboardPostEmbed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("@%s in #%s:", authorName, channelName),
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("⭐ %s | Message #%s | First starred at %s",
				humanize.Comma(int64(starEntry.Stars)),
				starEntry.MessageID,
				starEntry.FirstStarred.Format(time.ANSIC),
			),
		},
		Color: helpers.GetDiscordColorFromHex("ffd700"),
	}
	if content != "" {
		starboardPostEmbed.Description = content
	}
	if starEntry.MessageEmbedImageURL != "" {
		starboardPostEmbed.Image = &discordgo.MessageEmbedImage{URL: starEntry.MessageEmbedImageURL}
	} else if len(starEntry.MessageAttachmentURLs) > 0 {
		starboardPostEmbed.Image = &discordgo.MessageEmbedImage{URL: starEntry.MessageAttachmentURLs[0]}
	} else {
		imageFileExtensions := []string{"jpg", "jpeg", "png", "gif"}
	TryContentUrls:
		for _, foundUrl := range xurls.Strict.FindAllString(starEntry.MessageContent, -1) {
			for _, fileExtension := range imageFileExtensions {
				if strings.HasSuffix(foundUrl, "."+fileExtension) {
					starboardPostEmbed.Image = &discordgo.MessageEmbedImage{URL: foundUrl}
					break TryContentUrls
				}
			}

		}
	}
	if authorDP != "" {
		starboardPostEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: authorDP}
	}
	if starEntry.StarboardMessageChannelID != "" &&
		starEntry.StarboardMessageID != "" &&
		starEntry.StarboardMessageChannelID == settings.StarboardChannelID {
		_, err := cache.GetSession().ChannelMessageEditEmbed(
			settings.StarboardChannelID, starEntry.StarboardMessageID, starboardPostEmbed)
		return err
	} else {
		starboardPostMessage, err := cache.GetSession().ChannelMessageSendEmbed(
			settings.StarboardChannelID, starboardPostEmbed)
		if err != nil {
			return err
		}
		starEntry.StarboardMessageID = starboardPostMessage.ID
		starEntry.StarboardMessageChannelID = starboardPostMessage.ChannelID
		return s.setStarboardEntry(starEntry)
	}
}

func (s *Starboard) getStarrersEmbed(starEntry models.StarEntry) *discordgo.MessageEmbed {
	authorName := "N/A"
	author, err := helpers.GetGuildMember(starEntry.GuildID, starEntry.AuthorID)
	if err == nil && author != nil && author.User != nil {
		authorName = author.User.Username
		if author.Nick != "" {
			authorName += " ~ " + author.Nick
		}
	}

	var starrersText string
	var userName string
	for i, starrerUserID := range starEntry.StarUserIDs {
		userName = "N/A"
		author, err := helpers.GetGuildMember(starEntry.GuildID, starrerUserID)
		if err == nil && author != nil && author.User != nil {
			userName = "@" + author.User.Username
			if author.Nick != "" {
				userName += " ~ " + author.Nick
			}
		}
		starrersText += userName
		if i+2 == len(starEntry.StarUserIDs) {
			starrersText += " and "
		} else {
			starrersText += ", "
		}
	}

	starrersText = strings.TrimRight(starrersText, ", ")

	starrersText += fmt.Sprintf(" (%s ⭐)", humanize.Comma(int64(starEntry.Stars)))

	if starrersText == "" {
		starrersText = "N/A"
	}

	channelName := ""
	channel, err := helpers.GetChannel(starEntry.ChannelID)
	if err == nil && channel != nil {
		channelName = channel.Name
	}

	content := starEntry.MessageContent
	for _, url := range starEntry.MessageAttachmentURLs {
		content += "\n" + url
	}
	if starEntry.MessageEmbedImageURL != "" {
		content += "\n" + starEntry.MessageEmbedImageURL
	}

	starrersEmbed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("Starrers of message #%s by @%s in #%s:", starEntry.MessageID, authorName, channelName),
		Description: starrersText,
		Color:       helpers.GetDiscordColorFromHex("ffd700"),
	}
	return starrersEmbed
}

func (s *Starboard) getTopMessagesEmbed(starEntries []models.StarEntry) (*discordgo.MessageEmbed, error) {
	if len(starEntries) <= 0 {
		return &discordgo.MessageEmbed{}, errors.New("no star entries passed")
	}

	guild, err := helpers.GetGuild(starEntries[0].GuildID)
	if err != nil {
		return &discordgo.MessageEmbed{}, err
	}

	var content string
	var authorName string
	topText := ""
	i := 1
	for _, starMessage := range starEntries {
		author, err := helpers.GetGuildMember(starMessage.GuildID, starMessage.AuthorID)
		authorName = "N/A"
		if err == nil && author != nil && author.User != nil {
			authorName = author.User.Username
			if author.Nick != "" {
				authorName += " ~ " + author.Nick
			}
		}

		content = starMessage.MessageContent
		if len(content) > 100 {
			content = content[0:99] + " ..."
		}
		if len(starMessage.MessageAttachmentURLs) > 0 {
			if content == "" {
				content = starMessage.MessageAttachmentURLs[0]
				if len(starMessage.MessageAttachmentURLs) > 1 {
					content += " ..."
				}
			} else if !strings.HasSuffix(content, "...") {
				content += " ..."
			}
		}
		if starMessage.MessageEmbedImageURL != "" {
			if content == "" {
				content = starMessage.MessageEmbedImageURL
			} else if !strings.HasSuffix(content, "...") {
				content += " ..."
			}
		}

		topText += fmt.Sprintf("#%d by %s (%s ⭐): %s\n",
			i, authorName, humanize.Comma(int64(starMessage.Stars)), content)
		i++
	}

	starrersEmbed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("Top starred messages on %s:", guild.Name),
		Description: topText,
		Color:       helpers.GetDiscordColorFromHex("ffd700"),
	}
	return starrersEmbed, nil
}

func (s *Starboard) getStarboardEntry(guildID string, messageID string) (models.StarEntry, error) {
	var entryBucket models.StarEntry
	listCursor, err := rethink.Table("starboard_entries").GetAllByIndex(
		"message_id", messageID,
	).Filter(
		rethink.Row.Field("guild_id").Eq(guildID),
	).Run(helpers.GetDB())
	if err != nil {
		return entryBucket, err
	}
	defer listCursor.Close()
	err = listCursor.One(&entryBucket)

	if err == rethink.ErrEmptyResult {
		return entryBucket, errors.New("no starboard entry")
	} else if err != nil {
		return entryBucket, err
	}

	return entryBucket, nil
}

func (s *Starboard) getTopStarboardEntries(guildID string) ([]models.StarEntry, error) {
	var entryBucket []models.StarEntry
	listCursor, err := rethink.Table("starboard_entries").Filter(
		rethink.Row.Field("guild_id").Eq(guildID),
	).OrderBy(rethink.Desc("stars")).Limit(10).Run(helpers.GetDB())
	if err != nil {
		return entryBucket, err
	}

	defer listCursor.Close()
	err = listCursor.All(&entryBucket)
	if err == rethink.ErrEmptyResult {
		return entryBucket, errors.New("no starboard entries")
	} else if err != nil {
		return entryBucket, err
	}

	return entryBucket, nil
}

func (s *Starboard) incrementStarboardEntry(starEntry *models.StarEntry, userID string) error {
	alreadyInList := false
	for _, starUserID := range starEntry.StarUserIDs {
		if starUserID == userID {
			alreadyInList = true
		}
	}
	if alreadyInList == false {
		starEntry.StarUserIDs = append(starEntry.StarUserIDs, userID)
		starEntry.Stars = len(starEntry.StarUserIDs)
	}
	return s.setStarboardEntry(*starEntry)
}
func (s *Starboard) decrementStarboardEntry(starEntry *models.StarEntry, userID string) (bool, error) {
	newStarUserIDs := make([]string, 0)
	for _, starUserID := range starEntry.StarUserIDs {
		if starUserID != userID {
			newStarUserIDs = append(newStarUserIDs, starUserID)
		}
	}
	starEntry.StarUserIDs = newStarUserIDs
	starEntry.Stars = len(starEntry.StarUserIDs)

	if len(starEntry.StarUserIDs) <= 0 {
		return true, s.deleteStarboardEntry(*starEntry)
	}
	return false, s.setStarboardEntry(*starEntry)
}

func (s *Starboard) createStarboardEntry(
	guildID string,
	messageID string,
	channelID string,
	authorID string,
	messageContent string,
	messageAttachmentURLs []string,
	messageEmbedImageURL string,
) (models.StarEntry, error) {
	insert := rethink.Table("starboard_entries").Insert(models.StarEntry{
		GuildID:               guildID,
		MessageID:             messageID,
		ChannelID:             channelID,
		AuthorID:              authorID,
		MessageContent:        messageContent,
		MessageAttachmentURLs: messageAttachmentURLs,
		MessageEmbedImageURL:  messageEmbedImageURL,
		StarUserIDs:           []string{},
		Stars:                 0,
		FirstStarred:          time.Now(),
	})
	_, err := insert.RunWrite(helpers.GetDB())
	if err != nil {
		return models.StarEntry{}, err
	} else {
		return s.getStarboardEntry(guildID, messageID)
	}
}

func (s *Starboard) setStarboardEntry(starEntry models.StarEntry) error {
	if starEntry.ID != "" {
		_, err := rethink.Table("starboard_entries").Get(starEntry.ID).Update(starEntry).RunWrite(helpers.GetDB())
		return err
	}
	return errors.New("empty starEntry submitted")
}

func (s *Starboard) deleteStarboardEntry(starEntry models.StarEntry) error {
	if starEntry.ID != "" {

		_, err := rethink.Table("starboard_entries").Get(starEntry.ID).Delete().RunWrite(helpers.GetDB())
		return err
	}
	return errors.New("empty starEntry submitted")
}

func (s *Starboard) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {

}

func (s *Starboard) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}
