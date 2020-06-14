package main

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"os"

	"bytes"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Seklfreak/Robyul2/modules"
	"github.com/Seklfreak/Robyul2/ratelimits"
	"github.com/bwmarrin/discordgo"
	raven "github.com/getsentry/raven-go"
	"github.com/sirupsen/logrus"
)

var (
	didLaunch = false
)

func BotOnReady(session *discordgo.Session, event *discordgo.Ready) {
	if !didLaunch {
		OnFirstReady(session, event)
		didLaunch = true
	} else {
		// OnReconnect(session, event)
	}
}

func OnFirstReady(session *discordgo.Session, event *discordgo.Ready) {
	log := cache.GetLogger()

	log.WithField("module", "bot").Info("Connected to discord!")
	log.WithField("module", "bot").Info("Invite link: " + fmt.Sprintf(
		"https://discordapp.com/oauth2/authorize?client_id=%s&scope=bot&permissions=%s",
		helpers.GetConfig().Path("discord.id").Data().(string),
		helpers.GetConfig().Path("discord.perms").Data().(string),
	))

	for _, guild := range session.State.Guilds {
		helpers.AddAutoleaverGuildID(guild.ID)
	}

	// request guild members from the gateway
	go func() {
		time.Sleep(5 * time.Minute)

		for _, guild := range session.State.Guilds {
			if helpers.IsBlacklistedGuild(guild.ID) {
				continue
			}

			//if guild.Large {
			err := session.RequestGuildMembers(guild.ID, "", 0)
			if err != nil && strings.Contains(err.Error(), "no websocket connection exists") {
				cache.GetLogger().WithField("module", "bot").Warn("OnFirstReady: no websocket connection exists, stopping Robyul")
				BotRuntimeChannel <- os.Interrupt
				return
			}
			helpers.RelaxLog(err)

			//cache.GetLogger().WithField("module", "bot").Debug(
			//	fmt.Sprintf("requesting guild member chunks for guild: %s",
			//		guild.ID))

			time.Sleep(1 * time.Second)
			//}
		}
	}()

	// Run ratelimiter
	ratelimits.Container.Init()

	go func() {
		time.Sleep(3 * time.Second)

		configName := helpers.GetConfig().Path("bot.name").Data().(string)

		// Change name if desired
		if configName != "" && configName != session.State.User.Username {
			session.UserUpdate(
				"",
				"",
				configName,
				session.State.User.Avatar,
				"",
			)
		}
	}()

	// go func() {
	// 	time.Sleep(60 * time.Second)
	//
	// 	helpers.UpdateBotlists()
	// }()
}

func BotDestroy() {
	modules.Uninit(cache.GetSession())
	helpers.RemoveReactionsFromPagedEmbeds()
}

func OnReconnect(session *discordgo.Session, event *discordgo.Ready) {
	cache.GetLogger().WithField("module", "bot").Info("Reconnected to discord!")

	// request guild members from the gateway
	go func() {
		time.Sleep(5 * time.Second)

		// for _, guild := range session.State.Guilds {
		// 	cache.GetLogger().WithField("module", "bot").Info("state guild:", guild.ID, guild.Name, guild.Large)
		// }
		// for _, guild := range session.State.Guilds {
		// 	cache.GetLogger().WithField("module", "bot").Info("cached state guild:", guild.ID, guild.Name, guild.Large)
		// }

		for _, guild := range session.State.Guilds {
			if helpers.IsBlacklistedGuild(guild.ID) {
				continue
			}

			//if guild.Large {
			err := session.RequestGuildMembers(guild.ID, "", 0)
			if err != nil && strings.Contains(err.Error(), "no websocket connection exists") {
				cache.GetLogger().WithField("module", "bot").Warn("OnReconnect: no websocket connection exists, stopping Robyul")
				BotRuntimeChannel <- os.Interrupt
				return
			}
			helpers.RelaxLog(err)

			//cache.GetLogger().WithField("module", "bot").Debug(
			//	fmt.Sprintf("requesting guild member chunks for guild: %s",
			//		guild.ID))

			time.Sleep(1 * time.Second)
			//}
		}
	}()

	// go func() {
	// 	time.Sleep(60 * time.Second)
	//
	// 	helpers.UpdateBotlists()
	// }()
}

func BotOnMemberListChunk(session *discordgo.Session, members *discordgo.GuildMembersChunk) {
	//cache.GetLogger().WithField("module", "bot").Debug(
	//	fmt.Sprintf("received guild member chunk for guild: %s (%d received)",
	//	members.GuildID, len(members.Members)))
}

