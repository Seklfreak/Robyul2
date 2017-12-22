package plugins

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/getsentry/raven-go"
	rethink "github.com/gorethink/gorethink"
)

type ReactionPolls struct{}

func (rp *ReactionPolls) Commands() []string {
	return []string{
		"reactionpolls",
		"reactionpoll",
	}
}

type DB_ReactionPoll struct {
	ID              string    `gorethink:"id,omitempty"`
	Text            string    `gorethink:"text"`
	MessageID       string    `gorethink:"messageid"`
	ChannelID       string    `gorethink:"channelid"`
	GuildID         string    `gorethink:"guildid"`
	CreatedByUserID string    `gorethink:"createdby_userid"`
	CreatedAt       time.Time `gorethink:"createdat"`
	Active          bool      `gorethinK:"active"`
	AllowedEmotes   []string  `gorethink:"allowedemotes"`
	MaxAllowedVotes int       `gorethink:"maxallowedemotes"`
}

var (
	reactionPollsCache []DB_ReactionPoll
)

// @TODO: add metrics
func (rp *ReactionPolls) Init(session *discordgo.Session) {
	reactionPollsCache = rp.getAllReactionPolls()

	// delete deleted poll messages from the database
	go func() {
		for _, reactionPoll := range reactionPollsCache {
			_, err := session.ChannelMessage(reactionPoll.ChannelID, reactionPoll.MessageID)
			if err != nil {
				cache.GetLogger().WithField("module", "reactionpolls").Warn(fmt.Sprintf("Removed Reaction Poll #%s from the database since the message is not available anymore", reactionPoll.ID))
				rp.deleteReactionPollByID(reactionPoll.ID)
			}
		}
		reactionPollsCache = rp.getAllReactionPolls()
	}()
}

func (rp *ReactionPolls) Uninit(session *discordgo.Session) {

}

func (rp *ReactionPolls) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	lastQuote := rune(0)
	f := func(c rune) bool {
		switch {
		case c == lastQuote:
			lastQuote = rune(0)
			return false
		case lastQuote != rune(0):
			return false
		case unicode.In(c, unicode.Quotation_Mark):
			lastQuote = c
			return false
		default:
			return unicode.IsSpace(c)

		}
	}
	args := strings.FieldsFunc(content, f)
	if len(args) < 1 {
		return
	}
	switch args[0] {
	case "create": // [p]reactionpolls create "<poll text>" <max number of votes> <allowed emotes>
		session.ChannelTyping(msg.ChannelID)
		if len(args) < 4 {
			_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			return
		}
		pollText := strings.TrimSuffix(strings.TrimPrefix(args[1], "\""), "\"")
		if pollText == "" {
			_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			return
		}
		pollMaxVotes, err := strconv.Atoi(args[2])
		if err != nil {
			_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			return
		}
		channel, err := helpers.GetChannel(msg.ChannelID)
		helpers.Relax(err)
		guild, err := helpers.GetGuild(channel.GuildID)
		helpers.Relax(err)
		allowedEmotes := make([]string, 0)
		for _, allowedEmote := range args[3:] {
			allowedEmotes = append(allowedEmotes,
				strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(allowedEmote, "<a:"), "<:"), ">"),
			)
		}
		if len(allowedEmotes) > 20 {
			_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.reactionpolls.create-too-many-reactions"))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			return
		}
		for _, allowedEmote := range allowedEmotes {
			if len(allowedEmote) > 1 {
				emoteParts := strings.Split(allowedEmote, ":")
				if len(emoteParts) >= 2 {
					_, err = session.State.Emoji(guild.ID, emoteParts[1])
					if err != nil {
						_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.reactionpolls.create-external-emote"))
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					}
				}
			}
		}

		newReactionPoll := rp.getReactionPollByOrCreateEmpty("id", "")
		newReactionPoll.Text = pollText
		newReactionPoll.ChannelID = channel.ID
		newReactionPoll.GuildID = guild.ID
		newReactionPoll.CreatedByUserID = msg.Author.ID
		newReactionPoll.CreatedAt = time.Now().UTC()
		newReactionPoll.Active = true
		newReactionPoll.AllowedEmotes = allowedEmotes
		newReactionPoll.MaxAllowedVotes = pollMaxVotes
		rp.setReactionPoll(newReactionPoll)

		pollEmbed := &discordgo.MessageEmbed{
			Color:       0x0FADED,
			Description: "**Poll is being created...** :construction_site:",
		}
		pollPostedMessages, err := helpers.SendEmbed(msg.ChannelID, pollEmbed)
		helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)

		if len(pollPostedMessages) <= 0 {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.errors.generic-nomessage"))
			return
		}
		pollPostedMessage := pollPostedMessages[0]

		newReactionPoll.MessageID = pollPostedMessage.ID
		rp.setReactionPoll(newReactionPoll)

		for _, allowedEmote := range allowedEmotes {
			err = session.MessageReactionAdd(pollPostedMessage.ChannelID, pollPostedMessage.ID, allowedEmote)
			helpers.Relax(err)
		}

		reactionPollsCache = rp.getAllReactionPolls()

		pollEmbed = rp.getEmbedForPoll(newReactionPoll, 0)
		_, err = helpers.EditEmbed(pollPostedMessage.ChannelID, pollPostedMessage.ID, pollEmbed)
		helpers.Relax(err)
		return
	case "refresh": // [p]reactionpolls refresh
		helpers.RequireBotAdmin(msg, func() {
			session.ChannelTyping(msg.ChannelID)
			reactionPollsCache = rp.getAllReactionPolls()
			_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.reactionpolls.refreshed-polls"))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
		})
		return
	}

}

