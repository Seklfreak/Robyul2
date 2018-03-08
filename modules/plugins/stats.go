package plugins

import (
	"fmt"
	"math"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/globalsign/mgo/bson"

	"context"

	"reflect"

	"encoding/json"

	"sync"

	"github.com/Jeffail/gabs"
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/emojis"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Seklfreak/Robyul2/version"
	"github.com/bradfitz/slice"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/getsentry/raven-go"
	"github.com/gorethink/gorethink"
	"github.com/olivere/elastic"
)

type Stats struct{}

func (s *Stats) Commands() []string {
	return []string{
		"stats",
		"serverinfo",
		"userinfo",
		"voicestats",
		"emotes",
		"emojis",
		"emoji",
		"memberlist",
		"members",
		"invite",
		"channelinfo",
		"serverindex",
		"roles",
		"rolelist",
		"channels",
		"channellist",
	}
}

var (
	VoiceSessionStarts     []VoiceSessionStart
	VoiceSessionStartsLock sync.Mutex
)

const (
	VOICE_SESSION_SAVE_DURATION_MIN_SECONDS = 60
)

type VoiceSessionStart struct {
	UserID    string
	ChannelID string
	GuildID   string
	JoinTime  time.Time
}

func (s *Stats) handleVoiceStateUpdate(session *discordgo.Session, update *discordgo.VoiceStateUpdate) {
	defer helpers.Recover()

	if update == nil || update.GuildID == "" ||
		helpers.IsBlacklistedGuild(update.GuildID) || helpers.IsLimitedGuild(update.GuildID) {
		return
	}

	user, err := helpers.GetUser(update.UserID)
	helpers.Relax(err)
	if user.Bot || helpers.IsBlacklisted(user.ID) {
		return
	}

	VoiceSessionStartsLock.Lock()
	defer VoiceSessionStartsLock.Unlock()
	if update.ChannelID == "" || update.SelfDeaf == true || update.Deaf == true {
		// disconnect
		newVoiceSessionStarts := make([]VoiceSessionStart, 0)
		for _, voiceSessionStart := range VoiceSessionStarts {
			if voiceSessionStart.UserID == update.UserID && voiceSessionStart.GuildID == update.GuildID {
				channelID := voiceSessionStart.ChannelID
				start := voiceSessionStart.JoinTime
				now := time.Now()
				go func() {
					defer helpers.Recover()

					if now.Sub(start).Seconds() < VOICE_SESSION_SAVE_DURATION_MIN_SECONDS {
						return
					}

					err := helpers.ElasticAddVoiceSession(update.GuildID, channelID, update.UserID,
						start, now)
					helpers.Relax(err)
					cache.GetLogger().WithField("module", "stats").Infof(
						"saved voice session Guild #%s User #%s Channel #%s Duration %d (disconnect)",
						update.GuildID, channelID, update.UserID, int(now.Sub(start).Seconds()),
					)
				}()
			} else {
				newVoiceSessionStarts = append(newVoiceSessionStarts, voiceSessionStart)
			}
		}
		VoiceSessionStarts = newVoiceSessionStarts
	} else if update.ChannelID != "" {
		// connect
		newVoiceSessionStarts := make([]VoiceSessionStart, 0)
		for _, voiceSessionStart := range VoiceSessionStarts {
			if voiceSessionStart.UserID == update.UserID {
				if voiceSessionStart.ChannelID == update.ChannelID {
					// nothing changed
					return
				}
				// change channel
				channelID := voiceSessionStart.ChannelID
				guildID := voiceSessionStart.GuildID
				start := voiceSessionStart.JoinTime
				now := time.Now()
				go func() {
					defer helpers.Recover()

					if now.Sub(start).Seconds() < VOICE_SESSION_SAVE_DURATION_MIN_SECONDS {
						return
					}

					err := helpers.ElasticAddVoiceSession(guildID, channelID, update.UserID,
						start, now)
					helpers.Relax(err)
					cache.GetLogger().WithField("module", "stats").Infof(
						"saved voice session Guild #%s User #%s Channel #%s Duration %d (channel change)",
						guildID, channelID, update.UserID, int(now.Sub(start).Seconds()),
					)
				}()
				continue
			}
			newVoiceSessionStarts = append(newVoiceSessionStarts, voiceSessionStart)
		}
		VoiceSessionStarts = newVoiceSessionStarts
		// new session
		VoiceSessionStarts = append(VoiceSessionStarts, VoiceSessionStart{
			GuildID:   update.GuildID,
			ChannelID: update.ChannelID,
			UserID:    update.UserID,
			JoinTime:  time.Now(),
		})
	}
}

func (s *Stats) Init(session *discordgo.Session) {
	VoiceSessionStarts = make([]VoiceSessionStart, 0)
	session.AddHandler(s.handleVoiceStateUpdate)
}

