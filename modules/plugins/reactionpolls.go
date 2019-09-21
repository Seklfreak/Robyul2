package plugins

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"sync"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Seklfreak/Robyul2/shardmanager"
	"github.com/bwmarrin/discordgo"
	humanize "github.com/dustin/go-humanize"
	"github.com/globalsign/mgo/bson"
)

type ReactionPolls struct{}

func (rp *ReactionPolls) Commands() []string {
	return []string{
		"reactionpolls",
		"reactionpoll",
	}
}

type ReactionPollCacheEntry struct {
	ID        bson.ObjectId
	MessageID string
}

var (
	reactionPollIDsCache    []ReactionPollCacheEntry
	reactionPollsEntryLocks = make(map[string]*sync.Mutex)
)

// @TODO: add metrics
func (rp *ReactionPolls) Init(session *shardmanager.Manager) {
	var err error
	reactionPollIDsCache, err = rp.getAllActiveReactionPollIDs()
	helpers.Relax(err)
}

func (rp *ReactionPolls) Uninit(session *shardmanager.Manager) {

}

func (rp *ReactionPolls) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermReactionPolls) {
		return
	}

	args, err := helpers.ToArgv(content)
	helpers.Relax(err)

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

		newEntry := models.ReactionpollsEntry{
			Text:            pollText,
			MessageID:       pollPostedMessage.ID,
			ChannelID:       channel.ID,
			GuildID:         guild.ID,
			CreatedByUserID: msg.Author.ID,
			CreatedAt:       time.Now().UTC(),
			Active:          true,
			AllowedEmotes:   allowedEmotes,
			MaxAllowedVotes: pollMaxVotes,
			Reactions:       nil,
			Initialised:     true,
		}

		_, err = helpers.MDbInsert(
			models.ReactionpollsTable,
			newEntry,
		)
		helpers.Relax(err)

		for _, allowedEmote := range allowedEmotes {
			err = session.MessageReactionAdd(pollPostedMessage.ChannelID, pollPostedMessage.ID, allowedEmote)
			helpers.Relax(err)
		}

		reactionPollIDsCache, err = rp.getAllActiveReactionPollIDs()
		helpers.Relax(err)

		pollEmbed = rp.getEmbedForPoll(newEntry, 0)
		_, err = helpers.EditEmbed(pollPostedMessage.ChannelID, pollPostedMessage.ID, pollEmbed)
		helpers.Relax(err)
		return
	case "refresh": // [p]reactionpolls refresh
		helpers.RequireBotAdmin(msg, func() {
			session.ChannelTyping(msg.ChannelID)
			var err error
			reactionPollIDsCache, err = rp.getAllActiveReactionPollIDs()
			helpers.Relax(err)
			_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.reactionpolls.refreshed-polls"))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
		})
		return
	}

}

func (rp *ReactionPolls) getEmbedForPoll(poll models.ReactionpollsEntry, totalVotes int) *discordgo.MessageEmbed {
	pollAuthor, err := helpers.GetUser(poll.CreatedByUserID)
	helpers.Relax(err)
	pollEmbed := &discordgo.MessageEmbed{
		Color:       0x0FADED,
		Description: poll.Text,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf(
				"Created By %s | Total Votes %s | Poll #%s",
				pollAuthor.Username, humanize.Comma(int64(totalVotes)), helpers.MdbIdToHuman(poll.ID)),
			IconURL: pollAuthor.AvatarURL("64"),
		},
	}
	return pollEmbed
}