func (rp *ReactionPolls) getEmbedForPoll(poll DB_ReactionPoll, totalVotes int) *discordgo.MessageEmbed {
	pollAuthor, err := helpers.GetUser(poll.CreatedByUserID)
	helpers.Relax(err)
	pollEmbed := &discordgo.MessageEmbed{
		Color:       0x0FADED,
		Description: poll.Text,
		Footer: &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Created By %s | Total Votes %s | Poll ID %s",
			pollAuthor.Username, humanize.Comma(int64(totalVotes)), poll.ID,
		)},
	}
	return pollEmbed
}

func (rp *ReactionPolls) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {
	// skip reactions by the bot
	if reaction.UserID == session.State.User.ID {
		return
	}
	for _, reactionPoll := range reactionPollsCache {
		if reactionPoll.Active == true && reactionPoll.MessageID == reaction.MessageID {
			// check if emote is allowed
			isAllowed := false
			for _, allowedEmote := range reactionPoll.AllowedEmotes {
				if allowedEmote == reaction.Emoji.APIName() {
					isAllowed = true
					break
				}
			}
			// remove emote if not allowed
			if isAllowed == false {
				err := session.MessageReactionRemove(reaction.ChannelID, reaction.MessageID, reaction.Emoji.APIName(), reaction.UserID)
				if err != nil {
					if errD, ok := err.(*discordgo.RESTError); !ok || errD.Message.Code != 50013 {
						raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
					}
				}
				return
			}
			// get reactions on the message
			votesOnMessage := 0
			for _, reactionOnMessageEmote := range reactionPoll.AllowedEmotes {
				reactionOnMessageUsers, err := session.MessageReactions(reaction.ChannelID, reaction.MessageID, reactionOnMessageEmote, 100)
				if err != nil {
					raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
				}
				for _, reactionOnMessageUser := range reactionOnMessageUsers {
					if reactionOnMessageUser.ID == reaction.UserID {
						votesOnMessage++
					}
				}
			}
			// check if user is allowed to add another vote
			if reactionPoll.MaxAllowedVotes > -1 && votesOnMessage > reactionPoll.MaxAllowedVotes {
				err := session.MessageReactionRemove(reaction.ChannelID, reaction.MessageID, reaction.Emoji.APIName(), reaction.UserID)
				if err != nil {
					if err, ok := err.(*discordgo.RESTError); ok && err.Message != nil {
						if err.Message.Code == 50013 {
							cache.GetLogger().WithField("module", "reactionpolls").Warnf("can not remove reaction from message #%s, missing permissions", reaction.MessageID)
						} else {
							raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
						}
					} else {
						raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
					}
				}
				return
			}
			// count total votes
			message, err := session.State.Message(reaction.ChannelID, reaction.MessageID)
			if err != nil {
				cache.GetLogger().WithField("module", "reactionpolls").Info(fmt.Sprintf("adding message #%s to world state", reaction.MessageID))
				message, err = session.ChannelMessage(reaction.ChannelID, reaction.MessageID)
				if err != nil {
					if errD, ok := err.(*discordgo.RESTError); ok && (errD.Message.Code == discordgo.ErrCodeMissingAccess || errD.Message.Code == discordgo.ErrCodeUnknownMessage) {
						return
					}
				}
				helpers.Relax(err)
				err = session.State.MessageAdd(message)
				helpers.Relax(err)
			}
			if message.Author.ID == session.State.User.ID {
				totalVotes := 0
				for _, reaction := range message.Reactions {
					totalVotes += reaction.Count
				}
				totalVotes -= len(reactionPoll.AllowedEmotes)
				// update embed
				pollEmbed := rp.getEmbedForPoll(reactionPoll, totalVotes)
				_, err = helpers.EditEmbed(reactionPoll.ChannelID, reactionPoll.MessageID, pollEmbed)
				if err != nil {
					raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
				}
			}
			return
		}
	}
}