func (s *Stats) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermStats) {
		return
	}

	switch command {
	case "stats":
		session.ChannelTyping(msg.ChannelID)
		// Count guilds, channels and users
		users := make(map[string]string)
		channels := 0
		guilds := session.State.Guilds

		for _, guild := range guilds {
			channels += len(guild.Channels)

			for _, u := range guild.Members {
				users[u.User.ID] = u.User.Username
			}
		}

		// Get RAM stats
		var ram runtime.MemStats
		runtime.ReadMemStats(&ram)

		// Get uptime
		bootTime, err := strconv.ParseInt(metrics.Uptime.String(), 10, 64)
		if err != nil {
			bootTime = 0
		}

		uptime := helpers.HumanizeDuration(time.Now().Sub(time.Unix(bootTime, 0)))

		var activeWorkersText string
		for _, worker := range cache.GetMachineryActiveWorkers() {
			activeWorkersText += worker.ConsumerTag + " (" + strconv.Itoa(worker.Concurrency) + ")"
		}
		if activeWorkersText == "" {
			activeWorkersText = "_N/A_"
		}

		var rethinkDBStatusText string

		rethinkDBServerStatus := make(map[string]interface{}, 0)
		rethinkDBServerStatusCursor, err := gorethink.DB(gorethink.SystemDatabase).Table(gorethink.ServerStatusSystemTable).
			Run(helpers.GetDB())
		helpers.Relax(err)
		rethinkDBServerStatusCursor.One(&rethinkDBServerStatus)
		defer rethinkDBServerStatusCursor.Close()

		rethinkDBProcess, ok := rethinkDBServerStatus["process"].(map[string]interface{})
		if ok {
			startedAt, ok := rethinkDBProcess["time_started"].(time.Time)
			if ok {
				rethinkDBStatusText += "Uptime " + helpers.HumanizeDuration(time.Now().Sub(startedAt)) + "\n"
			}
			cacheSizeMb, ok := rethinkDBProcess["cache_size_mb"].(float64)
			if ok {
				rethinkDBStatusText += "Cache " + humanize.Bytes(uint64(cacheSizeMb*1000000)) + "\n"
			}
		}

		mdbAliveServers := helpers.GetMDbSession().LiveServers()
		mdbStats := bson.M{}
		err = helpers.GetMDb().Run("dbstats", &mdbStats)
		helpers.Relax(err)
		mdbServerStatus := bson.M{}
		err = helpers.GetMDbSession().Run("serverStatus", &mdbServerStatus)
		helpers.Relax(err)

		mongodbLaunched := time.Now().Add(time.Duration(int64(mdbServerStatus["uptime"].(float64))) * time.Second * -1)
		mongodbUptime := helpers.HumanizeDuration(time.Now().Sub(mongodbLaunched))

		mongodbStatusText := fmt.Sprintf("Alive Servers %d\nUptime %s",
			len(mdbAliveServers),
			mongodbUptime,
		)

		var storageSize, avgObjSize uint64

		storageSizeFloat, ok := mdbStats["storageSize"].(float64)
		if ok {
			storageSize = uint64(storageSizeFloat)
		}
		storageSizeInt, ok := mdbStats["storageSize"].(int64)
		if ok {
			storageSize = uint64(storageSizeInt)
		}
		avgObjSizeFloat, ok := mdbStats["avgObjSize"].(float64)
		if ok {
			avgObjSize = uint64(avgObjSizeFloat)
		}
		avgObjSizeFloatInt, ok := mdbStats["avgObjSize"].(int64)
		if ok {
			avgObjSize = uint64(avgObjSizeFloatInt)
		}

		mongodbStorageText := fmt.Sprintf("Size %s\nAvg Object Size %s",
			humanize.Bytes(storageSize),
			humanize.Bytes(avgObjSize),
		)

		var redisUptimeSecondsText, redisConnectedClients, redisUsedMemoryHuman string

		redisInfoText, err := cache.GetRedisClient().Info().Result()
		helpers.Relax(err)
		for _, redisInfoLine := range strings.Split(redisInfoText, "\r\n") {
			args := strings.Split(redisInfoLine, ":")
			if len(args) < 2 {
				continue
			}

			switch args[0] {
			case "uptime_in_seconds":
				redisUptimeSecondsText = args[1]
			case "connected_clients":
				redisConnectedClients = args[1]
			case "used_memory_human":
				redisUsedMemoryHuman = args[1]
			}
		}

		redisUptimeSeconds, err := strconv.Atoi(redisUptimeSecondsText)
		helpers.Relax(err)

		redisLaunched := time.Now().Add(time.Duration(redisUptimeSeconds) * time.Second * -1)
		redisUptime := helpers.HumanizeDuration(time.Now().Sub(redisLaunched))

		var elasticStatusText, elasticStatsText, elasticProcessText string

		if cache.HasElastic() {
			clusterHealth, err := cache.GetElastic().ClusterHealth().Do(context.Background())
			helpers.Relax(err)
			clusterStats, err := cache.GetElastic().ClusterStats().Do(context.Background())
			helpers.Relax(err)
			elasticStatusText += fmt.Sprintf(
				"%s\n%d node(s)\n%d pending task(s)",
				clusterStats.Status, clusterStats.Nodes.Count.Total, clusterHealth.NumberOfPendingTasks,
			)
			elasticStatsText += fmt.Sprintf("%d Indices\n%s Documents\nSize %s",
				clusterStats.Indices.Count, humanize.Comma(int64(clusterStats.Indices.Docs.Count)),
				humanize.Bytes(uint64(clusterStats.Indices.Store.SizeInBytes)))
			elasticProcessText += fmt.Sprintf(
				"Uptime %s\nMemory %s\n%d Threads",
				helpers.HumanizeDuration(
					time.Now().Sub(time.Now().Add(-1*(time.Millisecond*time.Duration(clusterStats.Nodes.JVM.MaxUptimeInMillis)))),
				),
				humanize.Bytes(uint64(clusterStats.Nodes.JVM.Mem.HeapUsedInBytes)),
				clusterStats.Nodes.JVM.Threads,
			)
		}

		pendingTasks, err := cache.GetMachineryServer().GetBroker().GetPendingTasks("robyul_tasks")
		helpers.Relax(err)

		zeroWidthWhitespace, err := strconv.Unquote(`'\u200b'`)
		helpers.Relax(err)

		statsEmbed := &discordgo.MessageEmbed{
			Color: 0x0FADED,
			Fields: []*discordgo.MessageEmbedField{
				// Build
				{Name: "Build Time", Value: version.BUILD_TIME, Inline: true},
				{Name: "Build System", Value: version.BUILD_HOST, Inline: true},
				{Name: zeroWidthWhitespace, Value: zeroWidthWhitespace, Inline: true},

				// System
				{Name: "Bot Uptime", Value: uptime, Inline: true},
				{Name: "Bot Version", Value: version.BOT_VERSION, Inline: true},
				{Name: zeroWidthWhitespace, Value: zeroWidthWhitespace, Inline: true},

				// Library
				{Name: "Go Version", Value: runtime.Version(), Inline: true},
				{Name: "discordgo Version", Value: discordgo.VERSION, Inline: true},
				{Name: "API Version", Value: discordgo.APIVersion, Inline: true},

				// Bot
				{Name: "Heap /  Sys RAM", Value: humanize.Bytes(ram.Alloc) + "/" + humanize.Bytes(ram.Sys), Inline: true},
				{Name: "Collected garbage", Value: humanize.Bytes(ram.TotalAlloc), Inline: true},
				{Name: "Running coroutines", Value: strconv.Itoa(runtime.NumGoroutine()), Inline: true},

				// Discord
				{Name: "Connected servers", Value: strconv.Itoa(len(guilds)), Inline: true},
				{Name: "Watching channels", Value: strconv.Itoa(channels), Inline: true},
				{Name: "Users with access to me", Value: strconv.Itoa(len(users)), Inline: true},

				// Machinery
				{Name: "Pending / Delayed Tasks", Value: strconv.Itoa(len(pendingTasks)) + " / " + strconv.Itoa(int(metrics.MachineryDelayedTasksCount.Value())), Inline: true},
				{Name: "Active Workers", Value: activeWorkersText, Inline: true},
				{Name: zeroWidthWhitespace, Value: zeroWidthWhitespace, Inline: true},

				// RethinkDB
				{Name: "RethinkDB", Value: rethinkDBStatusText, Inline: false},

				// MongoDB
				{Name: "MongoDB Status", Value: mongodbStatusText, Inline: true},
				{Name: "MongoDB Storage", Value: mongodbStorageText, Inline: true},
				{Name: zeroWidthWhitespace, Value: zeroWidthWhitespace, Inline: true},

				// Redis
				{Name: "Redis Uptime", Value: redisUptime, Inline: true},
				{Name: "Redis Clients", Value: redisConnectedClients, Inline: true},
				{Name: "Redis Memory", Value: redisUsedMemoryHuman, Inline: true},
			},
		}

		// ElasticSearch
		if elasticStatusText != "" {
			statsEmbed.Fields = append(statsEmbed.Fields, &discordgo.MessageEmbedField{
				Name: "ElasticSearch Status", Value: elasticStatusText, Inline: true,
			})
			statsEmbed.Fields = append(statsEmbed.Fields, &discordgo.MessageEmbedField{
				Name: "ElasticSearch Storage", Value: elasticStatsText, Inline: true,
			})
			statsEmbed.Fields = append(statsEmbed.Fields, &discordgo.MessageEmbedField{
				Name: "ElasticSearch Process", Value: elasticProcessText, Inline: true,
			})
		}

		// Link
		statsEmbed.Fields = append(statsEmbed.Fields,
			&discordgo.MessageEmbedField{
				Name:   "Want more stats and awesome graphs?",
				Value:  "Visit my [stats dashboard](https://robyul.chat/statistics)",
				Inline: false,
			})

		_, err = helpers.SendEmbed(msg.ChannelID, statsEmbed)
		helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
	case "serverinfo":
		session.ChannelTyping(msg.ChannelID)

		args := strings.Fields(content)
		var err error
		var guild *discordgo.Guild
		if len(args) > 0 && helpers.IsRobyulMod(msg.Author.ID) {
			guild, err = helpers.GetGuild(args[0])
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok {
					if errD.Message.Code == 50001 {
						_, err = helpers.SendMessage(msg.ChannelID, "Unable to get information for this Server.")
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					}
				}
				helpers.Relax(err)
			}
		} else {
			currentChannel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)
			guild, err = helpers.GetGuild(currentChannel.GuildID)
			helpers.Relax(err)
		}

		usersCount := len(guild.Members)

		textChannels := 0
		voiceChannels := 0
		categoryChannels := 0
		for _, channel := range guild.Channels {
			if channel.Type == discordgo.ChannelTypeGuildVoice {
				voiceChannels++
			} else if channel.Type == discordgo.ChannelTypeGuildText {
				textChannels++
			} else if channel.Type == discordgo.ChannelTypeGuildCategory {
				categoryChannels++
			}
		}
		channelsText := fmt.Sprintf("%d/%d/%d/%d text/voice/category/total", textChannels, voiceChannels, categoryChannels, len(guild.Channels))
		online := 0
		for _, presence := range guild.Presences {
			if presence.Status == discordgo.StatusOnline || presence.Status == discordgo.StatusDoNotDisturb || presence.Status == discordgo.StatusIdle {
				online += 1
			}
		}

		createdAtTime := helpers.GetTimeFromSnowflake(guild.ID)

		owner, err := helpers.GetUser(guild.OwnerID)
		helpers.Relax(err)
		member, err := helpers.GetGuildMember(guild.ID, guild.OwnerID)
		helpers.Relax(err)
		ownerText := fmt.Sprintf("%s#%s\n`#%s`", owner.Username, owner.Discriminator, owner.ID)
		if member.Nick != "" {
			ownerText = fmt.Sprintf("%s#%s ~ %s\n`#%s`", owner.Username, owner.Discriminator, member.Nick, owner.ID)
		}

		emoteText := "_None_"
		if len(guild.Emojis) > 0 {
			animatedEmoji := 0
			for _, emote := range guild.Emojis {
				if emote.Animated {
					animatedEmoji++
				}
			}
			emoteText = fmt.Sprintf("(%d/%d/%d normal/animated/total) ",
				len(guild.Emojis)-animatedEmoji, animatedEmoji, len(guild.Emojis))
			for i, emote := range guild.Emojis {
				if i > 0 {
					emoteText += ", "
				}
				emoteText += fmt.Sprintf("`:%s:`", emote.Name)
				//emoteText += fmt.Sprintf("<:%s>", emote.APIName())
			}
		}

		numberOfRoles := 0
		for _, role := range guild.Roles {
			if role.Name != "@everyone" {
				numberOfRoles += 1
			}
		}

		totalMessagesText := "N/A"
		searchResponse, err := helpers.GuildFriendRequest(
			guild.ID,
			"GET",
			fmt.Sprintf("guilds/%s/messages/search", guild.ID),
		)
		if err != nil {
			if strings.Contains(err.Error(), "No friend on this server!") {
				totalMessagesText = ""
			} else {
				raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{
					"ChannelID":       msg.ChannelID,
					"Content":         msg.Content,
					"Timestamp":       string(msg.Timestamp),
					"TTS":             strconv.FormatBool(msg.Tts),
					"MentionEveryone": strconv.FormatBool(msg.MentionEveryone),
					"IsBot":           strconv.FormatBool(msg.Author.Bot),
				})
			}
		} else {
			searchResult, err := gabs.ParseJSON(searchResponse)
			if err == nil {
				if searchResult.Exists("total_results") {
					totalMessagesText = humanize.Commaf(searchResult.Path("total_results").Data().(float64)) + " Messages"
				}
			}
		}

		serverinfoEmbed := &discordgo.MessageEmbed{
			Color:       0x0FADED,
			Title:       guild.Name,
			Description: fmt.Sprintf("Since: %s. That's %s.", createdAtTime.Format(time.ANSIC), helpers.SinceInDaysText(createdAtTime)),
			Footer:      &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Server #%s", guild.ID)},
			Fields: []*discordgo.MessageEmbedField{
				{Name: "Region", Value: guild.Region, Inline: true},
				{Name: "Users", Value: fmt.Sprintf("%d/%d", online, usersCount), Inline: true},
				{Name: "Channels", Value: channelsText, Inline: true},
				{Name: "Roles", Value: strconv.Itoa(numberOfRoles), Inline: true},
				{Name: "Owner", Value: ownerText, Inline: true},
				{Name: "Emotes", Value: emoteText, Inline: false},
			},
		}

		if guild.Icon != "" {
			serverinfoEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: fmt.Sprintf("https://cdn.discordapp.com/icons/%s/%s.jpg", guild.ID, guild.Icon)}
			serverinfoEmbed.URL = fmt.Sprintf("https://cdn.discordapp.com/icons/%s/%s.png?size=2048", guild.ID, guild.Icon)
		}

		if totalMessagesText != "" {
			serverinfoEmbed.Fields = append(serverinfoEmbed.Fields, &discordgo.MessageEmbedField{Name: "Total Messages", Value: totalMessagesText, Inline: false})
		}

		_, err = helpers.SendEmbed(msg.ChannelID, serverinfoEmbed)
		helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
	case "userinfo":
		session.ChannelTyping(msg.ChannelID)
		targetUser, err := helpers.GetUser(msg.Author.ID)
		helpers.Relax(err)
		args := strings.Fields(content)
		if len(args) >= 1 && args[0] != "" {
			targetUser, err = helpers.GetUserFromMention(args[0])
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); (ok && errD.Message.Code == 10013) || strings.Contains(err.Error(), "User not found.") {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.stats.user-not-found"))
					helpers.Relax(err)
					return
				}
				helpers.Relax(err)
			}
		}

		currentChannel, err := helpers.GetChannel(msg.ChannelID)
		helpers.Relax(err)
		currentGuild, err := helpers.GetGuild(currentChannel.GuildID)
		helpers.Relax(err)
		targetMember, err := helpers.GetGuildMember(currentGuild.ID, targetUser.ID)
		if err != nil {
			if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == 10007 {
				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.stats.user-not-found"))
				helpers.Relax(err)
				return
			} else {
				helpers.Relax(err)
			}
		}

		status := ""
		game := ""
		gameUrl := ""
		for _, presence := range currentGuild.Presences {
			if presence.User.ID == targetUser.ID {
				status = string(presence.Status)
				switch status {
				case "dnd":
					status = "Do Not Disturb"
				}
				if presence.Game != nil {
					game = presence.Game.Name
					gameUrl = presence.Game.URL
				}
			}
		}
		nick := ""
		if targetMember.Nick != "" {
			nick = targetMember.Nick
		}
		description := ""
		if status != "" {
			description = fmt.Sprintf("**%s**", status)
			if game != "" {
				description = fmt.Sprintf("**%s** (Playing: **%s**)", status, game)
				if gameUrl != "" {
					description = fmt.Sprintf("**%s** (:mega: Streaming: **%s**)", status, game)
				}
			}
		}
		title := fmt.Sprintf("%s#%s", targetMember.User.Username, targetMember.User.Discriminator)
		if nick != "" {
			title = fmt.Sprintf("%s#%s ~ %s", targetMember.User.Username, targetMember.User.Discriminator, nick)
		}
		rolesText := "None"
		guildRoles, err := session.GuildRoles(currentGuild.ID)
		if err != nil {
			if err, ok := err.(*discordgo.RESTError); ok && err.Message != nil {
				if err.Message.Code == 50013 {
					rolesText = "Unable to gather roles"
				} else {
					helpers.Relax(err)
				}
			} else {
				helpers.Relax(err)
			}
		} else {
			isFirst := true
			slice.Sort(guildRoles, func(i, j int) bool {
				return guildRoles[i].Position > guildRoles[j].Position
			})
			for _, guildRole := range guildRoles {
				for _, userRole := range targetMember.Roles {
					if guildRole.ID == userRole {
						if isFirst == true {
							rolesText = fmt.Sprintf("<@&%s>", guildRole.ID)
						} else {
							rolesText += fmt.Sprintf(", <@&%s>", guildRole.ID)
						}
						isFirst = false
					}
				}
			}
		}

		joinedTime := helpers.GetTimeFromSnowflake(targetUser.ID)
		joinedServerTime, err := discordgo.Timestamp(targetMember.JoinedAt).Parse()
		if err != nil {

		}

		var allMembers []*discordgo.Member
		for _, u := range currentGuild.Members {
			allMembers = append(allMembers, u)
		}
		slice.Sort(allMembers[:], func(i, j int) bool {
			defer helpers.Recover()

			if allMembers[i].JoinedAt != "" && allMembers[j].JoinedAt != "" {
				iMemberTime, err := discordgo.Timestamp(allMembers[i].JoinedAt).Parse()
				helpers.Relax(err)
				jMemberTime, err := discordgo.Timestamp(allMembers[j].JoinedAt).Parse()
				helpers.Relax(err)
				return iMemberTime.Before(jMemberTime)
			} else {
				return false
			}
		})
		userNumber := -1
		for i, sortedMember := range allMembers[:] {
			if sortedMember.User.ID == targetUser.ID {
				userNumber = i + 1
				break
			}
		}

		totalMessagesText := "N/A"
		searchResponse, err := helpers.GuildFriendRequest(
			currentChannel.GuildID,
			"GET",
			fmt.Sprintf("guilds/%s/messages/search?author_id=%s", currentChannel.GuildID, targetUser.ID),
		)
		if err != nil {
			if strings.Contains(err.Error(), "No friend on this server!") {
				totalMessagesText = ""
			} else {
				raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{
					"ChannelID":       msg.ChannelID,
					"Content":         msg.Content,
					"Timestamp":       string(msg.Timestamp),
					"TTS":             strconv.FormatBool(msg.Tts),
					"MentionEveryone": strconv.FormatBool(msg.MentionEveryone),
					"IsBot":           strconv.FormatBool(msg.Author.Bot),
				})
			}
		} else {
			searchResult, err := gabs.ParseJSON(searchResponse)
			if err == nil {
				if searchResult.Exists("total_results") {
					totalMessagesText = humanize.Commaf(searchResult.Path("total_results").Data().(float64)) + " Messages"
				}
			}
		}

		var sinceStatusName, sinceStatusValue, lastMessageText string
		if cache.HasElastic() && !helpers.GuildSettingsGetCached(currentGuild.ID).ChatlogDisabled {
			queryString := "UserID:" + targetUser.ID + " AND NOT Status:\"\""
			termQuery := elastic.NewQueryStringQuery(queryString)
			searchResult, err := cache.GetElastic().Search().
				Index(models.ElasticIndexPresenceUpdates).
				Type("doc").
				Query(termQuery).
				Sort("CreatedAt", false).
				From(0).Size(1).
				Do(context.Background())
			if err == nil {
				if searchResult.TotalHits() > 0 {
					var ttyp models.ElasticPresenceUpdate
					for _, item := range searchResult.Each(reflect.TypeOf(ttyp)) {
						if presenceUpdate, ok := item.(models.ElasticPresenceUpdate); ok {
							sinceStatusName = presenceUpdate.Status
							sinceStatusValue = humanize.Time(presenceUpdate.CreatedAt)
							switch sinceStatusName {
							case "dnd":
								sinceStatusName = "Do Not Disturb"
							}
						}
					}
				}
			} else {
				if errE, ok := err.(*elastic.Error); ok {
					raven.CaptureError(fmt.Errorf("%#v", errE), map[string]string{
						"ChannelID":        msg.ChannelID,
						"Content":          msg.Content,
						"Timestamp":        string(msg.Timestamp),
						"TTS":              strconv.FormatBool(msg.Tts),
						"MentionEveryone":  strconv.FormatBool(msg.MentionEveryone),
						"IsBot":            strconv.FormatBool(msg.Author.Bot),
						"ElasticTermQuery": queryString,
					})
				} else {
					raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{
						"ChannelID":        msg.ChannelID,
						"Content":          msg.Content,
						"Timestamp":        string(msg.Timestamp),
						"TTS":              strconv.FormatBool(msg.Tts),
						"MentionEveryone":  strconv.FormatBool(msg.MentionEveryone),
						"IsBot":            strconv.FormatBool(msg.Author.Bot),
						"ElasticTermQuery": queryString,
					})
				}
			}

			queryString = "UserID:" + targetUser.ID + " AND GuildID:" + currentGuild.ID
			termQuery = elastic.NewQueryStringQuery(queryString)
			searchResult, err = cache.GetElastic().Search().
				Index(models.ElasticIndexMessages).
				Type("doc").
				Query(termQuery).
				Sort("CreatedAt", false).
				From(0).Size(1).
				Do(context.Background())
			if err == nil {
				if searchResult.TotalHits() > 0 {
					var ttyp models.ElasticMessage
					for _, item := range searchResult.Each(reflect.TypeOf(ttyp)) {
						if message, ok := item.(models.ElasticMessage); ok {
							lastMessageText = humanize.Time(message.CreatedAt)
						}
					}
				}
			} else {
				if errE, ok := err.(*elastic.Error); ok {
					raven.CaptureError(fmt.Errorf("%#v", errE), map[string]string{
						"ChannelID":        msg.ChannelID,
						"Content":          msg.Content,
						"Timestamp":        string(msg.Timestamp),
						"TTS":              strconv.FormatBool(msg.Tts),
						"MentionEveryone":  strconv.FormatBool(msg.MentionEveryone),
						"IsBot":            strconv.FormatBool(msg.Author.Bot),
						"ElasticTermQuery": queryString,
					})
				} else {
					raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{
						"ChannelID":        msg.ChannelID,
						"Content":          msg.Content,
						"Timestamp":        string(msg.Timestamp),
						"TTS":              strconv.FormatBool(msg.Tts),
						"MentionEveryone":  strconv.FormatBool(msg.MentionEveryone),
						"IsBot":            strconv.FormatBool(msg.Author.Bot),
						"ElasticTermQuery": queryString,
					})
				}
			}
		}

		userinfoEmbed := &discordgo.MessageEmbed{
			Color:  0x0FADED,
			Title:  title,
			Footer: &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Member #%d | User #%s", userNumber, targetUser.ID)},
			Fields: []*discordgo.MessageEmbedField{
				{Name: "Joined Discord on", Value: fmt.Sprintf("%s (%s)", joinedTime.Format(time.ANSIC), helpers.SinceInDaysText(joinedTime)), Inline: true},
				{Name: "Joined this server on", Value: fmt.Sprintf("%s (%s)", joinedServerTime.Format(time.ANSIC), helpers.SinceInDaysText(joinedServerTime)), Inline: true},
				{Name: "Roles", Value: rolesText, Inline: false},
				{Name: "Voice Stats",
					Value: fmt.Sprintf("use `%svoicestats @%s` to view the voice stats for this user",
						helpers.GetPrefixForServer(currentGuild.ID),
						fmt.Sprintf("%s#%s", targetMember.User.Username, targetMember.User.Discriminator)), Inline: false},
			},
		}
		if description != "" {
			userinfoEmbed.Description = description
		}

		if targetMember.User.Avatar != "" {
			userinfoEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: targetMember.User.AvatarURL("")}
			userinfoEmbed.URL = helpers.GetAvatarUrl(targetMember.User)
		}
		if gameUrl != "" {
			userinfoEmbed.URL = gameUrl
		}

		if sinceStatusName != "" && sinceStatusValue != "" {
			userinfoEmbed.Fields = append(userinfoEmbed.Fields, &discordgo.MessageEmbedField{Name: strings.Title(sinceStatusName) + " since", Value: sinceStatusValue, Inline: true})
		}
		if lastMessageText != "" {
			userinfoEmbed.Fields = append(userinfoEmbed.Fields, &discordgo.MessageEmbedField{Name: "Last Message", Value: lastMessageText, Inline: true})
		}

		if totalMessagesText != "" {
			userinfoEmbed.Fields = append(userinfoEmbed.Fields, &discordgo.MessageEmbedField{Name: "Total Messages", Value: totalMessagesText, Inline: true})
		}

		_, err = helpers.SendEmbed(msg.ChannelID, userinfoEmbed)
		helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
	case "channelinfo":
		session.ChannelTyping(msg.ChannelID)

		sourceChannel, err := helpers.GetChannel(msg.ChannelID)
		if err != nil {
			if errD, ok := err.(*discordgo.RESTError); ok {
				if errD.Message.Code == discordgo.ErrCodeMissingAccess {
					_, err = helpers.SendMessage(msg.ChannelID, "Unable to get information for this Channel.")
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}
			}
			helpers.Relax(err)
			return
		}

		args := strings.Fields(content)
		var channel *discordgo.Channel
		if len(args) > 0 {
			channel, err = helpers.GetGlobalChannelFromMention(args[0])
			if err != nil {
				if strings.Contains(err.Error(), "Channel not found.") {
					_, err = helpers.SendMessage(msg.ChannelID, "Channel not found.")
					helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
					return
				}
				if errD, ok := err.(*discordgo.RESTError); ok {
					if errD.Message.Code == discordgo.ErrCodeMissingAccess {
						_, err = helpers.SendMessage(msg.ChannelID, "Unable to get information for this Channel.")
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					}
				}
				helpers.Relax(err)
				return
			}
			channel, err = helpers.GetChannel(channel.ID)
			helpers.Relax(err)
			if channel.GuildID != sourceChannel.GuildID && !helpers.IsRobyulMod(msg.Author.ID) {
				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				return
			}
		} else {
			channel, err = helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)
		}

		guild, err := helpers.GetGuild(channel.GuildID)
		helpers.Relax(err)

		createdAtTime := helpers.GetTimeFromSnowflake(channel.ID)

		totalMessagesText := "N/A"
		searchResponse, err := helpers.GuildFriendRequest(
			guild.ID,
			"GET",
			fmt.Sprintf("guilds/%s/messages/search?channel_id=%s", guild.ID, channel.ID),
		)
		if err != nil {
			if strings.Contains(err.Error(), "No friend on this server!") {
				totalMessagesText = ""
			} else {
				raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{
					"ChannelID":       msg.ChannelID,
					"Content":         msg.Content,
					"Timestamp":       string(msg.Timestamp),
					"TTS":             strconv.FormatBool(msg.Tts),
					"MentionEveryone": strconv.FormatBool(msg.MentionEveryone),
					"IsBot":           strconv.FormatBool(msg.Author.Bot),
				})
			}
		} else {
			searchResult, err := gabs.ParseJSON(searchResponse)
			if err == nil {
				if searchResult.Exists("total_results") {
					totalMessagesText = humanize.Commaf(searchResult.Path("total_results").Data().(float64)) + " Messages"
				}
			}
		}

		nsfwText := "No"
		if channel.NSFW {
			nsfwText = "Yes"
		}

		channelTypeText := "N/A"
		switch channel.Type {
		case discordgo.ChannelTypeGuildCategory:
			channelTypeText = "Category"
			break
		case discordgo.ChannelTypeGuildText:
			channelTypeText = "Text"
			break
		case discordgo.ChannelTypeGuildVoice:
			channelTypeText = "Voice"
			break
		}

		topicText := "_None_"
		if channel.Topic != "" {
			topicText = channel.Topic
		}

		var parentChannelTitleText string
		var parentChannelFooterText string
		if (channel.Type == discordgo.ChannelTypeGuildText || channel.Type == discordgo.ChannelTypeGuildVoice) &&
			channel.ParentID != "" {
			parentChannel, err := helpers.GetChannel(channel.ParentID)
			if err != nil {
				parentChannel = new(discordgo.Channel)
				parentChannel.ID = "N/A"
				parentChannel.Name = "N/A"
			}
			parentChannelTitleText = parentChannel.Name + " / "
			parentChannelFooterText = "| Parent Channel #" + parentChannel.ID + " "
		}

		channelinfoEmbed := &discordgo.MessageEmbed{
			Color:       0x0FADED,
			Title:       channel.Name + " / " + parentChannelTitleText + guild.Name,
			Description: fmt.Sprintf("Since: %s. That's %s.", createdAtTime.Format(time.ANSIC), helpers.SinceInDaysText(createdAtTime)),
			Footer:      &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Channel #%s %s| Server #%s", channel.ID, parentChannelFooterText, guild.ID)},
			Fields: []*discordgo.MessageEmbedField{
				{Name: "Topic", Value: topicText, Inline: false},
				{Name: "Type", Value: channelTypeText, Inline: true},
				{Name: "NSFW", Value: nsfwText, Inline: true},
			},
		}

		if totalMessagesText != "" {
			channelinfoEmbed.Fields = append(channelinfoEmbed.Fields, &discordgo.MessageEmbedField{Name: "Total Messages", Value: totalMessagesText, Inline: false})
		}

		_, err = helpers.SendEmbed(msg.ChannelID, channelinfoEmbed)
		helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
	case "voicestats": // [p]voicestats <user> or [p]voicestats top // @TODO: sort by time connected
		session.ChannelTyping(msg.ChannelID)
		targetUser, err := helpers.GetUser(msg.Author.ID)
		helpers.Relax(err)
		args := strings.Fields(content)

		channel, err := helpers.GetChannel(msg.ChannelID)
		helpers.Relax(err)

		channelsAgg := elastic.NewTermsAggregation().
			Field("ChannelID.keyword").
			Order("_count", false).
			Size(100)

		usersAgg := elastic.NewTermsAggregation().
			Field("UserID.keyword").
			Order("totalDuration", false).
			Size(5)

		durationSumAgg := elastic.NewSumAggregation().Field("DurationSeconds")

		durationByChannelsAgg := channelsAgg.SubAggregation("totalDuration", durationSumAgg)

		durationbyChannelsByUsersAgg := channelsAgg.SubAggregation("users",
			usersAgg.SubAggregation("totalDuration", durationSumAgg))

		if len(args) >= 1 && args[0] != "" {
			switch args[0] {
			case "leaderboard", "top": // [p]voicestats top
				termQuery := elastic.NewQueryStringQuery("GuildID:" + channel.GuildID)
				searchResult, err := cache.GetElastic().Search().
					Index(models.ElasticIndexVoiceSessions).
					Type("doc").
					Query(termQuery).
					Aggregation("channels", durationbyChannelsByUsersAgg).
					Size(0).
					Do(context.Background())
				helpers.Relax(err)

				channelUsersDurations := make(map[string]map[string]int64, 0)

				if agg, found := searchResult.Aggregations.Terms("channels"); found {
					for _, bucket := range agg.Buckets {
						channelUsersDurations[bucket.Key.(string)] = make(map[string]int64, 0)
						if subAgg, found := bucket.Aggregations.Terms("users"); found {
							for _, subBucket := range subAgg.Buckets {
								if subSubAgg, found := subBucket.Aggregations.Sum("totalDuration"); found {
									channelUsersDurations[bucket.Key.(string)][subBucket.Key.(string)] = int64(*subSubAgg.Value)
								}
							}
						}
					}
				}

				if len(channelUsersDurations) <= 0 {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.stats.voicestats-toplist-no-entries"))
					helpers.Relax(err)
					return
				}

				var totalDurationSeconds int64
				for _, voiceChannelData := range channelUsersDurations {
					for _, voiceChannelUserDuration := range voiceChannelData {
						totalDurationSeconds += voiceChannelUserDuration
					}
				}

				if totalDurationSeconds <= 0 {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.stats.voicestats-toplist-no-entries"))
					helpers.Relax(err)
					return
				}

				totalVoiceStatsEmbed := &discordgo.MessageEmbed{
					Color: 0x0FADED,
					Title: helpers.GetText("plugins.stats.voicestats-toplist-embed-title"),
					Description: fmt.Sprintf("Total time connected by all users: **%s**",
						helpers.HumanizedTimesSinceText(time.Now().UTC().Add(-1*(time.Second*time.Duration(totalDurationSeconds))))),
					Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.stats.voicestats-embed-footer")},
					Fields: []*discordgo.MessageEmbedField{},
				}

				guild, err := session.State.Guild(channel.GuildID)
				helpers.Relax(err)

				slice.Sort(guild.Channels, func(i, j int) bool {
					return guild.Channels[i].Position < guild.Channels[j].Position
				})

				var i int
				var channelToplistText string
				for _, guildChannel := range guild.Channels {
					userDurations, ok := channelUsersDurations[guildChannel.ID]
					if !ok {
						continue
					}

					durations := s.rankByDuration(userDurations)

					channelToplistText = ""
					i = 0
					for _, voiceChannelUserDurationData := range durations {
						channelToplistText += fmt.Sprintf("#%d: <@%s>: %s\n",
							i+1,
							voiceChannelUserDurationData.Key,
							helpers.HumanizedTimesSinceText(time.Now().Add(-1*(time.Second*time.Duration(voiceChannelUserDurationData.Value)))))
						i++
						if i >= 5 {
							break
						}
					}

					totalVoiceStatsEmbed.Fields = append(totalVoiceStatsEmbed.Fields, &discordgo.MessageEmbedField{
						Name:   fmt.Sprintf("Top 5 users connected to #%s", guildChannel.Name),
						Value:  channelToplistText,
						Inline: false,
					})
				}

				_, err = helpers.SendEmbed(msg.ChannelID, totalVoiceStatsEmbed)
				helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
				return
			}
			targetUser, err = helpers.GetUserFromMention(args[0])
			if err != nil || targetUser.ID == "" {
				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				helpers.Relax(err)
				return
			}
		}

		currentConnectionText := "Currently not connected to any Voice Channel on this server."
		for _, voiceSessionStart := range VoiceSessionStarts {
			if voiceSessionStart.GuildID == channel.GuildID && voiceSessionStart.UserID == targetUser.ID {
				currentVoiceChannel, err := helpers.GetChannel(voiceSessionStart.ChannelID)
				if err == nil {
					currentConnectionText = fmt.Sprintf("Connected to **<#%s>** since **%s**",
						currentVoiceChannel.ID,
						helpers.HumanizedTimesSinceText(voiceSessionStart.JoinTime))
				}
			}
		}

		title := fmt.Sprintf("Voice Stats for %s", targetUser.Username)

		termQuery := elastic.NewQueryStringQuery("GuildID:" + channel.GuildID + " AND UserID:" + targetUser.ID)
		searchResult, err := cache.GetElastic().Search().
			Index(models.ElasticIndexVoiceSessions).
			Type("doc").
			Query(termQuery).
			Aggregation("channels", durationByChannelsAgg).
			Size(0).
			Do(context.Background())
		helpers.Relax(err)

		channelDurations := make(map[string]int64, 0)

		if agg, found := searchResult.Aggregations.Terms("channels"); found {
			for _, bucket := range agg.Buckets {
				if subAgg, found := bucket.Aggregations.Sum("totalDuration"); found {
					channelDurations[bucket.Key.(string)] = int64(*subAgg.Value)
				}
			}
		}

		voicestatsEmbed := &discordgo.MessageEmbed{
			Color:       0x0FADED,
			Title:       title,
			Description: currentConnectionText,
			Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.stats.voicestats-embed-footer")},
			Fields:      []*discordgo.MessageEmbedField{},
		}

		guild, err := session.State.Guild(channel.GuildID)
		helpers.Relax(err)

		slice.Sort(guild.Channels, func(i, j int) bool {
			return guild.Channels[i].Position < guild.Channels[j].Position
		})

		for _, guildChannel := range guild.Channels {
			duration, ok := channelDurations[guildChannel.ID]
			if !ok {
				continue
			}
			voicestatsEmbed.Fields = append(voicestatsEmbed.Fields, &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("Total duration connected to #%s", guildChannel.Name),
				Value:  fmt.Sprintf("%s", helpers.HumanizedTimesSinceText(time.Now().UTC().Add(time.Second*time.Duration(duration)))),
				Inline: false,
			})
		}

		_, err = helpers.SendEmbed(msg.ChannelID, voicestatsEmbed)
		helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
	case "emotes", "emojis", "emoji": // [p]emotes
		session.ChannelTyping(msg.ChannelID)
		channel, err := helpers.GetChannel(msg.ChannelID)
		helpers.Relax(err)
		guild, err := helpers.GetGuild(channel.GuildID)
		helpers.Relax(err)

		args := strings.Fields(content)
		if len(args) > 0 && helpers.IsRobyulMod(msg.Author.ID) {
			guild, err = helpers.GetGuild(args[0])
			if err != nil {
				if errD, ok := err.(*discordgo.RESTError); ok {
					if errD.Message.Code == discordgo.ErrCodeMissingAccess {
						_, err = helpers.SendMessage(msg.ChannelID, "Unable to get information for this Server.")
						helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
						return
					}
				}
				helpers.Relax(err)
			}
		}

		if len(guild.Emojis) <= 0 {
			_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.stats.no-emotes"))
			helpers.Relax(err)
			return
		}

		numberOfPages := int(math.Ceil(float64(len(guild.Emojis)) / float64(12)))
		footerAdditionalText := ""
		numberOfAnimatedEmojis := 0
		for _, emoji := range guild.Emojis {
			if emoji.Animated {
				numberOfAnimatedEmojis++
			}
		}
		if numberOfAnimatedEmojis > 0 {
			footerAdditionalText += fmt.Sprintf(" %d out of these are animated.", numberOfAnimatedEmojis)
		}
		if numberOfPages > 1 {
			footerAdditionalText += " Click on the numbers below to change the page."
		}

		reactionEmbed := &discordgo.MessageEmbed{
			Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.stats.reaction-embed-footer", len(guild.Emojis)) + footerAdditionalText},
		}

		s.setEmbedEmojiPage(reactionEmbed, msg.Author, guild, 1, numberOfPages)
		reactionEmbedMessages, err := helpers.SendEmbed(msg.ChannelID, reactionEmbed)
		helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)

		if len(reactionEmbedMessages) <= 0 {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.errors.generic-nomessage"))
			return
		}
		reactionEmbedMessage := reactionEmbedMessages[0]

		reactionsAdded := 0
		if numberOfPages > 1 {
			go func() {
				defer helpers.Recover()

				for {
					err = session.MessageReactionAdd(msg.ChannelID, reactionEmbedMessage.ID, emojis.From(strconv.Itoa(reactionsAdded+1)))
					helpers.Relax(err)
					reactionsAdded++
					if reactionsAdded >= numberOfPages {
						break
					}
				}
			}()
		}

		closeHandler := session.AddHandler(func(session *discordgo.Session, reaction *discordgo.MessageReactionAdd) {
			defer helpers.Recover()
			if reaction.MessageID == reactionEmbedMessage.ID {
				if reaction.UserID == session.State.User.ID {
					return
				}

				if reaction.UserID == msg.Author.ID {
					newPageN := emojis.ToNumber(reaction.Emoji.Name)
					if newPageN >= 1 && newPageN <= numberOfPages {
						s.setEmbedEmojiPage(reactionEmbed, msg.Author, guild, newPageN, numberOfPages)
						reactionEmbedMessage, err = helpers.EditEmbed(msg.ChannelID, reactionEmbedMessage.ID, reactionEmbed)
						helpers.Relax(err)
					}
				}
				err = session.MessageReactionRemove(reaction.ChannelID, reaction.MessageID, reaction.Emoji.Name, reaction.UserID)
				if errD, ok := err.(*discordgo.RESTError); !ok || (errD.Message.Code != discordgo.ErrCodeUnknownMessage && errD.Message.Code != discordgo.ErrCodeMissingPermissions) {
					helpers.RelaxLog(err)
				}
			}
		})
		time.Sleep(3 * time.Minute)
		closeHandler()
		reactionsRemoved := 0
		if numberOfPages > 1 {
			for {
				err = session.MessageReactionRemove(msg.ChannelID, reactionEmbedMessage.ID, emojis.From(strconv.Itoa(reactionsRemoved+1)), session.State.User.ID)
				if errD, ok := err.(*discordgo.RESTError); !ok || (errD.Message.Code != discordgo.ErrCodeUnknownMessage && errD.Message.Code != discordgo.ErrCodeMissingPermissions) {
					helpers.RelaxLog(err)
				}
				reactionsRemoved++
				if reactionsRemoved >= numberOfPages {
					break
				}
			}

		}

		return
	case "memberlist", "members": // [p]memberlist [<page #>]
		session.ChannelTyping(msg.ChannelID)
		channel, err := helpers.GetChannel(msg.ChannelID)
		helpers.Relax(err)

		guild, err := helpers.GetGuild(channel.GuildID)
		helpers.Relax(err)
		var role discordgo.Role

		args := strings.Fields(content)
		if len(args) >= 1 {
			if helpers.IsBotAdmin(msg.Author.ID) {
				otherGuild, err := helpers.GetGuild(args[0])
				if err == nil && otherGuild != nil && otherGuild.ID != "" {
					guild = otherGuild
				}
			}

			// TODO: implement channel
			/*
				otherChannel, err := helpers.GetChannelFromMention(msg, args[len(args)-1])
				if err == nil && otherChannel != nil && otherChannel.ID != "" {
					// check if user can access channel
					channel = otherChannel
				}*/

			for _, scanRole := range guild.Roles {
				if scanRole.ID == args[len(args)-1] || strings.ToLower(scanRole.Name) == strings.ToLower(args[len(args)-1]) {
					role = *scanRole
				}
			}
		}

		allMembers := guild.Members
		kind := "guild"
		var kindTitle string
		if role.ID != "" {
			kind = "role"
			kindTitle = role.Name
			allMembers = make([]*discordgo.Member, 0)
			for _, member := range guild.Members {
				for _, memberRole := range member.Roles {
					if memberRole == role.ID {
						allMembers = append(allMembers, member)
					}
				}
			}
		}

		if len(allMembers) <= 0 {
			allMembers = guild.Members
			kind = "guild"
			kindTitle = ""
		}

		slice.Sort(allMembers[:], func(i, j int) bool {
			if allMembers[i].JoinedAt == "" || allMembers[j].JoinedAt == "" {
				return false
			}

			iMemberTime, err := discordgo.Timestamp(allMembers[i].JoinedAt).Parse()
			helpers.Relax(err)
			jMemberTime, err := discordgo.Timestamp(allMembers[j].JoinedAt).Parse()
			helpers.Relax(err)
			return iMemberTime.Before(jMemberTime)
		})

		numberOfPages := int(math.Ceil(float64(len(allMembers)) / float64(10)))
		footerAdditionalText := ""
		if numberOfPages > 1 {
			footerAdditionalText += " Click on the arrows below to change the page."
		}

		currentPage := 1
		if len(args) > 0 {
			currentPage, err = strconv.Atoi(args[0])
			if err != nil {
				currentPage = 1
			}
		}
		if currentPage > numberOfPages {
			currentPage = 1
		}

		memberlistEmbed := &discordgo.MessageEmbed{
			Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.stats.memberlist-embed-footer", humanize.Comma(int64(len(allMembers)))) + footerAdditionalText},
		}

		s.setEmbedMemberlistPage(memberlistEmbed, msg.Author, guild, allMembers, currentPage, numberOfPages, kind, kindTitle)
		memberlistEmbedMessages, err := helpers.SendEmbed(msg.ChannelID, memberlistEmbed)
		helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)

		if len(memberlistEmbedMessages) <= 0 {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.errors.generic-nomessage"))
			return
		}
		memberlistEmbedMessage := memberlistEmbedMessages[0]

		if numberOfPages > 1 {
			err = session.MessageReactionAdd(msg.ChannelID, memberlistEmbedMessage.ID, "⬅")
			helpers.Relax(err)
			err = session.MessageReactionAdd(msg.ChannelID, memberlistEmbedMessage.ID, "➡")
			helpers.Relax(err)
		}

		closeHandler := session.AddHandler(func(session *discordgo.Session, reaction *discordgo.MessageReactionAdd) {
			defer helpers.Recover()

			if reaction.MessageID == memberlistEmbedMessage.ID {
				if reaction.UserID == session.State.User.ID {
					return
				}

				if reaction.UserID == msg.Author.ID {
					if reaction.Emoji.Name == "➡" {
						if currentPage+1 <= numberOfPages {
							currentPage += 1
							s.setEmbedMemberlistPage(memberlistEmbed, msg.Author, guild, allMembers, currentPage, numberOfPages, kind, kindTitle)
							memberlistEmbedMessage, err = helpers.EditEmbed(msg.ChannelID, memberlistEmbedMessage.ID, memberlistEmbed)
							helpers.Relax(err)
						}
					} else if reaction.Emoji.Name == "⬅" {
						if currentPage-1 >= 1 {
							currentPage -= 1
							s.setEmbedMemberlistPage(memberlistEmbed, msg.Author, guild, allMembers, currentPage, numberOfPages, kind, kindTitle)
							memberlistEmbedMessage, err = helpers.EditEmbed(msg.ChannelID, memberlistEmbedMessage.ID, memberlistEmbed)
							helpers.Relax(err)
						}
					}
				}
				err = session.MessageReactionRemove(reaction.ChannelID, reaction.MessageID, reaction.Emoji.Name, reaction.UserID)
				if errD, ok := err.(*discordgo.RESTError); !ok ||
					(errD.Message.Code != discordgo.ErrCodeUnknownMessage &&
						errD.Message.Code != discordgo.ErrCodeMissingPermissions &&
						errD.Message.Code != discordgo.ErrCodeUnknownEmoji) {
					helpers.RelaxLog(err)
				}
			}
		})
		time.Sleep(3 * time.Minute)
		closeHandler()
		if numberOfPages > 1 {
			err = session.MessageReactionRemove(msg.ChannelID, memberlistEmbedMessage.ID, "⬅", session.State.User.ID)
			helpers.Relax(err)
			err = session.MessageReactionRemove(msg.ChannelID, memberlistEmbedMessage.ID, "➡", session.State.User.ID)
			helpers.Relax(err)
		}

		return
	case "invite":
		session.ChannelTyping(msg.ChannelID)
		args := strings.Fields(content)

		if len(args) < 1 {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
			return
		}

		inviteCode := args[0]
		inviteCode = strings.Replace(inviteCode, "https://", "", -1)
		inviteCode = strings.Replace(inviteCode, "http://", "", -1)
		inviteCode = strings.Replace(inviteCode, "discord.gg/", "", -1)
		inviteCode = strings.Replace(inviteCode, "invite/", "", -1)

		type InviteWithCounts struct {
			Guild                    *discordgo.Guild    `json:"guild"`
			Channel                  *discordgo.Channel  `json:"channel"`
			Inviter                  *discordgo.User     `json:"inviter"`
			Code                     string              `json:"code"`
			CreatedAt                discordgo.Timestamp `json:"created_at"`
			MaxAge                   int                 `json:"max_age"`
			Uses                     int                 `json:"uses"`
			MaxUses                  int                 `json:"max_uses"`
			XkcdPass                 string              `json:"xkcdpass"`
			Revoked                  bool                `json:"revoked"`
			Temporary                bool                `json:"temporary"`
			ApproximateMemberCount   int                 `json:"approximate_member_count"`
			ApproximatePresenceCount int                 `json:"approximate_presence_count"`
		}

		respBody, err := session.RequestWithBucketID("GET", discordgo.EndpointInvite(inviteCode)+"?with_counts=true", nil, discordgo.EndpointInvite(""))
		if err != nil {
			if errD, ok := err.(*discordgo.RESTError); ok && errD.Message.Code == discordgo.ErrCodeUnknownInvite {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.stats.unknown-invite"))
				return
			}
		}
		helpers.Relax(err)

		var invite InviteWithCounts

		err = json.Unmarshal(respBody, &invite)
		helpers.Relax(err)

		guild, err := helpers.GetGuild(invite.Guild.ID)
		if err == nil {
			invite.Guild.Channels = guild.Channels
			invite.Guild.Members = guild.Members
			guildInvites, err := session.GuildInvites(invite.Guild.ID)
			if err == nil {
				for _, guildInvite := range guildInvites {
					if guildInvite.Code == invite.Code {
						invite.Uses = guildInvite.Uses
						invite.MaxAge = guildInvite.MaxAge
						invite.MaxUses = guildInvite.MaxUses
						invite.Revoked = guildInvite.Revoked
						invite.CreatedAt = guildInvite.CreatedAt
					}
				}
			}
		}

		maxAgeText := "never"
		if invite.MaxAge > 0 {
			maxAgeText = strconv.Itoa((invite.MaxAge/60)/60) + " hours"
		}
		maxUsesText := "unlimited times"
		if invite.MaxUses > 0 {
			maxUsesText = fmt.Sprintf("%d times", invite.MaxUses)
		}
		revokedText := "not revoked"
		if invite.Revoked {
			revokedText = "is revoked"
		}
		createdAt, _ := invite.CreatedAt.Parse()

		numberOfMembers := len(invite.Guild.Members)
		if numberOfMembers <= 0 {
			numberOfMembers = invite.ApproximateMemberCount
		}

		inviterUsername := "N/A"
		inviterID := "N/A"
		if invite.Inviter != nil {
			inviterUsername = invite.Inviter.Username + "#" + invite.Inviter.Discriminator
			inviterID = invite.Inviter.ID
		}

		inviteEmbed := &discordgo.MessageEmbed{
			Title:     "Invite for " + invite.Guild.Name,
			URL:       "https://discord.gg/" + invite.Code,
			Thumbnail: &discordgo.MessageEmbedThumbnail{URL: invite.Guild.Icon},
			Footer:    &discordgo.MessageEmbedFooter{Text: "Server #" + invite.Guild.ID},

			Fields: []*discordgo.MessageEmbedField{
				{Name: "Link", Value: "https://discord.gg/" + invite.Code, Inline: true},
				{Name: "Channel", Value: fmt.Sprintf("#%s (`#%s`)", invite.Channel.Name, invite.Channel.ID), Inline: true},
				{Name: "Members", Value: humanize.Comma(int64(numberOfMembers)), Inline: true},
				{Name: "Times Used", Value: humanize.Comma(int64(invite.Uses)), Inline: true},
				{Name: "Usage Limit", Value: maxUsesText, Inline: true},
				{Name: "Expires", Value: maxAgeText, Inline: true},
				{Name: "Revoked", Value: revokedText, Inline: true},
				{Name: "Inviter", Value: fmt.Sprintf("%s (`#%s`)", inviterUsername, inviterID), Inline: true},
				{Name: "Created At", Value: humanize.Time(createdAt), Inline: true},
			},
		}

		if invite.Guild.Icon != "" {
			inviteEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{
				URL: fmt.Sprintf("https://cdn.discordapp.com/icons/%s/%s.jpg", invite.Guild.ID, invite.Guild.Icon),
			}
		}

		if helpers.IsRobyulMod(msg.Author.ID) && !helpers.GuildIsOnWhitelist(invite.Guild.ID) {
			inviteEmbed.Fields = append(inviteEmbed.Fields, &discordgo.MessageEmbedField{Name: "Whitelisted", Value: ":warning: Guild is not whitelisted!", Inline: false})
		}

		_, err = helpers.SendEmbed(msg.ChannelID, inviteEmbed)
		helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
		return
	case "serverindex": // [p]serverindex [<excluded channel> <excluded channel ...>]
		session.ChannelTyping(msg.ChannelID)
		channel, err := helpers.GetChannel(msg.ChannelID)
		helpers.Relax(err)
		guild, err := helpers.GetGuild(channel.GuildID)
		helpers.Relax(err)

		hiddenChannels := make([]string, 0)
		for _, fieldChannelTag := range strings.Fields(content) {
			fieldChannel, err := helpers.GetChannelFromMention(msg, fieldChannelTag)
			if err == nil {
				hiddenChannels = append(hiddenChannels, fieldChannel.ID)
			}
		}

		channels := guild.Channels
		sort.Slice(channels, func(i, j int) bool { return guild.Channels[i].Position < guild.Channels[j].Position })

		type FoundLinks struct {
			ChannelID string
			Links     int
		}

		countedLinks := make([]FoundLinks, 0)
		var links int
	NextChannel:
		for _, guildChannel := range channels {
			for _, hiddenChannel := range hiddenChannels {
				if guildChannel.ID == hiddenChannel {
					continue NextChannel
				}
			}

			messages, err := session.ChannelMessages(guildChannel.ID, 100, "", "", "")
			links = 0
			for _, message := range messages {
				links += strings.Count(message.Content, "discord.gg")
			}
			if err == nil {
				countedLinks = append(countedLinks, FoundLinks{ChannelID: guildChannel.ID, Links: links})
			}
		}

		var totalLinks int
		resultText := "__**Server Index Stats**__\n"
		for _, foundLink := range countedLinks {
			if foundLink.Links > 0 {
				resultText += fmt.Sprintf("<#%s>: %d invites\n", foundLink.ChannelID, foundLink.Links)
				totalLinks += foundLink.Links
			}
		}
		resultText += fmt.Sprintf("_Found **%d invites** in total._", totalLinks)

		for _, page := range helpers.Pagify(resultText, "\n") {
			_, err := helpers.SendMessage(msg.ChannelID, page)
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
		}
		return
	case "roles", "rolelist":
		session.ChannelTyping(msg.ChannelID)
		channel, err := helpers.GetChannel(msg.ChannelID)
		helpers.Relax(err)

		guild, err := helpers.GetGuild(channel.GuildID)
		helpers.Relax(err)

		args := strings.Fields(content)
		if len(args) >= 1 {
			if helpers.IsBotAdmin(msg.Author.ID) {
				otherGuild, err := helpers.GetGuild(args[len(args)-1])
				if err == nil && otherGuild != nil && otherGuild.ID != "" {
					guild = otherGuild
				}
			}
		}

		allRoles := guild.Roles

		if len(allRoles) <= 0 {
			_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.stats.rolelist-none"))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			return
		}

		slice.Sort(allRoles, func(i, j int) bool {
			return allRoles[i].Position > allRoles[j].Position
		})

		numberOfPages := int(math.Ceil(float64(len(allRoles)) / float64(10)))
		footerAdditionalText := ""
		if numberOfPages > 1 {
			footerAdditionalText += " Click on the arrows below to change the page."
		}

		currentPage := 1
		if len(args) > 0 {
			currentPage, err = strconv.Atoi(args[0])
			if err != nil {
				currentPage = 1
			}
		}
		if currentPage > numberOfPages {
			currentPage = 1
		}

		rolelistEmbed := &discordgo.MessageEmbed{
			Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.stats.rolelist-embed-footer", humanize.Comma(int64(len(allRoles)))) + footerAdditionalText},
		}

		s.setEmbedRolelistPage(rolelistEmbed, msg.Author, guild, allRoles, currentPage, numberOfPages)
		rolelistEmbedMessages, err := helpers.SendEmbed(msg.ChannelID, rolelistEmbed)
		helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)

		if len(rolelistEmbedMessages) <= 0 {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.errors.generic-nomessage"))
			return
		}
		rolelistEmbedMessage := rolelistEmbedMessages[0]

		if numberOfPages > 1 {
			err = session.MessageReactionAdd(msg.ChannelID, rolelistEmbedMessage.ID, "⬅")
			helpers.Relax(err)
			err = session.MessageReactionAdd(msg.ChannelID, rolelistEmbedMessage.ID, "➡")
			helpers.Relax(err)
		}

		closeHandler := session.AddHandler(func(session *discordgo.Session, reaction *discordgo.MessageReactionAdd) {
			defer helpers.Recover()

			if reaction.MessageID == rolelistEmbedMessage.ID {
				if reaction.UserID == session.State.User.ID {
					return
				}

				if reaction.UserID == msg.Author.ID {
					if reaction.Emoji.Name == "➡" {
						if currentPage+1 <= numberOfPages {
							currentPage += 1
							s.setEmbedRolelistPage(rolelistEmbed, msg.Author, guild, allRoles, currentPage, numberOfPages)
							rolelistEmbedMessage, err = helpers.EditEmbed(msg.ChannelID, rolelistEmbedMessage.ID, rolelistEmbed)
							helpers.Relax(err)
						}
					} else if reaction.Emoji.Name == "⬅" {
						if currentPage-1 >= 1 {
							currentPage -= 1
							s.setEmbedRolelistPage(rolelistEmbed, msg.Author, guild, allRoles, currentPage, numberOfPages)
							rolelistEmbedMessage, err = helpers.EditEmbed(msg.ChannelID, rolelistEmbedMessage.ID, rolelistEmbed)
							helpers.Relax(err)
						}
					}
				}
				err = session.MessageReactionRemove(reaction.ChannelID, reaction.MessageID, reaction.Emoji.Name, reaction.UserID)
				if errD, ok := err.(*discordgo.RESTError); !ok || (errD.Message.Code != discordgo.ErrCodeUnknownMessage && errD.Message.Code != discordgo.ErrCodeMissingPermissions) {
					helpers.RelaxLog(err)
				}
			}
		})
		time.Sleep(3 * time.Minute)
		closeHandler()
		if numberOfPages > 1 {
			err = session.MessageReactionRemove(msg.ChannelID, rolelistEmbedMessage.ID, "⬅", session.State.User.ID)
			if errD, ok := err.(*discordgo.RESTError); !ok || errD.Message.Code != discordgo.ErrCodeUnknownMessage {
				helpers.RelaxLog(err)
			}
			err = session.MessageReactionRemove(msg.ChannelID, rolelistEmbedMessage.ID, "➡", session.State.User.ID)
			if errD, ok := err.(*discordgo.RESTError); !ok || errD.Message.Code != discordgo.ErrCodeUnknownMessage {
				helpers.RelaxLog(err)
			}
		}

		return
	case "channels", "channellist":
		session.ChannelTyping(msg.ChannelID)
		channel, err := helpers.GetChannel(msg.ChannelID)
		helpers.Relax(err)

		guild, err := helpers.GetGuild(channel.GuildID)
		helpers.Relax(err)

		args := strings.Fields(content)
		if len(args) >= 1 {
			if helpers.IsBotAdmin(msg.Author.ID) {
				otherGuild, err := helpers.GetGuild(args[len(args)-1])
				if err == nil && otherGuild != nil && otherGuild.ID != "" {
					guild = otherGuild
				}
			}
		}

		allChannels := guild.Channels

		if len(allChannels) <= 0 {
			_, err = helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.stats.channellist-none"))
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
			return
		}

		allChannelsByCategory := make(map[*discordgo.Channel][]*discordgo.Channel, 0)
		allChannelCategories := make([]*discordgo.Channel, 0)

		for _, foundChannel := range allChannels {
			if foundChannel.Type == discordgo.ChannelTypeGuildCategory {
				allChannelCategories = append(allChannelCategories, foundChannel)
			}

			if foundChannel.ParentID == "" {
				if foundChannel.Type != discordgo.ChannelTypeGuildCategory {
					if _, ok := allChannelsByCategory[nil]; !ok {
						allChannelsByCategory[nil] = make([]*discordgo.Channel, 0)
					}
					allChannelsByCategory[nil] = append(allChannelsByCategory[nil], foundChannel)
				}
			} else {
				parentChannel, err := helpers.GetChannel(foundChannel.ParentID)
				if err != nil {
					parentChannel = &discordgo.Channel{ID: foundChannel.ParentID, Name: "N/A"}
				}

				if _, ok := allChannelsByCategory[parentChannel]; !ok {
					allChannelsByCategory[parentChannel] = make([]*discordgo.Channel, 0)
				}
				allChannelsByCategory[parentChannel] = append(allChannelsByCategory[parentChannel], foundChannel)
			}
		}

		slice.Sort(allChannelCategories, func(i, j int) bool {
			return allChannelCategories[i].Position < allChannelCategories[j].Position
		})
		allChannelCategories = append([]*discordgo.Channel{nil}, allChannelCategories...)

		allChannels = make([]*discordgo.Channel, 0)

		var foundChannels []*discordgo.Channel
		var ok bool
		for _, foundCategory := range allChannelCategories {
			if foundChannels, ok = allChannelsByCategory[foundCategory]; !ok {
				continue
			}

			if foundCategory != nil {
				allChannels = append(allChannels, foundCategory)
			}

			slice.Sort(foundChannels, func(i, j int) bool {
				return foundChannels[i].Position < foundChannels[j].Position
			})
			for _, foundChannel := range foundChannels {
				allChannels = append(allChannels, foundChannel)
			}
		}

		numberOfPages := int(math.Ceil(float64(len(allChannels)) / float64(10)))
		footerAdditionalText := ""
		if numberOfPages > 1 {
			footerAdditionalText += " Click on the arrows below to change the page."
		}

		currentPage := 1
		if len(args) > 0 {
			currentPage, err = strconv.Atoi(args[0])
			if err != nil {
				currentPage = 1
			}
		}
		if currentPage > numberOfPages {
			currentPage = 1
		}

		channellistEmbed := &discordgo.MessageEmbed{
			Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.stats.channellist-embed-footer", humanize.Comma(int64(len(allChannels)))) + footerAdditionalText},
		}

		s.setEmbedChannellistPage(channellistEmbed, msg.Author, guild, allChannels, currentPage, numberOfPages)
		channellistEmbedMessages, err := helpers.SendEmbed(msg.ChannelID, channellistEmbed)
		helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)

		if len(channellistEmbedMessages) <= 0 {
			helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.errors.generic-nomessage"))
			return
		}
		channellistEmbedMessage := channellistEmbedMessages[0]

		if numberOfPages > 1 {
			err = session.MessageReactionAdd(msg.ChannelID, channellistEmbedMessage.ID, "⬅")
			helpers.Relax(err)
			err = session.MessageReactionAdd(msg.ChannelID, channellistEmbedMessage.ID, "➡")
			helpers.Relax(err)
		}

		closeHandler := session.AddHandler(func(session *discordgo.Session, reaction *discordgo.MessageReactionAdd) {
			defer helpers.Recover()

			if reaction.MessageID == channellistEmbedMessage.ID {
				if reaction.UserID == session.State.User.ID {
					return
				}

				if reaction.UserID == msg.Author.ID {
					if reaction.Emoji.Name == "➡" {
						if currentPage+1 <= numberOfPages {
							currentPage += 1
							s.setEmbedChannellistPage(channellistEmbed, msg.Author, guild, allChannels, currentPage, numberOfPages)
							channellistEmbedMessage, err = helpers.EditEmbed(msg.ChannelID, channellistEmbedMessage.ID, channellistEmbed)
							helpers.Relax(err)
						}
					} else if reaction.Emoji.Name == "⬅" {
						if currentPage-1 >= 1 {
							currentPage -= 1
							s.setEmbedChannellistPage(channellistEmbed, msg.Author, guild, allChannels, currentPage, numberOfPages)
							channellistEmbedMessage, err = helpers.EditEmbed(msg.ChannelID, channellistEmbedMessage.ID, channellistEmbed)
							helpers.Relax(err)
						}
					}
				}
				err = session.MessageReactionRemove(reaction.ChannelID, reaction.MessageID, reaction.Emoji.Name, reaction.UserID)
				if errD, ok := err.(*discordgo.RESTError); !ok || (errD.Message.Code != discordgo.ErrCodeUnknownMessage && errD.Message.Code != discordgo.ErrCodeMissingPermissions) {
					helpers.RelaxLog(err)
				}
			}
		})
		time.Sleep(3 * time.Minute)
		closeHandler()
		if numberOfPages > 1 {
			err = session.MessageReactionRemove(msg.ChannelID, channellistEmbedMessage.ID, "⬅", session.State.User.ID)
			if errD, ok := err.(*discordgo.RESTError); !ok || errD.Message.Code != discordgo.ErrCodeUnknownMessage {
				helpers.RelaxLog(err)
			}
			err = session.MessageReactionRemove(msg.ChannelID, channellistEmbedMessage.ID, "➡", session.State.User.ID)
			if errD, ok := err.(*discordgo.RESTError); !ok || errD.Message.Code != discordgo.ErrCodeUnknownMessage {
				helpers.RelaxLog(err)
			}
		}

		return
	}
}