func (rp *ReactionPolls) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {
	// skip reactions by the bot
	if reaction.UserID == session.State.User.ID {
		return
	}
	for _, reactionPollIDs := range reactionPollIDsCache {
		if reactionPollIDs.MessageID != reaction.MessageID {
			continue
		}

		rp.lockEntry(reactionPollIDs.ID)
		defer rp.unlockEntry(reactionPollIDs.ID)
		var reactionPoll models.ReactionpollsEntry
		err := helpers.MdbOneWithoutLogging(
			helpers.MdbCollection(models.ReactionpollsTable).Find(bson.M{"_id": reactionPollIDs.ID}),
			&reactionPoll,
		)
		helpers.Relax(err)

		// check if emote is allowed
		isAllowed := false
		for _, allowedEmote := range reactionPoll.AllowedEmotes {
			if allowedEmote == reaction.Emoji.APIName() {
				isAllowed = true
				break
			}
		}
		// remove emote if not allowed
		if !isAllowed {
			session.MessageReactionRemove(reaction.ChannelID, reaction.MessageID, reaction.Emoji.APIName(), reaction.UserID)
			return
		}
		// count total votes
		message, err := session.State.Message(reaction.ChannelID, reaction.MessageID)
		if err != nil {
			cache.GetLogger().WithField("module", "reactionpolls").Info(fmt.Sprintf("adding message #%s to world state", reaction.MessageID))
			message, err = session.ChannelMessage(reaction.ChannelID, reaction.MessageID)
			if err == nil {
				err = session.State.MessageAdd(message)
				helpers.Relax(err)
			}
		}
		if message.Author.ID != session.State.User.ID {
			return
		}
		// update entry
		if reactionPoll.Reactions[reaction.Emoji.APIName()] == nil {
			reactionPoll.Reactions[reaction.Emoji.APIName()] = make([]string, 0)
		}
		reactionPoll.Reactions[reaction.Emoji.APIName()] = append(reactionPoll.Reactions[reaction.Emoji.APIName()], reaction.UserID)
		err = helpers.MDbUpdateWithoutLogging(models.ReactionpollsTable, reactionPoll.ID, reactionPoll)
		helpers.Relax(err)
		// check if user is allowed to add another vote
		if reactionPoll.MaxAllowedVotes > -1 {
			if rp.getTotalVotes(reactionPoll, reaction.UserID) > reactionPoll.MaxAllowedVotes {
				session.MessageReactionRemove(reaction.ChannelID, reaction.MessageID, reaction.Emoji.APIName(), reaction.UserID)
				return
			}
		}
		// update embed
		pollEmbed := rp.getEmbedForPoll(reactionPoll, rp.getTotalVotes(reactionPoll, ""))
		_, err = helpers.EditEmbed(reactionPoll.ChannelID, reactionPoll.MessageID, pollEmbed)
		helpers.RelaxLog(err)
		return
	}
}

func (rp *ReactionPolls) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {
	// skip reactions by the bot
	if reaction.UserID == session.State.User.ID {
		return
	}
	for _, reactionPollIDs := range reactionPollIDsCache {
		if reactionPollIDs.MessageID != reaction.MessageID {
			continue
		}

		rp.lockEntry(reactionPollIDs.ID)
		defer rp.unlockEntry(reactionPollIDs.ID)
		var reactionPoll models.ReactionpollsEntry
		err := helpers.MdbOneWithoutLogging(
			helpers.MdbCollection(models.ReactionpollsTable).Find(bson.M{"_id": reactionPollIDs.ID}),
			&reactionPoll,
		)
		helpers.Relax(err)

		// check if emote is allowed
		isAllowed := false
		for _, allowedEmote := range reactionPoll.AllowedEmotes {
			if allowedEmote == reaction.Emoji.APIName() {
				isAllowed = true
				break
			}
		}
		// skip embed update if emote is not allowed
		if !isAllowed {
			return
		}
		// count total votes for the message
		message, err := session.State.Message(reaction.ChannelID, reaction.MessageID)
		if err != nil {
			cache.GetLogger().WithField("module", "reactionpolls").Info(fmt.Sprintf("adding message #%s to world state", reaction.MessageID))
			message, err = session.ChannelMessage(reaction.ChannelID, reaction.MessageID)
			if err == nil {
				err = session.State.MessageAdd(message)
				helpers.Relax(err)
			}
		}
		if message.Author.ID != session.State.User.ID {
			return
		}
		// update entry
		if reactionPoll.Reactions[reaction.Emoji.APIName()] != nil {
			without := make([]string, 0)
			for _, storedReactionUserID := range reactionPoll.Reactions[reaction.Emoji.APIName()] {
				if storedReactionUserID == reaction.UserID {
					continue
				}
				without = append(without, storedReactionUserID)
			}
			reactionPoll.Reactions[reaction.Emoji.APIName()] = without
		}
		err = helpers.MDbUpdateWithoutLogging(models.ReactionpollsTable, reactionPoll.ID, reactionPoll)
		helpers.Relax(err)
		// update embed
		pollEmbed := rp.getEmbedForPoll(reactionPoll, rp.getTotalVotes(reactionPoll, ""))
		_, err = helpers.EditEmbed(reactionPoll.ChannelID, reactionPoll.MessageID, pollEmbed)
		helpers.RelaxLog(err)
		return
	}
}