func BotGuildOnPresenceUpdate(session *discordgo.Session, presence *discordgo.PresenceUpdate) {
	if presence.GuildID == "" {
		return
	}

	member, err := session.State.Member(presence.GuildID, presence.User.ID)
	if err != nil {
		if strings.Contains(err.Error(), "state cache not found") {
			return
		}
		raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
		return
	}

	change := false
	if presence.User.Avatar != "" {
		member.User.Avatar = presence.User.Avatar
		change = true
	}
	if presence.User.Discriminator != "" {
		member.User.Discriminator = presence.User.Discriminator
		change = true
	}
	if presence.User.Email != "" {
		member.User.Email = presence.User.Email
		change = true
	}
	if presence.User.Token != "" {
		member.User.Token = presence.User.Token
		change = true
	}
	if presence.User.Username != "" {
		member.User.Username = presence.User.Username
		change = true
	}

	if change == true {
		err = session.State.MemberAdd(member)
		if err != nil {
			raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
		}
	}
}

func BotOnGuildMemberAdd(session *discordgo.Session, member *discordgo.GuildMemberAdd) {
	modules.CallExtendedPluginOnGuildMemberAdd(
		member.Member,
	)
}

func BotOnGuildMemberRemove(session *discordgo.Session, member *discordgo.GuildMemberRemove) {
	modules.CallExtendedPluginOnGuildMemberRemove(
		member.Member,
	)
}

func BotOnGuildBanAdd(session *discordgo.Session, user *discordgo.GuildBanAdd) {
	modules.CallExtendedPluginOnGuildBanAdd(
		user,
	)
}

func BotOnGuildBanRemove(session *discordgo.Session, user *discordgo.GuildBanRemove) {
	modules.CallExtendedPluginOnGuildBanRemove(
		user,
	)
}