func (rp *ReactionPolls) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {
	// skip reactions by the bot
	if reaction.UserID == session.State.User.ID {
		return
	}
	for _, reactionPoll := range reactionPollsCache {
		if reactionPoll.Active == true && reactionPoll.MessageID == reaction.MessageID {
			// check if emote is allowed
			isAllowed := false
			for _, allowedEmote := range reactionPoll.AllowedEmotes {
				if allowedEmote == reaction.Emoji.APIName() {
					isAllowed = true
					break
				}
			}
			// skip embed update if emote is not allowed
			if isAllowed == false {
				return
			}
			// count total votes for the message
			message, err := session.ChannelMessage(reaction.ChannelID, reaction.MessageID)
			helpers.Relax(err)
			if message.Author.ID == session.State.User.ID {
				totalVotes := 0
				for _, reaction := range message.Reactions {
					totalVotes += reaction.Count
				}
				totalVotes -= len(reactionPoll.AllowedEmotes)
				// update embed
				pollEmbed := rp.getEmbedForPoll(reactionPoll, totalVotes)
				_, err = helpers.EditEmbed(reactionPoll.ChannelID, reactionPoll.MessageID, pollEmbed)
				if err != nil {
					raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
				}
			}
			return
		}
	}
}

func (rp *ReactionPolls) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {

}

func (rp *ReactionPolls) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {

}

func (rp *ReactionPolls) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {

}

func (rp *ReactionPolls) getReactionPollBy(key string, id string) DB_ReactionPoll {
	var entryBucket DB_ReactionPoll
	listCursor, err := rethink.Table("reactionpolls").Filter(
		rethink.Row.Field(key).Eq(id),
	).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer listCursor.Close()
	err = listCursor.One(&entryBucket)

	if err == rethink.ErrEmptyResult {
		return entryBucket
	} else if err != nil {
		panic(err)
	}

	return entryBucket
}

func (rp *ReactionPolls) getReactionPollByOrCreateEmpty(key string, id string) DB_ReactionPoll {
	var entryBucket DB_ReactionPoll
	listCursor, err := rethink.Table("reactionpolls").Filter(
		rethink.Row.Field(key).Eq(id),
	).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer listCursor.Close()
	err = listCursor.One(&entryBucket)

	if err == rethink.ErrEmptyResult {
		insert := rethink.Table("reactionpolls").Insert(DB_ReactionPoll{})
		res, e := insert.RunWrite(helpers.GetDB())
		if e != nil {
			panic(e)
		} else {
			return rp.getReactionPollBy("id", res.GeneratedKeys[0])
		}
	} else if err != nil {
		panic(err)
	}

	return entryBucket
}

func (rp *ReactionPolls) setReactionPoll(entry DB_ReactionPoll) {
	_, err := rethink.Table("reactionpolls").Update(entry).Run(helpers.GetDB())
	helpers.Relax(err)
}

func (rp *ReactionPolls) deleteReactionPollByID(id string) {
	_, err := rethink.Table("reactionpolls").Filter(
		rethink.Row.Field("id").Eq(id),
	).Delete().RunWrite(helpers.GetDB())
	helpers.Relax(err)
}

func (rp *ReactionPolls) getAllReactionPolls() []DB_ReactionPoll {
	var entryBucket []DB_ReactionPoll
	listCursor, err := rethink.Table("reactionpolls").Run(helpers.GetDB())
	helpers.Relax(err)
	defer listCursor.Close()
	err = listCursor.All(&entryBucket)

	if err != nil && err != rethink.ErrEmptyResult {
		helpers.Relax(err)
	}

	metrics.ReactionPollsCount.Set(int64(len(entryBucket)))
	return entryBucket
}

func (rp *ReactionPolls) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {

}
func (rp *ReactionPolls) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}
func (rp *ReactionPolls) OnMessageDelete(msg *discordgo.MessageDelete, session *discordgo.Session) {

}