func (r *Stats) setEmbedEmojiPage(reactionEmbed *discordgo.MessageEmbed, author *discordgo.User, guild *discordgo.Guild, pageN int, maxPagesN int) {
	reactionEmbed.Fields = []*discordgo.MessageEmbedField{}
	pageText := ""
	if maxPagesN > 1 {
		pageText = fmt.Sprintf(" | Page %d of %d", pageN, maxPagesN)
	}
	reactionEmbed.Title = helpers.GetTextF("plugins.stats.reaction-embed-title", author.Username, guild.Name) + pageText
	startEmoteN := (pageN - 1) * 12
	i := startEmoteN
	var value string
	for {
		if i < len(guild.Emojis) {
			value = "<"
			if guild.Emojis[i].Animated {
				value += "a"
			}
			value += fmt.Sprintf(":%s>", guild.Emojis[i].APIName())
			reactionEmbed.Fields = append(reactionEmbed.Fields, &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("`:%s:`", guild.Emojis[i].Name),
				Value:  value,
				Inline: true,
			})
		}
		i++
		if i >= startEmoteN+12 {
			break
		}
	}
	return
}

func (r *Stats) setEmbedMemberlistPage(memberlistEmbed *discordgo.MessageEmbed, author *discordgo.User, guild *discordgo.Guild, allMembers []*discordgo.Member, pageN int, maxPagesN int, kind string, kindTitle string) {
	memberlistEmbed.Fields = []*discordgo.MessageEmbedField{}
	pageText := ""
	if maxPagesN > 1 {
		pageText = fmt.Sprintf(" | Page %s of %s", humanize.Comma(int64(pageN)), humanize.Comma(int64(maxPagesN)))
	}
	switch kind {
	case "role":
		memberlistEmbed.Title = helpers.GetTextF("plugins.stats.role-memberlist-embed-title", author.Username, guild.Name, kindTitle) + pageText
		break
	default:
		memberlistEmbed.Title = helpers.GetTextF("plugins.stats.memberlist-embed-title", author.Username, guild.Name) + pageText
		break
	}
	memberlistEmbed.Description = ""
	startMemberN := (pageN - 1) * 10
	i := startMemberN
	for {
		if i < len(allMembers) {
			title := fmt.Sprintf("%s#%s", allMembers[i].User.Username, allMembers[i].User.Discriminator)
			if allMembers[i].Nick != "" {
				title = fmt.Sprintf("%s#%s ~ %s", allMembers[i].User.Username, allMembers[i].User.Discriminator, allMembers[i].Nick)
			}

			joinedServerTime, _ := discordgo.Timestamp(allMembers[i].JoinedAt).Parse()
			memberlistEmbed.Description += fmt.Sprintf("%d: %s joined %s\n", i+1, title, helpers.SinceInDaysText(joinedServerTime))
		}
		i++
		if i >= startMemberN+10 {
			break
		}
	}
	return
}