func (rp *ReactionPolls) getTotalVotes(reactionPoll models.ReactionpollsEntry, userID string) (count int) {
	if reactionPoll.Reactions == nil {
		reactionPoll.Reactions = make(map[string][]string, 0)
	}

	if !reactionPoll.Initialised {
		cache.GetLogger().WithField("module", "reactionpolls").Info(
			"initialising reaction poll #", helpers.MdbIdToHuman(reactionPoll.ID),
		)
		message, err := cache.GetSession().SessionForGuildS(reactionPoll.GuildID).ChannelMessage(reactionPoll.ChannelID, reactionPoll.MessageID)
		if err != nil {
			return 0
		}
		for _, messageReaction := range message.Reactions {
			for _, allowedEmote := range reactionPoll.AllowedEmotes {
				if allowedEmote != messageReaction.Emoji.APIName() {
					continue
				}
				messageReactionUsers, err := cache.GetSession().SessionForGuildS(reactionPoll.GuildID).MessageReactions(
					reactionPoll.ChannelID, reactionPoll.MessageID, messageReaction.Emoji.APIName(), 100)
				if err == nil {
					userIDs := make([]string, 0)
					for _, messageReactionUser := range messageReactionUsers {
						if messageReactionUser.ID == cache.GetSession().SessionForGuildS(reactionPoll.GuildID).State.User.ID {
							continue
						}

						userIDs = append(userIDs, messageReactionUser.ID)
					}
					reactionPoll.Reactions[messageReaction.Emoji.APIName()] = userIDs
				}
			}
		}
		reactionPoll.Initialised = true
		err = helpers.MDbUpdateWithoutLogging(models.ReactionpollsTable, reactionPoll.ID, reactionPoll)
		helpers.RelaxLog(err)
		if err != nil {
			return 0
		}
	}

NextReaction:
	for reactionEmoji, reactionUserIDs := range reactionPoll.Reactions {
	NextAllowedEmoji:
		for _, allowedEmote := range reactionPoll.AllowedEmotes {
			if allowedEmote != reactionEmoji {
				continue NextAllowedEmoji
			}
			if userID == "" {
				count += len(reactionUserIDs)
				continue NextReaction
			} else {
				for _, reactionUserID := range reactionUserIDs {
					if reactionUserID == userID {
						count++
						break
					}
				}
			}
		}
	}
	return
}

func (rp *ReactionPolls) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {

}

func (rp *ReactionPolls) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {

}

func (rp *ReactionPolls) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {

}

func (rp *ReactionPolls) getAllActiveReactionPollIDs() (ids []ReactionPollCacheEntry, err error) {
	var entryBucket []models.ReactionpollsEntry
	err = helpers.MDbIter(
		helpers.MdbCollection(models.ReactionpollsTable).
			Find(bson.M{"active": true}).
			Select(bson.M{"_id": 1, "messageid": 1}),
	).All(&entryBucket)
	if err != nil {
		return nil, err
	}
	ids = make([]ReactionPollCacheEntry, 0)
	if entryBucket != nil && len(entryBucket) >= 1 {
		for _, entry := range entryBucket {
			ids = append(ids, ReactionPollCacheEntry{
				ID:        entry.ID,
				MessageID: entry.MessageID,
			})
		}
	}
	return ids, nil
}

func (rp *ReactionPolls) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {

}
func (rp *ReactionPolls) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}
func (rp *ReactionPolls) OnMessageDelete(msg *discordgo.MessageDelete, session *discordgo.Session) {

}

func (rp *ReactionPolls) lockEntry(entryID bson.ObjectId) {
	if _, ok := reactionPollsEntryLocks[string(entryID)]; ok {
		reactionPollsEntryLocks[string(entryID)].Lock()
		return
	}
	reactionPollsEntryLocks[string(entryID)] = new(sync.Mutex)
	reactionPollsEntryLocks[string(entryID)].Lock()
}

func (rp *ReactionPolls) unlockEntry(entryID bson.ObjectId) {
	if _, ok := reactionPollsEntryLocks[string(entryID)]; ok {
		reactionPollsEntryLocks[string(entryID)].Unlock()
	}
}