// BotOnMessageCreate gets called after a new message was sent
// This will be called after *every* message on *every* server so it should die as soon as possible
// or spawn costly work inside of coroutines.
func BotOnMessageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	// Ignore other bots
	if message.Author.Bot {
		return
	}

	if helpers.IsBlacklisted(message.Author.ID) {
		return
	}

	if !cache.IsSession() {
		return
	}

	// Get the channel
	// Ignore the event if we cannot resolve the channel
	channel, err := helpers.GetChannelWithoutApi(message.ChannelID)
	if err != nil {
		return
	}

	if channel.Type == discordgo.ChannelTypeDM {
		return
	}

	if helpers.IsBlacklistedGuild(channel.GuildID) {
		return
	}

	if !helpers.GuildIsOnWhitelist(message.GuildID) {
		return
	}

	// Check if the message contains @mentions for us
	if strings.HasPrefix(message.Content, "<@") && len(message.Mentions) > 0 && message.Mentions[0].ID == session.State.User.ID {
		// Consume a key for this action
		e := ratelimits.Container.Drain(1, message.Author.ID)
		if e != nil {
			return
		}

		// Prepare content for editing
		msg := message.Content

		/// Remove our @mention and @nickname mention
		msg = strings.Replace(msg, "<@"+session.State.User.ID+">", "", -1)
		msg = strings.Replace(msg, "<@!"+session.State.User.ID+">", "", -1)

		// Trim message
		msg = strings.TrimSpace(msg)

		// Convert to []byte before matching
		bmsg := []byte(msg)

		// Match against common task patterns
		// Send to cleverbot if nothing matches
		switch {
		case regexp.MustCompile("(?i)^HELP.*").Match(bmsg):
			metrics.CommandsExecuted.Add(1)
			sendHelp(message)
			return

		case regexp.MustCompile("(?i)^PREFIX.*").Match(bmsg):
			metrics.CommandsExecuted.Add(1)
			prefix := helpers.GetPrefixForServer(channel.GuildID)
			if prefix == "" {
				helpers.SendMessage(
					channel.ID,
					helpers.GetText("bot.prefix.not-set"),
				)
				return
			}

			helpers.SendMessage(
				channel.ID,
				helpers.GetTextF("bot.prefix.is", prefix),
			)
			return

		case regexp.MustCompile("(?i)^SET PREFIX (.){1,25}$").Match(bmsg):
			metrics.CommandsExecuted.Add(1)
			helpers.RequireAdmin(message.Message, func() {
				// Extract prefix
				prefix := strings.Fields(regexp.MustCompile("(?i)^SET PREFIX\\s").ReplaceAllString(msg, ""))[0]

				// Set new prefix
				settings := helpers.GuildSettingsGetCached(channel.GuildID)

				oldPrefix := settings.Prefix

				settings.Prefix = prefix
				err = helpers.GuildSettingsSet(channel.GuildID, settings)
				helpers.Relax(err)

				_, err = helpers.EventlogLog(time.Now(), channel.GuildID, channel.GuildID,
					models.EventlogTargetTypeGuild, message.Author.ID,
					models.EventlogTypeRobyulPrefixUpdate, "",
					[]models.ElasticEventlogChange{
						{
							Key:      "prefix",
							OldValue: oldPrefix,
							NewValue: settings.Prefix,
						},
					},
					nil, false)
				helpers.RelaxLog(err)

				if err != nil {
					helpers.SendError(message.Message, err)
				} else {
					helpers.SendMessage(channel.ID,
						helpers.GetTextF("plugins.mod.prefix-set-success",
							helpers.GetPrefixForServer(channel.GuildID)))
				}
			})
			return

		case regexp.MustCompile("(?i)^SHUTDOWN.*").Match(bmsg):
			helpers.RequireBotAdmin(message.Message, func() {
				session.ChannelTyping(message.ChannelID)

				if helpers.ConfirmEmbed(message.GuildID, message.ChannelID, message.Author,
					"Are you sure you want me to shutdown Robyul?", "✅", "🚫") {
					cache.GetLogger().WithField("module", "debug").Warnf("shutting down Robuyul on request by %s#%s (%s)",
						message.Author.Username, message.Author.Discriminator, message.Author.ID)
					BotRuntimeChannel <- os.Interrupt
				}
			})

		default:
			if !helpers.ModuleIsAllowed(message.ChannelID, message.ID, message.Author.ID, helpers.ModulePermMisc) {
				return
			}

			// Resolve other @mentions before sending the message
			for _, user := range message.Mentions {
				msg = strings.Replace(msg, "<@"+user.ID+">", user.Username, -1)
			}

			if helpers.IsEmoji(msg) {
				// send big emoji
				emojiID, emojiName, animated := helpers.ParseCustomEmoji(msg)
				// download and send custom emoji
				if emojiID != "" {
					fileName := emojiName
					if !animated {
						fileName += ".png"
					} else {
						fileName += ".gif"
					}
					emojiData, err := helpers.NetGetUAWithError(
						helpers.EmojIURL(emojiID, animated), helpers.DEFAULT_UA,
					)
					helpers.RelaxLog(err)
					if err == nil {
						_, err = helpers.SendComplex(message.ChannelID, &discordgo.MessageSend{
							Files: []*discordgo.File{
								{
									Name:   fileName,
									Reader: bytes.NewReader(emojiData),
								},
							},
						})
						if err != nil {
							if errD, ok := err.(*discordgo.RESTError); ok && errD.Message != nil {
								if errD.Message.Code == discordgo.ErrCodeMissingPermissions {
									return
								}
							}
						}
						helpers.RelaxLog(err)
						return
					}
				}
				// send builtin emoji
				for character := range msg {
					filename := emojiFile(helpers.GetConfig().Path("assets_folder").Data().(string)+"/twemoji72", msg[character:])
					if filename != "" {
						emojiFile, err := os.Open(filename)
						if err == nil {
							defer emojiFile.Close()
							_, err = helpers.SendComplex(message.ChannelID, &discordgo.MessageSend{
								Files: []*discordgo.File{
									{
										Name:   "emoji.png",
										Reader: emojiFile,
									},
								},
							})
							if err != nil {
								if errD, ok := err.(*discordgo.RESTError); ok && errD.Message != nil {
									if errD.Message.Code == discordgo.ErrCodeMissingPermissions {
										return
									}
								}
							}
							helpers.RelaxLog(err)
							return
						}
					}
				}
				return
			}
			return
		}
	}

	modules.CallExtendedPlugin(
		message.Content,
		message.Message,
	)

	// Only continue if a prefix is set
	prefix := helpers.GetPrefixForServer(channel.GuildID)
	if prefix == "" {
		return
	}

	// Check if the message is prefixed for us
	// If not exit
	if !strings.HasPrefix(message.Content, prefix) {
		robyulIsMentioned := false
		for _, mention := range message.Mentions {
			if mention == nil {
				continue
			}
			if mention.ID == session.State.User.ID {
				robyulIsMentioned = true
			}
		}
		if robyulIsMentioned {
			reactions := []string{
				"a:ablobwave:393869340975300638",
				"a:ablobgrimace:394026913108328449",
				"a:ablobwink:394026912436977665",
				"a:ablobshocked:394026914076950539",
				":blobglare:317044032658341888",
				":blobonfire:317034288896016384",
				":blobsalute:317043033004703744",
				":blobthinkingeyes:317044481499201538",
				":googleghost:317030645786476545",
			}
			cache.GetSession().SessionForGuildS(message.GuildID).MessageReactionAdd(message.ChannelID, message.ID, reactions[rand.Intn(len(reactions))])
		}
		return
	}

	// Check if the user is allowed to request commands
	if !ratelimits.Container.HasKeys(message.Author.ID) && !helpers.IsBotAdmin(message.Author.ID) {
		helpers.SendMessage(message.ChannelID, helpers.GetTextF("bot.ratelimit.hit", message.Author.ID))

		ratelimits.Container.Set(message.Author.ID, -1)
		return
	}

	// Split the message into parts
	parts := strings.Fields(message.Content)

	// Save a sanitized version of the command (no prefix)
	cmd := strings.Replace(parts[0], prefix, "", 1)

	// Check if the user calls for help
	if cmd == "h" || cmd == "help" {
		metrics.CommandsExecuted.Add(1)
		sendHelp(message)
		return
	}

	// Separate arguments from the command
	content := strings.TrimSpace(strings.Replace(message.Content, prefix+cmd, "", -1))

	// Log commands
	cache.GetLogger().WithFields(logrus.Fields{
		"module":    "bot",
		"channelID": message.ChannelID,
		"userID":    message.Author.ID,
	}).Debug(fmt.Sprintf("%s (#%s): %s",
		message.Author.Username, message.Author.ID, message.Content))

	// Check if a module matches said command
	modules.CallBotPlugin(cmd, content, message.Message)
}