func (r *Stats) setEmbedRolelistPage(memberlistEmbed *discordgo.MessageEmbed, author *discordgo.User, guild *discordgo.Guild, allRoles []*discordgo.Role, pageN int, maxPagesN int) {
	memberlistEmbed.Fields = []*discordgo.MessageEmbedField{}
	pageText := ""
	if maxPagesN > 1 {
		pageText = fmt.Sprintf(" | Page %s of %s", humanize.Comma(int64(pageN)), humanize.Comma(int64(maxPagesN)))
	}
	memberlistEmbed.Title = helpers.GetTextF("plugins.stats.rolelist-embed-title", author.Username, guild.Name) + pageText
	memberlistEmbed.Description = ""
	startMemberN := (pageN - 1) * 10
	i := startMemberN
	for {
		if i < len(allRoles) {
			var information []string
			var informationText string
			if allRoles[i].Color > 0 {
				information = append(information, "#"+helpers.GetHexFromDiscordColor(allRoles[i].Color))
			}
			if allRoles[i].Hoist {
				information = append(information, "hoisted")
			}
			if allRoles[i].Mentionable {
				information = append(information, "mentionable")
			}
			if allRoles[i].Managed {
				information = append(information, "managed")
			}
			if len(information) > 0 {
				informationText = ", " + strings.Join(information, ", ")
			}

			memberlistEmbed.Description += fmt.Sprintf(
				"%d: %s (#%s%s)\n", i+1, allRoles[i].Name, allRoles[i].ID, informationText)
		}
		i++
		if i >= startMemberN+10 {
			break
		}
	}
	return
}

