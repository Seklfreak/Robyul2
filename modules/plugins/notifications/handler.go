package notifications

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/globalsign/mgo/bson"
)

// _noti list
func handleList(session *discordgo.Session, msg *discordgo.Message) {
	guild, err := helpers.GetGuild(msg.GuildID)
	helpers.Relax(err)

	var entryBucket []models.NotificationsEntry
	err = helpers.MDbIter(helpers.MdbCollection(models.NotificationsTable).Find(bson.M{
		"userid":  msg.Author.ID,
		"guildid": bson.M{"$in": []string{msg.GuildID, "global"}},
	}).Sort("-triggered")).All(&entryBucket)
	helpers.Relax(err)

	if entryBucket == nil || len(entryBucket) <= 0 {
		helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.notifications.keyword-list-no-keywords-error", msg.Author.ID))
		return
	}

	resultMessage := fmt.Sprintf("Enabled keywords for the server: `%s`:\n", guild.Name)
	for _, entry := range entryBucket {
		resultMessage += fmt.Sprintf("`%s` (triggered `%d` times)", entry.Keyword, entry.Triggered)

		if len(entry.IgnoredGuildIDs) > 0 {
			resultMessage += " [Ignored in these Guild(s): "
			for _, ignoredGuildID := range entry.IgnoredGuildIDs {
				guild, err := helpers.GetGuildWithoutApi(ignoredGuildID)
				if err != nil {
					resultMessage += "N/A"
				} else {
					resultMessage += "`" + guild.Name + "`"
				}
				resultMessage += ", "
			}
			resultMessage = strings.TrimRight(resultMessage, ", ")
			resultMessage += "]"
		}

		if len(entry.IgnoredChannelIDs) > 0 {
			resultMessage += " [Ignored in these Channel(s): "
			for _, ignoredChannelID := range entry.IgnoredChannelIDs {
				resultMessage += "<#" + ignoredChannelID + ">, "
			}
			resultMessage = strings.TrimRight(resultMessage, ", ")
			resultMessage += "]"
		}

		if entry.GuildID == "global" {
			resultMessage += " [Global Keyword] :globe_with_meridians:"
		}

		resultMessage += "\n"
	}
	resultMessage += fmt.Sprintf("Found **%d** Keywords in total.", len(entryBucket))

	dmChannel, err := session.UserChannelCreate(msg.Author.ID)
	helpers.Relax(err)

	_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.check-your-dms", msg.Author.ID))
	helpers.RelaxMessage(err, msg.ChannelID, msg.ID)

	_, err = helpers.SendMessage(dmChannel.ID, resultMessage)
	helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
}

// _noti ignore <keyword(s)> [<#channel or channel id>]
func handleIgnore(session *discordgo.Session, content string, msg *discordgo.Message, args []string) {
	if len(args) < 2 {
		helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
		return
	}

	session.ChannelMessageDelete(msg.ChannelID, msg.ID)
	session.ChannelTyping(msg.ChannelID)

	var targetChannelID string
	targetChannel, err := helpers.GetChannelFromMention(msg, args[len(args)-1])
	if err == nil {
		targetChannelID = targetChannel.ID
	}

	// trim commands
	keywords := strings.TrimSpace(strings.Replace(content, args[0], "", 1))
	if targetChannelID != "" {
		// trim target channel ID
		keywords = strings.TrimSpace(
			keywords[0:strings.LastIndex(keywords, args[len(args)-1])],
		)
	}

	added, err := ignoreKeywordInGuildOrChannel(msg.Author.ID, keywords, msg.GuildID, targetChannelID)
	if err != nil {
		switch err {
		case KeywordsNotFoundError:
			helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.notifications.keyword-ignore-not-found-error"))
			return
		}
		helpers.Relax(err)
	}

	var message string
	if added {
		if targetChannelID == "" {
			message = helpers.GetText("plugins.notifications.keyword-ignore-guild-added")
		} else {
			message = helpers.GetTextF("plugins.notifications.keyword-ignore-channel-added", targetChannel.Mention())
		}
	} else {
		if targetChannelID == "" {
			message = helpers.GetText("plugins.notifications.keyword-ignore-guild-removed")
		} else {
			message = helpers.GetTextF("plugins.notifications.keyword-ignore-channel-removed", targetChannel.Mention())
		}
	}

	_, err = helpers.SendMessage(msg.ChannelID, message)
	helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
}

func ignoreKeywordInGuildOrChannel(userID, keywords, guildID, channelID string) (bool, error) {
	var added bool
	var entryBucket models.NotificationsEntry

	findArgs := bson.M{"userid": userID,
		"keyword": bson.M{"$regex": bson.RegEx{Pattern: "^" + regexp.QuoteMeta(keywords) + "$", Options: "i"}}}
	if channelID == "" {
		// ignore guild? look for global keywords
		findArgs["guildid"] = "global"
	} else {
		// ignore channels? look for global keywords, or keywords on this guild
		findArgs["$or"] = []bson.M{{"guildid": "global"}, {"guildid": guildID}}
	}

	err := helpers.MdbOne(
		helpers.MdbCollection(models.NotificationsTable).Find(findArgs),
		&entryBucket,
	)
	if err != nil {
		if helpers.IsMdbNotFound(err) {
			return added, KeywordsNotFoundError
		}
		return added, err
	}

	if channelID == "" {
		// ignore Guild
		ignoredGuildIDsWithout := make([]string, 0)
		for _, alreadyIgnoredGuildID := range entryBucket.IgnoredGuildIDs {
			if alreadyIgnoredGuildID != guildID {
				ignoredGuildIDsWithout = append(ignoredGuildIDsWithout, alreadyIgnoredGuildID)
			}
		}

		if len(ignoredGuildIDsWithout) != len(entryBucket.IgnoredGuildIDs) {
			entryBucket.IgnoredGuildIDs = ignoredGuildIDsWithout
			added = false
		} else {
			entryBucket.IgnoredGuildIDs = append(entryBucket.IgnoredGuildIDs, guildID)
			added = true
		}
	} else {
		// ignore Channel
		ignoredChannelIDsWithout := make([]string, 0)
		for _, alreadyIgnoredChannelID := range entryBucket.IgnoredChannelIDs {
			if alreadyIgnoredChannelID != channelID {
				ignoredChannelIDsWithout = append(ignoredChannelIDsWithout, alreadyIgnoredChannelID)
			}
		}

		if len(ignoredChannelIDsWithout) != len(entryBucket.IgnoredChannelIDs) {
			entryBucket.IgnoredChannelIDs = ignoredChannelIDsWithout
			added = false
		} else {
			entryBucket.IgnoredChannelIDs = append(entryBucket.IgnoredChannelIDs, channelID)
			added = true
		}
	}

	err = helpers.MDbUpdate(models.NotificationsTable, entryBucket.ID, entryBucket)
	if err != nil {
		return added, err
	}

	asyncRefresh()

	return added, nil
}
