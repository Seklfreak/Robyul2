package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/emojis"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/modules"
	"github.com/Seklfreak/Robyul2/ratelimits"
	"github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
	"github.com/getsentry/raven-go"
)

var didLaunch = false

func BotOnReady(session *discordgo.Session, event *discordgo.Ready) {
	if !didLaunch {
		OnFirstReady(session, event)
		didLaunch = true
	} else {
		OnReconnect(session, event)
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
		cache.AddAutoleaverGuildID(guild.ID)
	}

	// Cache the session
	cache.SetSession(session)

	// Load and init all modules
	modules.Init(session)

	// Run async worker for guild changes
	go helpers.GuildSettingsUpdater()

	// Run async game-changer
	go changeGameInterval(session)

	// request guild members from the gateway
	go func() {
		time.Sleep(5 * time.Second)

		for _, guild := range session.State.Guilds {
			if guild.Large {
				err := session.RequestGuildMembers(guild.ID, "", 0)
				if err != nil {
					raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
				}

				cache.GetLogger().WithField("module", "bot").Debug(
					fmt.Sprintf("requesting guild member chunks for guild: %s",
						guild.ID))

				time.Sleep(1 * time.Second)
			}
		}
	}()

	// Run auto-leaver for non-beta guilds
	//go autoLeaver(session)

	// Run ratelimiter
	ratelimits.Container.Init()

	go func() {
		time.Sleep(3 * time.Second)

		configName := helpers.GetConfig().Path("bot.name").Data().(string)
		configAvatar := helpers.GetConfig().Path("bot.avatar").Data().(string)

		// Change avatar if desired
		if configAvatar != "" && configAvatar != session.State.User.Avatar {
			session.UserUpdate(
				"",
				"",
				session.State.User.Username,
				configAvatar,
				"",
			)
		}

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

	// Run async game-changer
	//go changeGameInterval(session)
}

func OnReconnect(session *discordgo.Session, event *discordgo.Ready) {
	cache.GetLogger().WithField("module", "bot").Info("Reconnected to discord!")

	// request guild members from the gateway
	go func() {
		time.Sleep(5 * time.Second)

		for _, guild := range session.State.Guilds {
			if guild.Large {
				err := session.RequestGuildMembers(guild.ID, "", 0)
				if err != nil {
					raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
				}

				cache.GetLogger().WithField("module", "bot").Debug(
					fmt.Sprintf("requesting guild member chunks for guild: %s",
						guild.ID))

				time.Sleep(1 * time.Second)
			}
		}
	}()
}

func BotOnMemberListChunk(session *discordgo.Session, members *discordgo.GuildMembersChunk) {
	i := 0
	var err error
	for _, member := range members.Members {
		member.GuildID = members.GuildID
		err = session.State.MemberAdd(member)
		if err != nil {
			raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
		} else {
			i++
		}
	}
	cache.GetLogger().WithField("module", "bot").Debug(
		fmt.Sprintf("received guild member chunk for guild: %s (%d/%d received/added)",
			members.GuildID, len(members.Members), i))
}

func BotGuildOnPresenceUpdate(session *discordgo.Session, presence *discordgo.PresenceUpdate) {
	if presence.GuildID == "" {
		return
	}

	member, err := cache.GetSession().State.Member(presence.GuildID, presence.User.ID)
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
	// Ignore other bots and @everyone/@here
	if message.Author.Bot || message.MentionEveryone {
		return
	}

	if helpers.IsBlacklisted(message.Author.ID) {
		return
	}

	// Get the channel
	// Ignore the event if we cannot resolve the channel
	channel, err := helpers.GetChannel(message.ChannelID)
	if err != nil {
		go raven.CaptureError(err, map[string]string{})
		return
	}

	/*
		if channel.Type == discordgo.ChannelTypeDM {
			// Track usage
			metrics.CleverbotRequests.Add(1)

			// Mark typing
			session.ChannelTyping(message.ChannelID)

			// Prepare content for editing
			msg := message.Content

			/// Remove our @mention
			msg = strings.Replace(msg, "<@"+session.State.User.ID+">", "", -1)

			// Trim message
			msg = strings.TrimSpace(msg)

			// Resolve other @mentions before sending the message
			for _, user := range message.Mentions {
				msg = strings.Replace(msg, "<@"+user.ID+">", user.Username, -1)
			}

			// Remove smileys
			msg = regexp.MustCompile(`:\w+:`).ReplaceAllString(msg, "")

			// Send to cleverbot
			helpers.CleverbotSend(session, channel.ID, msg)
			return
		}*/

	// Check if the message contains @mentions for us
	if strings.HasPrefix(message.Content, "<@") && len(message.Mentions) > 0 && message.Mentions[0].ID == session.State.User.ID {
		// Consume a key for this action
		e := ratelimits.Container.Drain(1, message.Author.ID)
		if e != nil {
			return
		}

		// Prepare content for editing
		msg := message.Content

		/// Remove our @mention
		msg = strings.Replace(msg, "<@"+session.State.User.ID+">", "", -1)

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
				cache.GetSession().ChannelMessageSend(
					channel.ID,
					helpers.GetText("bot.prefix.not-set"),
				)
			}

			cache.GetSession().ChannelMessageSend(
				channel.ID,
				helpers.GetTextF("bot.prefix.is", prefix),
			)
			return

			/*
				case regexp.MustCompile("(?i)^REFRESH CHAT SESSION$").Match(bmsg):
					metrics.CommandsExecuted.Add(1)
					helpers.RequireAdmin(message.Message, func() {
						// Refresh cleverbot session
						helpers.CleverbotRefreshSession(channel.ID)
						cache.GetSession().ChannelMessageSend(channel.ID, helpers.GetText("bot.cleverbot.refreshed"))
					})
					return*/

		case regexp.MustCompile("(?i)^SET PREFIX (.){1,25}$").Match(bmsg):
			metrics.CommandsExecuted.Add(1)
			helpers.RequireAdmin(message.Message, func() {
				// Extract prefix
				prefix := strings.Fields(regexp.MustCompile("(?i)^SET PREFIX\\s").ReplaceAllString(msg, ""))[0]

				// Set new prefix
				err := helpers.SetPrefixForServer(
					channel.GuildID,
					prefix,
				)

				if err != nil {
					helpers.SendError(message.Message, err)
				} else {
					cache.GetSession().ChannelMessageSend(channel.ID, helpers.GetTextF("bot.prefix.saved", prefix))
				}
			})
			return

			/*
				default:
					// Track usage
					metrics.CleverbotRequests.Add(1)

					// Mark typing
					session.ChannelTyping(message.ChannelID)

					// Resolve other @mentions before sending the message
					for _, user := range message.Mentions {
						msg = strings.Replace(msg, "<@"+user.ID+">", user.Username, -1)
					}

					// Remove smileys
					msg = regexp.MustCompile(`:\w+:`).ReplaceAllString(msg, "")

					// Send to cleverbot
					helpers.CleverbotSend(session, channel.ID, msg)
					return*/
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
		return
	}

	// Check if the user is allowed to request commands
	if !ratelimits.Container.HasKeys(message.Author.ID) && !helpers.IsBotAdmin(message.Author.ID) {
		session.ChannelMessageSend(message.ChannelID, helpers.GetTextF("bot.ratelimit.hit", message.Author.ID))

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

	// Check if a trigger matches
	modules.CallTriggerPlugin(cmd, content, message.Message)
}

func BotOnMessageDelete(session *discordgo.Session, message *discordgo.MessageDelete) {
	modules.CallExtendedPluginOnMessageDelete(message)
}

// BotOnReactionAdd gets called after a reaction is added
// This will be called after *every* reaction added on *every* server so it
// should die as soon as possible or spawn costly work inside of coroutines.
// This is currently used for the *poll* plugin.
func BotOnReactionAdd(session *discordgo.Session, reaction *discordgo.MessageReactionAdd) {
	modules.CallExtendedPluginOnReactionAdd(reaction)

	if reaction.UserID == session.State.User.ID {
		return
	}

	channel, err := helpers.GetChannel(reaction.ChannelID)
	if err != nil {
		return
	}
	if emojis.ToNumber(reaction.Emoji.Name) == -1 {
		//session.MessageReactionRemove(reaction.ChannelID, reaction.MessageID, reaction.Emoji.Name, reaction.UserID)
		return
	}
	if helpers.VotePollIfItsOne(channel.GuildID, reaction.MessageReaction) {
		helpers.UpdatePollMsg(channel.GuildID, reaction.MessageID)
	}

}

func BotOnReactionRemove(session *discordgo.Session, reaction *discordgo.MessageReactionRemove) {
	modules.CallExtendedPluginOnReactionRemove(reaction)
}

func sendHelp(message *discordgo.MessageCreate) {
	channel, err := helpers.GetChannel(message.ChannelID)
	if err != nil {
		channel.GuildID = ""
	}

	cache.GetSession().ChannelMessageSend(
		message.ChannelID,
		helpers.GetTextF("bot.help", message.Author.ID, channel.GuildID),
	)
}

// Changes the game interval every ten minutes after called
func changeGameInterval(session *discordgo.Session) {
	for {
		guilds := session.State.Guilds

		err := session.UpdateStatus(0, fmt.Sprintf("on %d servers | robyul.chat | _help", len(guilds)))
		if err != nil {
			raven.CaptureError(err, map[string]string{})
		}

		time.Sleep(1 * time.Hour)
	}
}