func (r *Stats) setEmbedChannellistPage(memberlistEmbed *discordgo.MessageEmbed, author *discordgo.User, guild *discordgo.Guild, allChannels []*discordgo.Channel, pageN int, maxPagesN int) {
	memberlistEmbed.Fields = []*discordgo.MessageEmbedField{}
	pageText := ""
	if maxPagesN > 1 {
		pageText = fmt.Sprintf(" | Page %s of %s", humanize.Comma(int64(pageN)), humanize.Comma(int64(maxPagesN)))
	}
	memberlistEmbed.Title = helpers.GetTextF("plugins.stats.channellist-embed-title", author.Username, guild.Name) + pageText
	memberlistEmbed.Description = ""
	startMemberN := (pageN - 1) * 10
	i := startMemberN
	for {
		if i < len(allChannels) {
			var information []string
			var informationText, prefixText string
			switch allChannels[i].Type {
			case discordgo.ChannelTypeGuildCategory:
				information = append(information, "category")
				break
			case discordgo.ChannelTypeGuildVoice:
				information = append(information, "voice")
				break
			case discordgo.ChannelTypeGuildText:
				information = append(information, "text")
				break
			}

			if allChannels[i].NSFW {
				information = append(information, "NSFW")
			}
			if allChannels[i].Bitrate > 0 {
				information = append(information, humanize.Comma(int64(allChannels[i].Bitrate))+"kbps")
			}

			if len(information) > 0 {
				informationText = ", " + strings.Join(information, ", ")
			}

			if allChannels[i].Type == discordgo.ChannelTypeGuildCategory {
				prefixText += ":arrow_down: "
			}

			memberlistEmbed.Description += fmt.Sprintf(
				"%d: %s (#%s%s)\n", i+1, prefixText+allChannels[i].Name, allChannels[i].ID, informationText)
		}
		i++
		if i >= startMemberN+10 {
			break
		}
	}
	return
}

// source: http://stackoverflow.com/a/18695740
func (r *Stats) rankByDuration(durations map[string]int64) VoiceChannelDurationPairList {
	pl := make(VoiceChannelDurationPairList, len(durations))
	i := 0
	for k, v := range durations {
		pl[i] = VoiceChannelDurationPair{k, v}
		i++
	}
	sort.Sort(sort.Reverse(pl))
	return pl
}

type VoiceChannelDurationPair struct {
	Key   string
	Value int64
}
type VoiceChannelDurationPairList []VoiceChannelDurationPair

func (p VoiceChannelDurationPairList) Len() int           { return len(p) }
func (p VoiceChannelDurationPairList) Less(i, j int) bool { return p[i].Value < p[j].Value }
func (p VoiceChannelDurationPairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