func emojiFile(base, s string) string {
	found := ""
	filename := ""
	for _, r := range s {
		if filename != "" {
			filename = fmt.Sprintf("%s-%x", filename, r)
		} else {
			filename = fmt.Sprintf("%x", r)
		}

		if _, err := os.Stat(fmt.Sprintf("%s/%s.png", base, filename)); err == nil {
			found = fmt.Sprintf("%s/%s.png", base, filename)
		} else if found != "" {
			return found
		}
	}
	return found
}

func BotOnMessageDelete(session *discordgo.Session, message *discordgo.MessageDelete) {
	if message.Author != nil {
		if helpers.IsBlacklisted(message.Author.ID) {
			return
		}
	}

	channel, err := helpers.GetChannelWithoutApi(message.ChannelID)
	if err != nil {
		return
	}

	if helpers.IsBlacklistedGuild(channel.GuildID) {
		return
	}

	modules.CallExtendedPluginOnMessageDelete(message)
}

// BotOnReactionAdd gets called after a reaction is added
// This will be called after *every* reaction added on *every* server so it
// should die as soon as possible or spawn costly work inside of coroutines.
// This is currently used for the *poll* plugin.
func BotOnReactionAdd(session *discordgo.Session, reaction *discordgo.MessageReactionAdd) {
	if reaction.UserID == session.State.User.ID {
		return
	}

	if helpers.IsBlacklisted(reaction.UserID) {
		return
	}

	channel, err := helpers.GetChannelWithoutApi(reaction.ChannelID)
	if err != nil {
		return
	}

	if helpers.IsBlacklistedGuild(channel.GuildID) {
		return
	}

	modules.CallExtendedPluginOnReactionAdd(reaction)

	// check if the reaction was added to a paged message
	if pagedMessage := helpers.GetPagedMessage(reaction.MessageID); pagedMessage != nil {
		go func() {
			defer helpers.Recover()

			pagedMessage.UpdateMessagePage(reaction)
		}()
	}
}

func BotOnReactionRemove(session *discordgo.Session, reaction *discordgo.MessageReactionRemove) {
	if reaction.UserID == session.State.User.ID {
		return
	}

	if helpers.IsBlacklisted(reaction.UserID) {
		return
	}

	channel, err := helpers.GetChannelWithoutApi(reaction.ChannelID)
	if err != nil {
		return
	}

	if helpers.IsBlacklistedGuild(channel.GuildID) {
		return
	}

	modules.CallExtendedPluginOnReactionRemove(reaction)
}

func BotOnGuildCreate(session *discordgo.Session, guild *discordgo.GuildCreate) {
}

func BotOnGuildDelete(session *discordgo.Session, guild *discordgo.GuildDelete) {
}

func sendHelp(message *discordgo.MessageCreate) {
	channel, err := helpers.GetChannel(message.ChannelID)
	if err != nil {
		channel.GuildID = ""
	}

	helpers.SendMessage(
		message.ChannelID,
		helpers.GetTextF("bot.help", message.Author.ID, channel.GuildID),
	)
}
