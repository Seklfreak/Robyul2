package plugins

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"bytes"

	"strconv"

	"mime"

	"github.com/Jeffail/gabs"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/globalsign/mgo/bson"
	"github.com/kennygrant/sanitize"
)

type CustomCommands struct{}

func (cc *CustomCommands) Commands() []string {
	return []string{
		"customcommands",
		"customcom",
		"commands",
		"command",
		"random",
	}
}

var (
	customCommandsCache            []models.CustomCommandsEntry
	customCommandsAllowedFiletypes = []string{"image/jpeg", "image/png", "image/gif", "video/mp4", "video/webm"}
)

func (cc *CustomCommands) Init(session *discordgo.Session) {
	var err error
	customCommandsCache, err = cc.getAllCustomCommands()
	helpers.Relax(err)
}

func (cc *CustomCommands) Uninit(session *discordgo.Session) {

}

func (cc *CustomCommands) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermCustomCommands) {
		return
	}

	// shortcut [p]random => [p]commands random
	if command == "random" {
		content = "random"
	}

	args := strings.Fields(content)
	if len(args) >= 1 {
		switch args[0] {
		case "toggle-permissions", "toggle-permission":
			helpers.RequireAdmin(msg, func() {
				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)

				guild, err := helpers.GetGuild(channel.GuildID)
				helpers.Relax(err)

				guildConfig := helpers.GuildSettingsGetCached(channel.GuildID)

				var targetRole *discordgo.Role
				var message string

				if len(args) >= 2 {
					// giving ability to add commands to a specific role?
					for _, guildRole := range guild.Roles {
						if strings.ToLower(guildRole.Name) == strings.ToLower(args[1]) ||
							guildRole.ID == strings.ToLower(args[1]) {
							targetRole = guildRole
						}
					}
				}

				if targetRole == nil {
					if guildConfig.CustomCommandsEveryoneCanAdd {
						guildConfig.CustomCommandsEveryoneCanAdd = false
						guildConfig.CustomCommandsAddRoleID = ""
						message = helpers.GetText("plugins.customcommands.disabled-everyone-canadd")
					} else {
						guildConfig.CustomCommandsEveryoneCanAdd = true
						guildConfig.CustomCommandsAddRoleID = ""
						message = helpers.GetText("plugins.customcommands.enabled-everyone-canadd")
					}
				} else {
					guildConfig.CustomCommandsEveryoneCanAdd = false
					guildConfig.CustomCommandsAddRoleID = targetRole.ID
					message = helpers.GetTextF("plugins.customcommands.role-canadd", targetRole.Name)
				}

				err = helpers.GuildSettingsSet(channel.GuildID, guildConfig)
				helpers.Relax(err)

				_, err = helpers.SendMessage(msg.ChannelID, message)
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			})
			return
		case "add": // [p]commands add <command name> <command text>
			// TODO: videos?
			session.ChannelTyping(msg.ChannelID)

			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)

			if !cc.canAddCommand(channel.GuildID, msg.Author.ID, nil) {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("mod.no_permission"))
				return
			}

			if len(args) < 3 && (len(msg.Attachments) <= 0 && len(args) < 2) {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
				return
			}

			if helpers.CommandExists(args[1]) {
				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.add-command-already-exists"))
				helpers.Relax(err)
				return
			}

			var entryBucket models.CustomCommandsEntry
			err = helpers.MdbOne(
				helpers.MdbCollection(models.CustomCommandsTable).Find(bson.M{"guildid": channel.GuildID, "keyword": args[1]}),
				&entryBucket,
			)
			if err == nil {
				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.add-keyword-already-exists"))
				helpers.Relax(err)
				return
			} else {
				if !strings.Contains(err.Error(), "not found") {
					helpers.Relax(err)
				}
			}

			var objectName, filetype, filename, hash string
			if len(msg.Attachments) > 0 {
				data, err := helpers.NetGetUAWithError(msg.Attachments[0].URL, helpers.DEFAULT_UA)
				helpers.Relax(err)

				filename = msg.Attachments[0].Filename

				filetype, err = helpers.SniffMime(data)
				helpers.Relax(err)

				// filetype allowed? (picture or video)
				if cc.isAllowedFiletype(filetype) {
					// user is allowed to upload files?
					if helpers.UseruploadsIsDisabled(msg.Author.ID) {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.errors.useruploads-disabled"))
						return
					}
					// <= 20 MB
					if msg.Attachments[0].Size > 20e+6 {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.fileupload-too-big"))
						return
					}
					// get object name
					objectName = models.CustomCommandsNewObjectName(channel.GuildID, msg.Author.ID)
					// upload to object storage
					err = helpers.UploadFile(objectName, data, map[string]string{
						"userid":    msg.Author.ID,
						"channelid": msg.ChannelID,
						"guildid":   channel.GuildID,
					})
					helpers.Relax(err)
				}
				hash = helpers.GetMD5Hash(objectName)
			}

			content := strings.TrimSpace(strings.Replace(content, strings.Join(args[:2], " "), "", 1))

			if content == "" && (filetype == "" || objectName == "") {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				return
			}

			_, err = helpers.MDbInsert(
				models.CustomCommandsTable,
				models.CustomCommandsEntry{
					GuildID:           channel.GuildID,
					CreatedByUserID:   msg.Author.ID,
					CreatedAt:         time.Now(),
					Triggered:         0,
					Keyword:           args[1],
					StorageMimeType:   filetype,
					StorageObjectName: objectName,
					StorageFilename:   filename,
					StorageHash:       hash,
					Content:           content,
				},
			)
			helpers.Relax(err)

			_, err = helpers.EventlogLog(time.Now(), channel.GuildID, channel.GuildID,
				models.EventlogTargetTypeGuild, msg.Author.ID,
				models.EventlogTypeRobyulCommandsAdd, "",
				nil,
				[]models.ElasticEventlogOption{
					{
						Key:   "command_keyword",
						Value: args[1],
					},
					{
						Key:   "command_content",
						Value: strings.TrimSpace(strings.Replace(content, strings.Join(args[:2], " "), "", 1)),
					},
				}, false)
			helpers.RelaxLog(err)

			_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.add-success"))
			helpers.Relax(err)
			customCommandsCache, err = cc.getAllCustomCommands()
			helpers.Relax(err)
			return
		case "random": // [p]commands random
			session.ChannelTyping(msg.ChannelID)

			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)

			var entryBucket models.CustomCommandsEntry
			// TODO: pipe aggregation
			err = helpers.MdbCollection(models.CustomCommandsTable).Pipe(
				[]bson.M{{"$match": bson.M{"guildid": channel.GuildID}}, {"$sample": bson.M{"size": 1}}},
			).One(&entryBucket)
			if err != nil && strings.Contains(err.Error(), "not found") {
				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.list-empty"))
				helpers.Relax(err)
				return
			}
			helpers.Relax(err)

			author, err := helpers.GetUserWithoutAPI(entryBucket.CreatedByUserID)
			authorText := "N/A"
			if err != nil {
				authorText = "N/A"
			} else {
				authorText = "@" + author.Username + "#" + author.Discriminator
			}

			content, filename, data := cc.getCommandContent(entryBucket)
			messageSend := &discordgo.MessageSend{
				Content: fmt.Sprintf("`%s%s` by **%s** triggered **%d times**:\n%s",
					helpers.GetPrefixForServer(channel.GuildID), entryBucket.Keyword,
					authorText,
					entryBucket.Triggered,
					content,
				),
			}
			if data != nil && len(data) > 0 {
				messageSend.Files = []*discordgo.File{
					{
						Name:   filename,
						Reader: bytes.NewReader(data),
					},
				}
			}
			messages, err := helpers.SendComplex(msg.ChannelID, messageSend)
			helpers.Relax(err)

			// TODO: update triggered in cache
			// increase triggered in DB by one
			err = helpers.MDbUpdate(models.CustomCommandsTable, entryBucket.ID, bson.M{"$inc": bson.M{"triggered": 1}})
			helpers.RelaxLog(err)
			metrics.CustomCommandsTriggered.Add(1)

			if len(messages) <= 0 {
				return
			}

			err = session.MessageReactionAdd(msg.ChannelID, messages[0].ID, "🎲")
			if err == nil {
				if err == nil {
					rerollHandler := session.AddHandler(func(session *discordgo.Session, reaction *discordgo.MessageReactionAdd) {
						defer helpers.Recover()

						if reaction.MessageID == messages[0].ID {
							if reaction.UserID == session.State.User.ID {
								return
							}

							if reaction.UserID == msg.Author.ID && reaction.Emoji.Name == "🎲" {
								err = helpers.MdbCollection(models.CustomCommandsTable).Pipe(
									[]bson.M{{"$match": bson.M{"guildid": channel.GuildID}}, {"$sample": bson.M{"size": 1}}},
								).One(&entryBucket)
								if err != nil && strings.Contains(err.Error(), "not found") {
									_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.list-empty"))
									helpers.Relax(err)
									return
								}
								helpers.Relax(err)

								author, err := helpers.GetUserWithoutAPI(entryBucket.CreatedByUserID)
								authorText := "N/A"
								if err != nil {
									authorText = "N/A"
								} else {
									authorText = "@" + author.Username + "#" + author.Discriminator
								}

								content, _, _ := cc.getCommandContent(entryBucket)
								content = fmt.Sprintf("`%s%s` by **%s** triggered **%d times**:\n%s",
									helpers.GetPrefixForServer(channel.GuildID), entryBucket.Keyword,
									authorText,
									entryBucket.Triggered,
									content,
								)
								messageEdit := &discordgo.MessageEdit{
									Content: &content,
									ID:      messages[0].ID,
									Channel: messages[0].ChannelID,
								}

								helpers.EditComplex(messageEdit)
								session.MessageReactionRemove(reaction.ChannelID, reaction.MessageID, reaction.Emoji.Name, reaction.UserID)

								helpers.MDbUpdate(models.CustomCommandsTable, entryBucket.ID, bson.M{"$inc": bson.M{"triggered": 1}})
								metrics.CustomCommandsTriggered.Add(1)
							}
						}
					})
					time.Sleep(5 * time.Minute)
					rerollHandler()
					session.MessageReactionRemove(msg.ChannelID, messages[0].ID, "🎲", session.State.User.ID)
				}
			}
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

			var entryBucket []models.CustomCommandsEntry
			if topCommands {
				err := helpers.MDbIter(helpers.MdbCollection(models.CustomCommandsTable).Find(bson.M{"guildid": channel.GuildID}).Sort("-triggered")).All(&entryBucket)
				helpers.Relax(err)
			} else {
				err := helpers.MDbIter(helpers.MdbCollection(models.CustomCommandsTable).Find(bson.M{"guildid": channel.GuildID}).Sort("keyword")).All(&entryBucket)
				helpers.Relax(err)
			}

			if len(entryBucket) <= 0 {
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
			session.ChannelTyping(msg.ChannelID)
			if len(args) < 2 {
				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
				helpers.Relax(err)
				return
			}
			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)

			var entryBucket models.CustomCommandsEntry
			err = helpers.MdbOne(
				helpers.MdbCollection(models.CustomCommandsTable).Find(bson.M{"guildid": channel.GuildID, "keyword": args[1]}),
				&entryBucket,
			)
			if err != nil && strings.Contains(err.Error(), "not found") {
				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.delete-not-found"))
				helpers.Relax(err)
				return
			}
			helpers.Relax(err)

			if !cc.canAddCommand(channel.GuildID, msg.Author.ID, &entryBucket) {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("mod.no_permission"))
				return
			}

			err = helpers.MDbDelete(models.CustomCommandsTable, entryBucket.ID)
			helpers.Relax(err)

			if entryBucket.StorageObjectName != "" {
				err = helpers.DeleteFile(entryBucket.StorageObjectName)
				helpers.Relax(err)
			}

			_, err = helpers.EventlogLog(time.Now(), channel.GuildID, channel.GuildID,
				models.EventlogTargetTypeGuild, msg.Author.ID,
				models.EventlogTypeRobyulCommandsDelete, "",
				nil,
				[]models.ElasticEventlogOption{
					{
						Key:   "command_keyword",
						Value: entryBucket.Keyword,
					},
					{
						Key:   "command_content",
						Value: entryBucket.Content,
					},
				}, false)
			helpers.RelaxLog(err)

			_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.delete-success"))
			helpers.Relax(err)
			customCommandsCache, err = cc.getAllCustomCommands()
			helpers.Relax(err)
			return
		case "replace", "edit": // [p]commands edit <command name> <new content>
			session.ChannelTyping(msg.ChannelID)
			if len(args) < 3 && (len(msg.Attachments) <= 0 && len(args) < 2) {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
				return
			}
			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)

			var entryBucket models.CustomCommandsEntry
			err = helpers.MdbOne(
				helpers.MdbCollection(models.CustomCommandsTable).Find(bson.M{"guildid": channel.GuildID, "keyword": args[1]}),
				&entryBucket,
			)
			if err != nil && strings.Contains(err.Error(), "not found") {
				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.edit-not-found"))
				helpers.Relax(err)
				return
			}
			helpers.Relax(err)

			if !cc.canAddCommand(channel.GuildID, msg.Author.ID, &entryBucket) {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("mod.no_permission"))
				return
			}

			var objectName, filetype, filename, hash string
			if len(msg.Attachments) > 0 {
				data, err := helpers.NetGetUAWithError(msg.Attachments[0].URL, helpers.DEFAULT_UA)
				helpers.Relax(err)

				filename = msg.Attachments[0].Filename

				filetype, err = helpers.SniffMime(data)
				helpers.Relax(err)

				// filetype allowed? (picture or video)
				if cc.isAllowedFiletype(filetype) {
					// user is allowed to upload files?
					if helpers.UseruploadsIsDisabled(msg.Author.ID) {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.errors.useruploads-disabled"))
						return
					}
					// <= 20 MB
					if msg.Attachments[0].Size > 20e+6 {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.fileupload-too-big"))
						return
					}
					// get object name
					objectName = models.CustomCommandsNewObjectName(channel.GuildID, msg.Author.ID)
					// upload to object storage
					err = helpers.UploadFile(objectName, data, map[string]string{
						"userid":    msg.Author.ID,
						"channelid": msg.ChannelID,
						"guildid":   channel.GuildID,
					})
					helpers.Relax(err)
				}
				hash = helpers.GetMD5Hash(objectName)
			}

			beforeContent := entryBucket.Content

			if entryBucket.StorageObjectName != "" {
				err = helpers.DeleteFile(entryBucket.StorageObjectName)
				helpers.RelaxLog(err)
			}

			content := strings.TrimSpace(strings.Replace(content, strings.Join(args[:2], " "), "", 1))

			if content == "" && (filetype == "" || objectName == "") {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				return
			}

			entryBucket.CreatedByUserID = msg.Author.ID
			entryBucket.CreatedAt = time.Now().UTC()
			entryBucket.Triggered = 0
			entryBucket.Content = content
			entryBucket.StorageFilename = filename
			entryBucket.StorageObjectName = objectName
			entryBucket.StorageHash = hash
			entryBucket.StorageMimeType = filetype
			err = helpers.MDbUpdate(models.CustomCommandsTable, entryBucket.ID, entryBucket)
			helpers.Relax(err)

			_, err = helpers.EventlogLog(time.Now(), channel.GuildID, channel.GuildID,
				models.EventlogTargetTypeGuild, msg.Author.ID,
				models.EventlogTypeRobyulCommandsUpdate, "",
				[]models.ElasticEventlogChange{
					{
						Key:      "command_content",
						OldValue: beforeContent,
						NewValue: entryBucket.Content,
					},
				},
				[]models.ElasticEventlogOption{
					{
						Key:   "command_keyword",
						Value: entryBucket.Keyword,
					},
				}, false)
			helpers.RelaxLog(err)

			_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.edit-success"))
			helpers.Relax(err)
			customCommandsCache, err = cc.getAllCustomCommands()
			helpers.Relax(err)
			return
		case "refresh": // [p]commands refresh
			helpers.RequireBotAdmin(msg, func() {
				session.ChannelTyping(msg.ChannelID)
				var err error
				customCommandsCache, err = cc.getAllCustomCommands()
				helpers.Relax(err)
				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.refreshed-commands"))
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

			var entryBucket []models.CustomCommandsEntry
			err = helpers.MDbIter(helpers.MdbCollection(models.CustomCommandsTable).Find(bson.M{"guildid": channel.GuildID, "keyword": bson.M{"$regex": bson.RegEx{Pattern: `.*` + args[1] + `.*`, Options: "i"}}}).Sort("keyword")).All(&entryBucket)
			helpers.Relax(err)
			if len(entryBucket) <= 0 {
				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.customcommands.search-empty", args[1]))
				helpers.Relax(err)
				return
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

			var entryBucket models.CustomCommandsEntry
			err = helpers.MdbOne(
				helpers.MdbCollection(models.CustomCommandsTable).Find(bson.M{"guildid": channel.GuildID, "keyword": args[1]}),
				&entryBucket,
			)
			if err != nil && strings.Contains(err.Error(), "not found") {
				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.customcommands.info-not-found"))
				helpers.Relax(err)
				return
			}
			helpers.Relax(err)

			author, err := helpers.GetUser(entryBucket.CreatedByUserID)
			helpers.Relax(err)

			content, filename, data := cc.getCommandContent(entryBucket)
			messageSend := &discordgo.MessageSend{
				Embed: &discordgo.MessageEmbed{
					Title:       fmt.Sprintf("Custom Command: `%s%s`", helpers.GetPrefixForServer(channel.GuildID), entryBucket.Keyword),
					Description: content,
					Fields: []*discordgo.MessageEmbedField{
						{Name: "Author", Value: fmt.Sprintf("%s#%s", author.Username, author.Discriminator)},
						{Name: "Times triggered", Value: humanize.Comma(int64(entryBucket.Triggered))},
						{Name: "Created At", Value: fmt.Sprintf("%s UTC", entryBucket.CreatedAt.Format(time.ANSIC))},
					},
				},
			}
			if data != nil && len(data) > 0 {
				messageSend.Files = []*discordgo.File{
					{
						Name:   strings.Replace(filename, " ", "-", -1),
						Reader: bytes.NewReader(data),
					},
				}
				if strings.HasPrefix(entryBucket.StorageMimeType, "image/") {
					messageSend.Embed.Image = &discordgo.MessageEmbedImage{URL: "attachment://" + strings.Replace(filename, " ", "-", -1)}
				}
			}

			_, err = helpers.SendComplex(msg.ChannelID, messageSend)
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

				var entryBucket []models.CustomCommandsEntry
				err = helpers.MDbIter(helpers.MdbCollection(models.CustomCommandsTable).Find(bson.M{"guildid": channel.GuildID}).Sort("keyword")).All(&entryBucket)
				helpers.Relax(err)

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

					_, err = helpers.MDbInsert(
						models.CustomCommandsTable,
						models.CustomCommandsEntry{
							GuildID:         channel.GuildID,
							CreatedByUserID: msg.Author.ID,
							CreatedAt:       time.Now(),
							Triggered:       0,
							Keyword:         newCustomCommandName,
							Content:         newCustomCommandContentText,
						},
					)
					helpers.Relax(err)

					helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Imported custom command `%s`", newCustomCommandName))
					i++
				}

				_, err = helpers.EventlogLog(time.Now(), channel.GuildID, channel.GuildID,
					models.EventlogTargetTypeGuild, msg.Author.ID,
					models.EventlogTypeRobyulCommandsJsonImport, "",
					nil,
					[]models.ElasticEventlogOption{
						{
							Key:   "commands_imported",
							Value: strconv.Itoa(i),
						},
					}, false)
				helpers.RelaxLog(err)

				_, err = helpers.SendMessage(msg.ChannelID, fmt.Sprintf("<@%s> I imported **%s** custom commands.", msg.Author.ID, humanize.Comma(int64(i))))
				helpers.Relax(err)
				customCommandsCache, err = cc.getAllCustomCommands()
				helpers.Relax(err)
			})
			return
		case "export-json": // [p]export-json
			session.ChannelTyping(msg.ChannelID)
			helpers.RequireMod(msg, func() {
				channel, err := helpers.GetChannel(msg.ChannelID)
				helpers.Relax(err)
				guild, err := helpers.GetGuild(channel.GuildID)
				helpers.Relax(err)

				var entryBucket []models.CustomCommandsEntry
				err = helpers.MDbIter(helpers.MdbCollection(models.CustomCommandsTable).Find(bson.M{"guildid": channel.GuildID}).Sort("keyword")).All(&entryBucket)
				helpers.Relax(err)

				jsonObj := gabs.New()
				for _, command := range entryBucket {
					jsonObj.Set(command.Content, command.Keyword)
				}
				jsonObj.StringIndent("", "  ")

				_, err = helpers.EventlogLog(time.Now(), channel.GuildID, channel.GuildID,
					models.EventlogTargetTypeGuild, msg.Author.ID,
					models.EventlogTypeRobyulCommandsJsonExport, "",
					nil,
					nil, false)
				helpers.RelaxLog(err)

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

// checks if the user can add or edit a command
// guildID		: the guild on which the user wants to add a command
// userID		: the user which wants to add the command
// editCommand	: if not nil, will check if user is allowed to edit that command
func (cc *CustomCommands) canAddCommand(guildID, userID string, editCommand *models.CustomCommandsEntry) (allowed bool) {
	// is mod or admin?
	if helpers.IsModByID(guildID, userID) {
		return true
	}
	// created the command in question?
	if editCommand != nil {
		if editCommand.CreatedByUserID == userID {
			return true
		} else {
			return false
		}
	}
	// everyone can add commands?
	if helpers.GuildSettingsGetCached(guildID).CustomCommandsEveryoneCanAdd {
		return true
	}
	// everyone with specific role can add commands, user got specific role?
	addRoleID := helpers.GuildSettingsGetCached(guildID).CustomCommandsAddRoleID
	if addRoleID != "" {
		member, err := helpers.GetGuildMemberWithoutApi(guildID, userID)
		helpers.RelaxLog(err)
		if err == nil {
			for _, memberRole := range member.Roles {
				if memberRole == addRoleID {
					return true
				}
			}
		}
	}
	// not allowed
	return false
}

// checks if a filetype is allowed for uploads
// filetype	: the filetype to check
func (cc *CustomCommands) isAllowedFiletype(filetype string) (allowed bool) {
	for _, allowedFiletype := range customCommandsAllowedFiletypes {
		if allowedFiletype == filetype {
			return true
		}
	}
	return false
}

func (cc *CustomCommands) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowedSilent(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermCustomCommands) {
		return
	}

	channel, err := helpers.GetChannel(msg.ChannelID)
	if err != nil {
		helpers.RelaxLog(err)
		return
	}
	prefix := helpers.GetPrefixForServer(channel.GuildID)

	for i, customCommand := range customCommandsCache {
		if customCommand.GuildID == channel.GuildID && prefix+customCommand.Keyword == content {
			session.ChannelTyping(msg.ChannelID)
			content, filename, data := cc.getCommandContent(customCommand)
			messageSend := &discordgo.MessageSend{
				Content: content,
			}
			if data != nil && len(data) > 0 {
				messageSend.Files = []*discordgo.File{
					{
						Name:   filename,
						Reader: bytes.NewReader(data),
					},
				}
			}
			_, err = helpers.SendComplex(msg.ChannelID, messageSend)
			if err != nil {
				helpers.RelaxLog(err)
				return
			}
			customCommandsCache[i].Triggered += 1

			// increase triggered in DB by one
			err = helpers.MDbUpdate(models.CustomCommandsTable, customCommandsCache[i].ID, bson.M{"$inc": bson.M{"triggered": 1}})
			helpers.RelaxLog(err)

			metrics.CustomCommandsTriggered.Add(1)
			return
		}
	}
}

func (cc *CustomCommands) getCommandContent(customCommand models.CustomCommandsEntry) (content, filename string, data []byte) {
	if customCommand.Content != "" {
		content += customCommand.Content + "\n"
	}
	if customCommand.StorageHash != "" {
		content += helpers.GetPublicFileLink(customCommand.StorageFilename, customCommand.StorageHash)
		return content, "", nil
	}
	if customCommand.StorageObjectName != "" {
		data, filename = cc.getCommandFile(customCommand)
		if data != nil {
			return content, filename, data
		}
	}
	return content, "", nil
}

func (cc *CustomCommands) getCommandFile(customCommand models.CustomCommandsEntry) (data []byte, filename string) {
	if customCommand.StorageMimeType == "" || customCommand.StorageObjectName == "" {
		return data, filename
	}

	data, err := helpers.RetrieveFile(customCommand.StorageObjectName)
	if err != nil {
		helpers.RelaxLog(err)
		return data, filename
	}
	filename = "Robyul " + customCommand.Keyword
	extension, err := mime.ExtensionsByType(customCommand.StorageMimeType)
	helpers.RelaxLog(err)
	if err == nil && extension != nil && len(extension) > 0 {
		filename += extension[0]
	}
	return data, filename
}

func (cc *CustomCommands) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {
}

func (cc *CustomCommands) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {
}

func (cc *CustomCommands) getAllCustomCommands() (ccommands []models.CustomCommandsEntry, err error) {
	err = helpers.MDbIter(helpers.MdbCollection(models.CustomCommandsTable).Find(nil)).All(&ccommands)
	if err != nil {
		return ccommands, err
	}

	metrics.CustomCommandsCount.Set(int64(len(ccommands)))
	return ccommands, nil
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
