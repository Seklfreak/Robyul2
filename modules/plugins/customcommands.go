package plugins

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"bytes"

	"github.com/Jeffail/gabs"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/getsentry/raven-go"
	rethink "github.com/gorethink/gorethink"
	"github.com/kennygrant/sanitize"
)

type CustomCommands struct{}

type DB_CustomCommands_Command struct {
	ID              string    `gorethink:"id,omitempty"`
	GuildID         string    `gorethink:"guildid"`
	CreatedByUserID string    `gorethink:"createdby_userid"`
	CreatedAt       time.Time `gorethink:"createdat"`
	Triggered       int       `gorethink:"triggered"`
	Keyword         string    `gorethink:"keyword"`
	Content         string    `gorethink:"content"`
}

func (cc *CustomCommands) Commands() []string {
	return []string{
		"customcommands",
		"customcom",
		"commands",
		"command",
	}
}

var (
	customCommandsCache []DB_CustomCommands_Command
)

func (cc *CustomCommands) Init(session *discordgo.Session) {
	customCommandsCache = cc.getAllCustomCommands()
}

func (cc *CustomCommands) Uninit(session *discordgo.Session) {

}

func (cc *CustomCommands) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	args := strings.Fields(content)
	if len(args) >= 1 {
		switch args[0] {
		case "add": // [p]commands add <command name> <command text>
			helpers.RequireMod(msg, func() {
				session.ChannelTyping(msg.ChannelID)
				if len(args) < 3 {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					helpers.Relax(err)
					return
				}
				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)

				if helpers.CommandExists(args[1]) {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.add-command-already-exists"))
					helpers.Relax(err)
					return
				}

				var entryBucket DB_CustomCommands_Command
				listCursor, err := rethink.Table("customcommands").Filter(
					rethink.Row.Field("guildid").Eq(channel.GuildID),
				).Filter(
					rethink.Row.Field("keyword").Eq(args[1]),
				).Run(helpers.GetDB())
				helpers.Relax(err)
				defer listCursor.Close()
				err = listCursor.One(&entryBucket)
				if err != rethink.ErrEmptyResult || entryBucket.ID != "" {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.add-keyword-already-exists"))
					helpers.Relax(err)
					return
				}

				newCommand := cc.getEntryByOrCreateEmpty("id", "")
				newCommand.GuildID = channel.GuildID
				newCommand.CreatedByUserID = msg.Author.ID
				newCommand.CreatedAt = time.Now().UTC()
				newCommand.Triggered = 0
				newCommand.Keyword = args[1]
				newCommand.Content = strings.TrimSpace(strings.Replace(content, strings.Join(args[:2], " "), "", 1))
				cc.setEntry(newCommand)

				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.add-success"))
				helpers.Relax(err)
				customCommandsCache = cc.getAllCustomCommands()
			})
			return
		case "list": // [p]commands list [top]
			session.ChannelTyping(msg.ChannelID)
			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)
			guild, err := helpers.GetGuild(channel.GuildID)
			helpers.Relax(err)

			var topCommands bool
			if len(args) >= 2 && strings.ToLower(args[1]) == "top" {
				topCommands = true
			}

			var entryBucket []DB_CustomCommands_Command
			var listCursor *rethink.Cursor
			if topCommands {
				listCursor, err = rethink.Table("customcommands").Filter(
					rethink.Row.Field("guildid").Eq(channel.GuildID),
				).OrderBy(rethink.Desc("triggered")).Run(helpers.GetDB())
			} else {
				listCursor, err = rethink.Table("customcommands").Filter(
					rethink.Row.Field("guildid").Eq(channel.GuildID),
				).OrderBy(rethink.Asc("keyword")).Run(helpers.GetDB())
			}
			helpers.Relax(err)
			defer listCursor.Close()
			err = listCursor.All(&entryBucket)
			if err == rethink.ErrEmptyResult || len(entryBucket) <= 0 {
				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.list-empty"))
				helpers.Relax(err)
				return
			} else if err != nil {
				helpers.Relax(err)
			}

			dmChannel, err := session.UserChannelCreate(msg.Author.ID)
			helpers.Relax(err)

			commandListText := fmt.Sprintf("Custom commands on `%s`:\n", guild.Name)
			for _, customCommand := range entryBucket {
				commandListText += fmt.Sprintf("`%s%s` (used %s times)\n",
					helpers.GetPrefixForServer(channel.GuildID), customCommand.Keyword, humanize.Comma(int64(customCommand.Triggered)))
			}
			commandListText += fmt.Sprintf("There are **%s** custom commands on this server.", humanize.Comma(int64(len(entryBucket))))

			helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.check-your-dms", msg.Author.ID))

			for _, page := range helpers.Pagify(commandListText, "\n") {
				_, err = helpers.SendMessage(dmChannel.ID, page)
				helpers.Relax(err)
			}
			return
		case "delete", "del", "remove": // [p]commands delete <command name>
			helpers.RequireMod(msg, func() {
				session.ChannelTyping(msg.ChannelID)
				if len(args) < 2 {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					helpers.Relax(err)
					return
				}
				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)

				var entryBucket DB_CustomCommands_Command
				listCursor, err := rethink.Table("customcommands").Filter(
					rethink.Row.Field("guildid").Eq(channel.GuildID),
				).Filter(
					rethink.Row.Field("keyword").Eq(args[1]),
				).Run(helpers.GetDB())
				helpers.Relax(err)
				defer listCursor.Close()
				err = listCursor.One(&entryBucket)
				if err == rethink.ErrEmptyResult || entryBucket.ID == "" {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.delete-not-found"))
					helpers.Relax(err)
					return
				} else if err != nil {
					helpers.Relax(err)
				}

				cc.deleteEntryById(entryBucket.ID)
				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.delete-success"))
				helpers.Relax(err)
				customCommandsCache = cc.getAllCustomCommands()
			})
			return
		case "replace", "edit": // [p]commands edit <command name> <new content>
			helpers.RequireMod(msg, func() {
				session.ChannelTyping(msg.ChannelID)
				if len(args) < 3 {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					helpers.Relax(err)
					return
				}
				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)

				var entryBucket DB_CustomCommands_Command
				listCursor, err := rethink.Table("customcommands").Filter(
					rethink.Row.Field("guildid").Eq(channel.GuildID),
				).Filter(
					rethink.Row.Field("keyword").Eq(args[1]),
				).Run(helpers.GetDB())
				helpers.Relax(err)
				defer listCursor.Close()
				err = listCursor.One(&entryBucket)

				if err == rethink.ErrEmptyResult || entryBucket.ID == "" {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.edit-not-found"))
					return
				} else if err != nil {
					helpers.Relax(err)
				}

				entryBucket.CreatedByUserID = msg.Author.ID
				entryBucket.CreatedAt = time.Now().UTC()
				entryBucket.Triggered = 0
				entryBucket.Content = strings.TrimSpace(strings.Replace(content, strings.Join(args[:2], " "), "", 1))
				cc.setEntry(entryBucket)

				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.edit-success"))
				helpers.Relax(err)
				customCommandsCache = cc.getAllCustomCommands()
			})
			return
		case "refresh": // [p]commands refresh
			helpers.RequireBotAdmin(msg, func() {
				session.ChannelTyping(msg.ChannelID)
				customCommandsCache = cc.getAllCustomCommands()
				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.refreshed-commands"))
				helpers.Relax(err)
			})
			return
		case "search": // [p]commands search <text>
			session.ChannelTyping(msg.ChannelID)
			if len(args) < 2 {
				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
				helpers.Relax(err)
				return
			}
			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)

			var entryBucket []DB_CustomCommands_Command
			listCursor, err := rethink.Table("customcommands").Filter(
				rethink.Row.Field("guildid").Eq(channel.GuildID),
			).Filter(
				rethink.Row.Field("keyword").Match(fmt.Sprintf("(?i)%s", args[1])),
			).Run(helpers.GetDB())
			if err != nil {
				if errR, ok := err.(rethink.RQLQueryLogicError); ok {
					if strings.Contains(errR.String(), "Error in regexp") {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
						return
					}
				}
			}
			helpers.Relax(err)
			defer listCursor.Close()
			err = listCursor.All(&entryBucket)
			if err == rethink.ErrEmptyResult || len(entryBucket) <= 0 {
				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.customcommands.search-empty", args[1]))
				helpers.Relax(err)
				return
			} else if err != nil {
				helpers.Relax(err)
			}

			commandListText := fmt.Sprintf("Custom commands including `%s` on this server:\n", args[1])
			for _, customCommand := range entryBucket {
				commandListText += fmt.Sprintf("`%s%s` (used %s times)\n",
					helpers.GetPrefixForServer(channel.GuildID), customCommand.Keyword, humanize.Comma(int64(customCommand.Triggered)))
			}
			if len(entryBucket) > 1 {
				commandListText += fmt.Sprintf("I found **%s** commands.", humanize.Comma(int64(len(entryBucket))))
			} else {
				commandListText += "I found **1** command."
			}

			for _, page := range helpers.Pagify(commandListText, "\n") {
				_, err = helpers.SendMessage(msg.ChannelID, page)
				helpers.Relax(err)
			}
			return
		case "info": // [p]commands info <command name>
			session.ChannelTyping(msg.ChannelID)
			if len(args) < 2 {
				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
				helpers.Relax(err)
				return
			}

			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)

			var entryBucket DB_CustomCommands_Command
			listCursor, err := rethink.Table("customcommands").Filter(
				rethink.Row.Field("guildid").Eq(channel.GuildID),
			).Filter(
				rethink.Row.Field("keyword").Eq(args[1]),
			).Run(helpers.GetDB())
			helpers.Relax(err)
			defer listCursor.Close()
			err = listCursor.One(&entryBucket)
			if err == rethink.ErrEmptyResult || entryBucket.ID == "" {
				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.info-not-found"))
				helpers.Relax(err)
				return
			} else if err != nil {
				helpers.Relax(err)
			}

			author, err := helpers.GetUser(entryBucket.CreatedByUserID)
			helpers.Relax(err)

			infoEmbed := &discordgo.MessageEmbed{
				Title:       fmt.Sprintf("Custom Command: `%s%s`", helpers.GetPrefixForServer(channel.GuildID), entryBucket.Keyword),
				Description: entryBucket.Content,
				Fields: []*discordgo.MessageEmbedField{
					{Name: "Author", Value: fmt.Sprintf("%s#%s", author.Username, author.Discriminator)},
					{Name: "Times triggered", Value: humanize.Comma(int64(entryBucket.Triggered))},
					{Name: "Created At", Value: fmt.Sprintf("%s UTC", entryBucket.CreatedAt.Format(time.ANSIC))},
				},
			}

			_, err = helpers.SendEmbed(msg.ChannelID, infoEmbed)
			helpers.Relax(err)
			return
		case "import-json": // [p]command import-json (with json file attached)
			helpers.RequireMod(msg, func() {
				session.ChannelTyping(msg.ChannelID)

				if len(msg.Attachments) <= 0 {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
					helpers.Relax(err)
					return
				}

				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)

				defer func() {
					err := recover()

					if err != nil {
						if err, ok := err.(*json.SyntaxError); ok {
							_, errNew := helpers.SendMessage(msg.ChannelID, fmt.Sprintf("JSON Error: `%s` (Offset %d)", err.Error(), err.Offset))
							helpers.Relax(errNew)
							return
						}
					}

					panic(err)
				}()

				commandsContainerJson := helpers.GetJSON(msg.Attachments[0].URL)
				commandsContainer, err := commandsContainerJson.ChildrenMap()
				helpers.Relax(err)

				var entryBucket []DB_CustomCommands_Command
				listCursor, err := rethink.Table("customcommands").Filter(
					rethink.Row.Field("guildid").Eq(channel.GuildID),
				).OrderBy(rethink.Asc("keyword")).Run(helpers.GetDB())
				helpers.Relax(err)
				defer listCursor.Close()
				err = listCursor.All(&entryBucket)
				if err != nil && err != rethink.ErrEmptyResult {
					helpers.Relax(err)
				}

				i := 0
				for newCustomCommandName, newCustomCommandContent := range commandsContainer {
					commandExists := false
					for _, customCommand := range entryBucket {
						if customCommand.Keyword == newCustomCommandName {
							commandExists = true
						}
					}
					if commandExists {
						helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Command with the name `%s` already exists.", newCustomCommandName))
						continue
					}

					newCustomCommandContentText := strings.TrimPrefix(strings.TrimSuffix(newCustomCommandContent.String(), "\""), "\"")
					newCommand := cc.getEntryByOrCreateEmpty("id", "")
					newCommand.GuildID = channel.GuildID
					newCommand.CreatedByUserID = msg.Author.ID
					newCommand.CreatedAt = time.Now().UTC()
					newCommand.Triggered = 0
					newCommand.Keyword = newCustomCommandName
					newCommand.Content = newCustomCommandContentText
					cc.setEntry(newCommand)
					helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Imported custom command `%s`", newCustomCommandName))
					i++
				}

				_, err = helpers.SendMessage(msg.ChannelID, fmt.Sprintf("<@%s> I imported **%s** custom commnands.", msg.Author.ID, humanize.Comma(int64(i))))
				helpers.Relax(err)
				customCommandsCache = cc.getAllCustomCommands()
			})
			return
		case "export-json": // [p]export-json
			session.ChannelTyping(msg.ChannelID)
			helpers.RequireMod(msg, func() {
				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)
				guild, err := helpers.GetGuild(channel.GuildID)
				helpers.Relax(err)

				var entryBucket []DB_CustomCommands_Command
				listCursor, err := rethink.Table("customcommands").Filter(
					rethink.Row.Field("guildid").Eq(channel.GuildID),
				).OrderBy(rethink.Asc("keyword")).Run(helpers.GetDB())
				helpers.Relax(err)
				defer listCursor.Close()
				err = listCursor.All(&entryBucket)
				if err == rethink.ErrEmptyResult || len(entryBucket) <= 0 {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.list-empty"))
					helpers.Relax(err)
					return
				} else if err != nil {
					helpers.Relax(err)
				}

				jsonObj := gabs.New()
				for _, command := range entryBucket {
					jsonObj.Set(command.Content, command.Keyword)
				}
				jsonObj.StringIndent("", "  ")

				_, err = session.ChannelFileSend(msg.ChannelID, sanitize.Path(guild.Name)+"-robyul-custom-commands.json",
					bytes.NewReader([]byte(jsonObj.StringIndent("", "  "))),
				)
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)

				return
			})
			return
		}
	}
}

func (cc *CustomCommands) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {
	channel, err := helpers.GetChannel(msg.ChannelID)
	if err != nil {
		go raven.CaptureError(err, map[string]string{})
		return
	}
	prefix := helpers.GetPrefixForServer(channel.GuildID)

	for i, customCommand := range customCommandsCache {
		if customCommand.GuildID == channel.GuildID && prefix+customCommand.Keyword == content {
			_, err := helpers.SendMessage(msg.ChannelID, customCommand.Content)
			if err != nil {
				go raven.CaptureError(err, map[string]string{})
				return
			}
			customCommandsCache[i].Triggered += 1
			cc.setEntry(customCommandsCache[i])
			metrics.CustomCommandsTriggered.Add(1)
			return
		}
	}
}

func (cc *CustomCommands) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {
}

func (cc *CustomCommands) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {
}

func (cc *CustomCommands) getEntryBy(key string, id string) DB_CustomCommands_Command {
	var entryBucket DB_CustomCommands_Command
	listCursor, err := rethink.Table("customcommands").Filter(
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

func (cc *CustomCommands) getEntryByOrCreateEmpty(key string, id string) DB_CustomCommands_Command {
	var entryBucket DB_CustomCommands_Command
	listCursor, err := rethink.Table("customcommands").Filter(
		rethink.Row.Field(key).Eq(id),
	).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
	defer listCursor.Close()
	err = listCursor.One(&entryBucket)

	// If user has no DB entries create an empty document
	if err == rethink.ErrEmptyResult {
		insert := rethink.Table("customcommands").Insert(DB_CustomCommands_Command{})
		res, e := insert.RunWrite(helpers.GetDB())
		// If the creation was successful read the document
		if e != nil {
			panic(e)
		} else {
			return cc.getEntryByOrCreateEmpty("id", res.GeneratedKeys[0])
		}
	} else if err != nil {
		panic(err)
	}

	return entryBucket
}

func (cc *CustomCommands) setEntry(entry DB_CustomCommands_Command) {
	_, err := rethink.Table("customcommands").Update(entry).Run(helpers.GetDB())
	helpers.Relax(err)
}

func (cc *CustomCommands) deleteEntryById(id string) {
	_, err := rethink.Table("customcommands").Filter(
		rethink.Row.Field("id").Eq(id),
	).Delete().RunWrite(helpers.GetDB())
	helpers.Relax(err)
}

func (cc *CustomCommands) getAllCustomCommands() []DB_CustomCommands_Command {
	var entryBucket []DB_CustomCommands_Command
	listCursor, err := rethink.Table("customcommands").Run(helpers.GetDB())
	helpers.Relax(err)
	defer listCursor.Close()
	err = listCursor.All(&entryBucket)

	if err != nil && err != rethink.ErrEmptyResult {
		helpers.Relax(err)
	}

	metrics.CustomCommandsCount.Set(int64(len(entryBucket)))
	return entryBucket
}

func (cc *CustomCommands) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {

}
func (cc *CustomCommands) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {

}
func (cc *CustomCommands) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {

}
func (cc *CustomCommands) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}
func (cc *CustomCommands) OnMessageDelete(msg *discordgo.MessageDelete, session *discordgo.Session) {

}
