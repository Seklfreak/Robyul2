package plugins

import (
    "fmt"
    "github.com/Seklfreak/Robyul2/helpers"
    "github.com/Seklfreak/Robyul2/metrics"
    "github.com/Seklfreak/Robyul2/version"
    "github.com/bwmarrin/discordgo"
    "github.com/dustin/go-humanize"
    "runtime"
    "strconv"
    "time"
    "strings"
    "github.com/bradfitz/slice"
    "github.com/Seklfreak/Robyul2/logger"
    rethink "github.com/gorethink/gorethink"
    "sort"
    "math"
    "github.com/Seklfreak/Robyul2/emojis"
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
        "memberlist",
        "members",
    }
}

var (
    voiceStatesWithTime []VoiceStateWithTime
)

type DB_VoiceTime struct {
    ID           string        `gorethink:"id,omitempty"`
    GuildID      string        `gorethink:"guildid"`
    ChannelID    string        `gorethink:"channelid"`
    UserID       string        `gorethink:"userid"`
    JoinTimeUtc  time.Time     `gorethink:"join_time_utc"`
    LeaveTimeUtc time.Time     `gorethink:"leave_time_utc"`
}

type VoiceStateWithTime struct {
    VoiceState  *discordgo.VoiceState
    JoinTimeUtc time.Time
}

func (s *Stats) Init(session *discordgo.Session) {
    go func() {
        defer helpers.Recover()

        var voiceStatesBefore []*discordgo.VoiceState
        var voiceStatesCurrently []*discordgo.VoiceState
        for {
            voiceStatesCurrently = []*discordgo.VoiceState{}
            // get for all vc users
            for _, guild := range session.State.Guilds {
                for _, voiceState := range guild.VoiceStates {
                    voiceStatesCurrently = append(voiceStatesCurrently, voiceState)
                    alreadyInVoiceStatesWithTime := false
                    for _, voiceStateWithTime := range voiceStatesWithTime {
                        if voiceState.UserID == voiceStateWithTime.VoiceState.UserID && voiceState.ChannelID == voiceStateWithTime.VoiceState.ChannelID {
                            alreadyInVoiceStatesWithTime = true
                        }
                    }
                    if alreadyInVoiceStatesWithTime == false {
                        voiceStatesWithTime = append(voiceStatesWithTime, VoiceStateWithTime{VoiceState: voiceState, JoinTimeUtc: time.Now().UTC()})
                    }
                }
            }
            // check who left since last check
            for _, voiceStateBefore := range voiceStatesBefore {
                userStillConnected := false
                voiceStateWithTimeIndex := -1
                for _, voiceStateCurrently := range voiceStatesCurrently {
                    if voiceStateCurrently.UserID == voiceStateBefore.UserID && voiceStateCurrently.ChannelID == voiceStateBefore.ChannelID {
                        userStillConnected = true
                    }
                }
                if userStillConnected == false {
                    for i, voiceStateWithTimeEntry := range voiceStatesWithTime {
                        if voiceStateBefore.UserID == voiceStateWithTimeEntry.VoiceState.UserID && voiceStateBefore.ChannelID == voiceStateWithTimeEntry.VoiceState.ChannelID {
                            voiceStateWithTimeIndex = i
                        }
                    }
                }
                if voiceStateWithTimeIndex >= 0 && voiceStateWithTimeIndex < len(voiceStatesWithTime) {
                    channel, err := session.State.Channel(voiceStateBefore.ChannelID)
                    if err == nil {
                        newVoiceTime := s.getVoiceTimeEntryByOrCreateEmpty("id", "")
                        newVoiceTime.GuildID = channel.GuildID
                        newVoiceTime.ChannelID = channel.ID
                        newVoiceTime.UserID = voiceStateBefore.UserID
                        newVoiceTime.LeaveTimeUtc = time.Now().UTC()
                        newVoiceTime.JoinTimeUtc = voiceStatesWithTime[voiceStateWithTimeIndex].JoinTimeUtc
                        s.setVoiceTimeEntry(newVoiceTime)
                        voiceStatesWithTime = append(voiceStatesWithTime[:voiceStateWithTimeIndex], voiceStatesWithTime[voiceStateWithTimeIndex+1:]...)
                        logger.PLUGIN.L("stats", fmt.Sprintf("Saved Voice Session Length in DB for user #%s in channel #%s on server #%s",
                            newVoiceTime.UserID, newVoiceTime.ChannelID, newVoiceTime.GuildID))
                    } else {
                        if err.Error() != "channel not found" {
                            helpers.Relax(err)
                        }
                    }
                }
            }
            voiceStatesBefore = voiceStatesCurrently

            time.Sleep(30 * time.Second)
        }
    }()

    logger.PLUGIN.L("stats", "Started voice stats loop (30s)")
}

func (s *Stats) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    switch command {
    case "stats":
        session.ChannelTyping(msg.ChannelID)
        // Count guilds, channels and users
        users := make(map[string]string)
        channels := 0
        guilds := session.State.Guilds

        for _, guild := range guilds {
            channels += len(guild.Channels)

            lastAfterMemberId := ""
            for {
                members, err := session.GuildMembers(guild.ID, lastAfterMemberId, 1000)
                if len(members) <= 0 {
                    break
                }
                lastAfterMemberId = members[len(members)-1].User.ID
                helpers.Relax(err)
                for _, u := range members {
                    users[u.User.ID] = u.User.Username
                }
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

        uptime := time.Now().Sub(time.Unix(bootTime, 0)).String()

        session.ChannelMessageSendEmbed(msg.ChannelID, &discordgo.MessageEmbed{
            Color: 0x0FADED,
            Thumbnail: &discordgo.MessageEmbedThumbnail{
                URL: fmt.Sprintf(
                    "https://cdn.discordapp.com/avatars/%s/%s.jpg",
                    session.State.User.ID,
                    session.State.User.Avatar,
                ),
            },
            Fields: []*discordgo.MessageEmbedField{
                // Build
                {Name: "Build Time", Value: version.BUILD_TIME, Inline: false},
                {Name: "Build System", Value: version.BUILD_USER + "@" + version.BUILD_HOST, Inline: false},

                // System
                {Name: "Bot Uptime", Value: uptime, Inline: true},
                {Name: "Bot Version", Value: version.BOT_VERSION, Inline: true},
                {Name: "GO Version", Value: runtime.Version(), Inline: true},

                // Bot
                {Name: "Used RAM", Value: humanize.Bytes(ram.Alloc) + "/" + humanize.Bytes(ram.Sys), Inline: true},
                {Name: "Collected garbage", Value: humanize.Bytes(ram.TotalAlloc), Inline: true},
                {Name: "Running coroutines", Value: strconv.Itoa(runtime.NumGoroutine()), Inline: true},

                // Discord
                {Name: "Connected servers", Value: strconv.Itoa(len(guilds)), Inline: true},
                {Name: "Watching channels", Value: strconv.Itoa(channels), Inline: true},
                {Name: "Users with access to me", Value: strconv.Itoa(len(users)), Inline: true},

                // Link
                {Name: "Want more stats and awesome graphs?", Value: "Visit my [datadog dashboard](https://p.datadoghq.com/sb/bde759469-fe3b1d3515)", Inline: false},
            },
        })
    case "serverinfo":
        session.ChannelTyping(msg.ChannelID)
        currentChannel, err := session.Channel(msg.ChannelID)
        helpers.Relax(err)
        guild, err := session.Guild(currentChannel.GuildID)
        helpers.Relax(err)
        users := make(map[string]string)
        lastAfterMemberId := ""
        for {
            members, err := session.GuildMembers(guild.ID, lastAfterMemberId, 1000)
            helpers.Relax(err)
            if len(members) <= 0 {
                break
            }

            lastAfterMemberId = members[len(members)-1].User.ID
            for _, u := range members {
                users[u.User.ID] = u.User.Username
            }
        }

        textChannels := 0
        voiceChannels := 0
        for _, channel := range guild.Channels {
            if channel.Type == "voice" {
                voiceChannels += 1
            } else if channel.Type == "text" {
                textChannels += 1
            }
        }
        online := 0
        for _, presence := range guild.Presences {
            if presence.Status == discordgo.StatusOnline || presence.Status == discordgo.StatusDoNotDisturb || presence.Status == discordgo.StatusIdle {
                online += 1
            }
        }

        createdAtTime := helpers.GetTimeFromSnowflake(guild.ID)

        owner, err := session.User(guild.OwnerID)
        helpers.Relax(err)
        member, err := session.GuildMember(guild.ID, guild.OwnerID)
        helpers.Relax(err)
        ownerText := fmt.Sprintf("%s#%s", owner.Username, owner.Discriminator)
        if member.Nick != "" {
            ownerText = fmt.Sprintf("%s#%s ~ %s", owner.Username, owner.Discriminator, member.Nick)
        }

        emoteText := "None"
        emoteN := 0
        for _, emote := range guild.Emojis {
            if emoteN == 0 {
                emoteText = fmt.Sprintf("`:%s:`", emote.Name)
            } else {

                emoteText += fmt.Sprintf(", `:%s:`", emote.Name)
            }
            emoteN += 1
        }
        if emoteText != "None" {
            emoteText += fmt.Sprintf(" (%d in Total)", emoteN)
        }

        serverinfoEmbed := &discordgo.MessageEmbed{
            Color:       0x0FADED,
            Title:       guild.Name,
            Description: fmt.Sprintf("Since: %s. That's %s.", createdAtTime.Format(time.ANSIC), helpers.SinceInDaysText(createdAtTime)),
            Footer:      &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Server ID: %s", guild.ID)},
            Fields: []*discordgo.MessageEmbedField{
                {Name: "Region", Value: guild.Region, Inline: true},
                {Name: "Users", Value: fmt.Sprintf("%d/%d", online, len(users)), Inline: true},
                {Name: "Text Channels", Value: strconv.Itoa(textChannels), Inline: true},
                {Name: "Voice Channels", Value: strconv.Itoa(voiceChannels), Inline: true},
                {Name: "Roles", Value: strconv.Itoa(len(guild.Roles)), Inline: true},
                {Name: "Owner", Value: ownerText, Inline: true},
                {Name: "Emotes", Value: emoteText, Inline: false},
            },
        }

        if guild.Icon != "" {
            serverinfoEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: fmt.Sprintf("https://cdn.discordapp.com/icons/%s/%s.jpg", guild.ID, guild.Icon) }
            serverinfoEmbed.URL = fmt.Sprintf("https://cdn.discordapp.com/icons/%s/%s.jpg", guild.ID, guild.Icon)
        }

        _, err = session.ChannelMessageSendEmbed(msg.ChannelID, serverinfoEmbed)
        helpers.Relax(err)
    case "userinfo":
        session.ChannelTyping(msg.ChannelID)
        targetUser, err := session.User(msg.Author.ID)
        helpers.Relax(err)
        args := strings.Fields(content)
        if len(args) >= 1 && args[0] != "" {
            targetUser, err = helpers.GetUserFromMention(args[0])
            if err != nil {
                if err, ok := err.(*discordgo.RESTError); ok && err.Message.Code == 10013 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.stats.user-not-found"))
                    helpers.Relax(err)
                    return
                } else {
                    helpers.Relax(err)
                }
            }
        }

        currentChannel, err := session.Channel(msg.ChannelID)
        helpers.Relax(err)
        currentGuild, err := session.Guild(currentChannel.GuildID)
        helpers.Relax(err)
        targetMember, err := session.GuildMember(currentGuild.ID, targetUser.ID)
        if err != nil {
            if err, ok := err.(*discordgo.RESTError); ok && err.Message.Code == 10007 {
                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.stats.user-not-found"))
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
                case "idle":
                    status = "Away"
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
        title := fmt.Sprintf("%s#%s", targetUser.Username, targetUser.Discriminator)
        if nick != "" {
            title = fmt.Sprintf("%s#%s ~ %s", targetUser.Username, targetUser.Discriminator, nick)
        }
        rolesText := "None"
        guildRoles, err := session.GuildRoles(currentGuild.ID)
        helpers.Relax(err)
        isFirst := true
        slice.Sort(guildRoles, func(i, j int) bool {
            return guildRoles[i].Position > guildRoles[j].Position
        })
        for _, guildRole := range guildRoles {
            for _, userRole := range targetMember.Roles {
                if guildRole.ID == userRole {
                    if isFirst == true {
                        rolesText = fmt.Sprintf("%s", guildRole.Name)
                    } else {

                        rolesText += fmt.Sprintf(", %s", guildRole.Name)
                    }
                    isFirst = false
                }
            }
        }

        joinedTime := helpers.GetTimeFromSnowflake(targetUser.ID)
        joinedServerTime, err := discordgo.Timestamp(targetMember.JoinedAt).Parse()
        helpers.Relax(err)

        lastAfterMemberId := ""
        var allMembers []*discordgo.Member
        for {
            members, err := session.GuildMembers(currentGuild.ID, lastAfterMemberId, 1000)
            if len(members) <= 0 {
                break
            }
            lastAfterMemberId = members[len(members)-1].User.ID
            helpers.Relax(err)
            for _, u := range members {
                allMembers = append(allMembers, u)
            }
        }
        slice.Sort(allMembers[:], func(i, j int) bool {
            iMemberTime, err := discordgo.Timestamp(allMembers[i].JoinedAt).Parse()
            helpers.Relax(err)
            jMemberTime, err := discordgo.Timestamp(allMembers[j].JoinedAt).Parse()
            helpers.Relax(err)
            return iMemberTime.Before(jMemberTime)
        })
        userNumber := -1
        for i, sortedMember := range allMembers[:] {
            if sortedMember.User.ID == targetUser.ID {
                userNumber = i + 1
                break
            }
        }

        // Bots cannot use the search endpoint...
        //totalMessagesText := "failed to acquire"
        //searchUrl := fmt.Sprintf(discordgo.EndpointAPI+"guilds/%s/messages/search?author_id=%s", currentChannel.GuildID, targetUser.ID)
        //resp, err := session.Request("GET", searchUrl, nil)
        //if err == nil {
        //    searchResult, err := gabs.ParseJSON(resp)
        //    if err == nil {
        //        totalMessagesText = humanize.Comma(searchResult.Path("total_results").Data().(int64)) + " Messages"
        //    }
        //}
        //if err != nil {
        //    fmt.Println(err)
        //    raven.SetUserContext(&raven.User{
        //        ID:       msg.ID,
        //        Username: msg.Author.Username + "#" + msg.Author.Discriminator,
        //    })
        //    raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{
        //        "ChannelID":       msg.ChannelID,
        //        "Content":         msg.Content,
        //        "Timestamp":       string(msg.Timestamp),
        //        "TTS":             strconv.FormatBool(msg.Tts),
        //        "MentionEveryone": strconv.FormatBool(msg.MentionEveryone),
        //        "IsBot":           strconv.FormatBool(msg.Author.Bot),
        //    })
        //    err = nil
        //}
        //{Name: "Total Messages", Value: totalMessagesText, Inline: false},

        userinfoEmbed := &discordgo.MessageEmbed{
            Color:  0x0FADED,
            Title:  title,
            Footer: &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Member #%d | User ID: %s", userNumber, targetUser.ID)},
            Fields: []*discordgo.MessageEmbedField{
                {Name: "Joined Discord on", Value: fmt.Sprintf("%s (%s)", joinedTime.Format(time.ANSIC), helpers.SinceInDaysText(joinedTime)), Inline: true},
                {Name: "Joined this server on", Value: fmt.Sprintf("%s (%s)", joinedServerTime.Format(time.ANSIC), helpers.SinceInDaysText(joinedServerTime)), Inline: true},
                {Name: "Roles", Value: rolesText, Inline: false},
                {Name: "Voice Stats",
                    Value: fmt.Sprintf("use `%svoicestats @%s` to view the voice stats for this user",
                        helpers.GetPrefixForServer(currentGuild.ID),
                        fmt.Sprintf("%s#%s", targetUser.Username, targetUser.Discriminator)), Inline: false},
            },
        }
        if description != "" {
            userinfoEmbed.Description = description
        }

        if helpers.GetAvatarUrl(targetUser) != "" {
            userinfoEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: helpers.GetAvatarUrl(targetUser)}
            userinfoEmbed.URL = helpers.GetAvatarUrl(targetUser)
        }
        if gameUrl != "" {
            userinfoEmbed.URL = gameUrl
        }

        _, err = session.ChannelMessageSendEmbed(msg.ChannelID, userinfoEmbed)
        helpers.Relax(err)
    case "voicestats": // [p]voicestats <user> or [p]voicestats top
        // @TODO: sort by time connected
        session.ChannelTyping(msg.ChannelID)
        targetUser, err := session.User(msg.Author.ID)
        helpers.Relax(err)
        args := strings.Fields(content)

        channel, err := session.Channel(msg.ChannelID)
        helpers.Relax(err)

        if len(args) >= 1 && args[0] != "" {
            switch args[0] {
            case "leaderboard", "top": // [p]voicestats top
                var entryBucket []DB_VoiceTime
                listCursor, err := rethink.Table("stats_voicetimes").Filter(
                    rethink.Row.Field("guildid").Eq(channel.GuildID),
                ).Run(helpers.GetDB())
                helpers.Relax(err)
                defer listCursor.Close()
                err = listCursor.All(&entryBucket)

                if err != nil {
                    if err == rethink.ErrEmptyResult {
                        _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.stats.voicestats-toplist-no-entries"))
                        helpers.Relax(err)
                    } else {
                        helpers.Relax(err)
                    }
                    return
                }
                if len(entryBucket) <= 0 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.stats.voicestats-toplist-no-entries"))
                    helpers.Relax(err)
                    return
                }

                voiceChannelsDurationPerUser := map[string]map[string]time.Duration{}
                var totalDuration time.Duration

                for _, voiceTime := range entryBucket {
                    if voiceTime.UserID == session.State.User.ID { // Don't show Robyul in the stats
                        continue
                    }

                    voiceChannelDuration := voiceTime.LeaveTimeUtc.Sub(voiceTime.JoinTimeUtc)
                    totalDuration += voiceChannelDuration
                    if _, ok := voiceChannelsDurationPerUser[voiceTime.ChannelID]; ok {
                        if _, ok := voiceChannelsDurationPerUser[voiceTime.ChannelID][voiceTime.UserID]; ok {
                            voiceChannelsDurationPerUser[voiceTime.ChannelID][voiceTime.UserID] += voiceChannelDuration
                        } else {
                            voiceChannelsDurationPerUser[voiceTime.ChannelID][voiceTime.UserID] = voiceChannelDuration
                        }
                    } else {
                        voiceChannelsDurationPerUser[voiceTime.ChannelID] = map[string]time.Duration{}
                        voiceChannelsDurationPerUser[voiceTime.ChannelID][voiceTime.UserID] = voiceChannelDuration
                    }
                }

                if totalDuration <= 0 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.stats.voicestats-toplist-no-entries"))
                    helpers.Relax(err)
                    return
                }

                totalVoiceStatsEmbed := &discordgo.MessageEmbed{
                    Color: 0x0FADED,
                    Title: helpers.GetText("plugins.stats.voicestats-toplist-embed-title"),
                    Description: fmt.Sprintf("Total time connected by all users: **%s**",
                        helpers.HumanizedTimesSinceText(time.Now().UTC().Add(totalDuration))),
                    Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.stats.voicestats-embed-footer")},
                    Fields: []*discordgo.MessageEmbedField{},
                }

                for voiceChannelID, voiceChannelDurations := range voiceChannelsDurationPerUser {
                    resultPairs := s.rankByDuration(voiceChannelDurations)

                    voiceChannel, err := session.Channel(voiceChannelID)
                    helpers.Relax(err)

                    channelToplistText := ""

                    i := 0
                    for _, voiceUserDuration := range resultPairs {
                        channelToplistText += fmt.Sprintf("#%d: <@%s>: %s\n",
                            i+1,
                            voiceUserDuration.Key,
                            helpers.HumanizedTimesSinceText(time.Now().UTC().Add(voiceUserDuration.Value)))
                        i++
                        if i >= 5 {
                            break
                        }
                    }

                    totalVoiceStatsEmbed.Fields = append(totalVoiceStatsEmbed.Fields, &discordgo.MessageEmbedField{
                        Name:   fmt.Sprintf("Top 5 users connected to #%s", voiceChannel.Name),
                        Value:  channelToplistText,
                        Inline: false,
                    })
                }

                _, err = session.ChannelMessageSendEmbed(msg.ChannelID, totalVoiceStatsEmbed)
                helpers.Relax(err)
                return
            }
            targetUser, err = helpers.GetUserFromMention(args[0])
            if err != nil || targetUser.ID == "" {
                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                helpers.Relax(err)
                return
            }
        }

        currentConnectionText := "Currently not connected to any Voice Channel on this server."
        for _, voiceStateWithTime := range voiceStatesWithTime {
            if voiceStateWithTime.VoiceState.GuildID == channel.GuildID && voiceStateWithTime.VoiceState.UserID == targetUser.ID {
                //duration := time.Since(voiceStateWithTime.JoinTimeUtc)
                currentVoiceChannel, err := session.Channel(voiceStateWithTime.VoiceState.ChannelID)
                helpers.Relax(err)
                currentConnectionText = fmt.Sprintf("Connected to **<#%s>** since **%s**",
                    currentVoiceChannel.ID,
                    helpers.HumanizedTimesSinceText(voiceStateWithTime.JoinTimeUtc))
            }
        }

        title := fmt.Sprintf("Voice Stats for %s", targetUser.Username)

        var entryBucket []DB_VoiceTime
        listCursor, err := rethink.Table("stats_voicetimes").Filter(
            rethink.Row.Field("guildid").Eq(channel.GuildID),
        ).Filter(
            rethink.Row.Field("userid").Eq(targetUser.ID),
        ).Run(helpers.GetDB())
        defer listCursor.Close()
        err = listCursor.All(&entryBucket)

        voiceChannelsDuration := map[string]time.Duration{}

        if err != rethink.ErrEmptyResult && len(entryBucket) > 0 {
            for _, voiceTime := range entryBucket {
                voiceChannelDuration := voiceTime.LeaveTimeUtc.Sub(voiceTime.JoinTimeUtc)
                if _, ok := voiceChannelsDuration[voiceTime.ChannelID]; ok {
                    voiceChannelsDuration[voiceTime.ChannelID] += voiceChannelDuration
                } else {
                    voiceChannelsDuration[voiceTime.ChannelID] = voiceChannelDuration
                }
            }
        } else if err != nil && err != rethink.ErrEmptyResult {
            helpers.Relax(err)
        }

        voicestatsEmbed := &discordgo.MessageEmbed{
            Color:       0x0FADED,
            Title:       title,
            Description: currentConnectionText,
            Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.stats.voicestats-embed-footer")},
            Fields:      []*discordgo.MessageEmbedField{},
        }

        guildChannels, err := session.GuildChannels(channel.GuildID)
        helpers.Relax(err)

        slice.Sort(guildChannels, func(i, j int) bool {
            return guildChannels[i].Position < guildChannels[j].Position
        })

        for _, guildChannel := range guildChannels {
            for voiceChannelID, voiceChannelDuration := range voiceChannelsDuration {
                if voiceChannelID == guildChannel.ID {
                    voiceChannel, err := session.Channel(voiceChannelID)
                    helpers.Relax(err)
                    voicestatsEmbed.Fields = append(voicestatsEmbed.Fields, &discordgo.MessageEmbedField{
                        Name:   fmt.Sprintf("Total duration connected to #%s", voiceChannel.Name),
                        Value:  fmt.Sprintf("%s", helpers.HumanizedTimesSinceText(time.Now().UTC().Add(voiceChannelDuration))),
                        Inline: false,
                    })
                }
            }
        }

        _, err = session.ChannelMessageSendEmbed(msg.ChannelID, voicestatsEmbed)
        helpers.Relax(err)
    case "emotes", "emojis": // [p]emotes
        session.ChannelTyping(msg.ChannelID)
        channel, err := session.State.Channel(msg.ChannelID)
        helpers.Relax(err)
        guild, err := session.Guild(channel.GuildID)
        helpers.Relax(err)

        if len(guild.Emojis) <= 0 {
            _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.stats.no-emotes"))
            helpers.Relax(err)
            return
        }

        numberOfPages := int(math.Ceil(float64(len(guild.Emojis)) / float64(9)))
        footerAdditionalText := ""
        if numberOfPages > 1 {
            footerAdditionalText += " Click on the numbers below to change the page."
        }

        reactionEmbed := &discordgo.MessageEmbed{
            Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.stats.reaction-embed-footer", len(guild.Emojis)) + footerAdditionalText},
        }

        s.setEmbedEmojiPage(reactionEmbed, msg.Author, guild, 1, numberOfPages)
        reactionEmbedMessage, err := session.ChannelMessageSendEmbed(msg.ChannelID, reactionEmbed)
        helpers.Relax(err)

        reactionsAdded := 0
        if numberOfPages > 1 {
            go func() {
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
            if reaction.MessageID == reactionEmbedMessage.ID {
                if reaction.UserID == session.State.User.ID {
                    return
                }

                if reaction.UserID == msg.Author.ID {
                    newPageN := emojis.ToNumber(reaction.Emoji.Name)
                    if newPageN >= 1 && newPageN <= numberOfPages {
                        s.setEmbedEmojiPage(reactionEmbed, msg.Author, guild, newPageN, numberOfPages)
                        reactionEmbedMessage, err = session.ChannelMessageEditEmbed(msg.ChannelID, reactionEmbedMessage.ID, reactionEmbed)
                        helpers.Relax(err)
                    }
                }
                err = session.MessageReactionRemove(reaction.ChannelID, reaction.MessageID, reaction.Emoji.Name, reaction.UserID)
                helpers.Relax(err)
            }
        })
        time.Sleep(3 * time.Minute)
        closeHandler()
        reactionsRemoved := 0
        if numberOfPages > 1 {
            for {
                session.MessageReactionRemove(msg.ChannelID, reactionEmbedMessage.ID, emojis.From(strconv.Itoa(reactionsRemoved+1)), session.State.User.ID)
                reactionsRemoved++
                if reactionsRemoved >= numberOfPages {
                    break
                }
            }

        }

        return
    case "memberlist", "members": // [p]memberlist [<page #>]
        session.ChannelTyping(msg.ChannelID)
        channel, err := session.State.Channel(msg.ChannelID)
        helpers.Relax(err)
        guild, err := session.Guild(channel.GuildID)
        helpers.Relax(err)

        memberlistEmbedMessage, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.stats.memberlist-gathering"))
        helpers.Relax(err)

        lastAfterMemberId := ""
        var allMembers []*discordgo.Member
        for {
            members, err := session.GuildMembers(guild.ID, lastAfterMemberId, 1000)
            if len(members) <= 0 {
                break
            }
            lastAfterMemberId = members[len(members)-1].User.ID
            helpers.Relax(err)
            for _, u := range members {
                allMembers = append(allMembers, u)
            }
        }
        slice.Sort(allMembers[:], func(i, j int) bool {
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
        args := strings.Fields(content)
        if len(args) > 0 {
            currentPage, err = strconv.Atoi(args[0])
            if err != nil {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
                return
            }
        }
        if currentPage > numberOfPages {
            currentPage = 1
        }

        memberlistEmbed := &discordgo.MessageEmbed{
            Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.stats.memberlist-embed-footer", humanize.Comma(int64(len(allMembers)))) + footerAdditionalText},
        }

        s.setEmbedMemberlistPage(memberlistEmbed, msg.Author, guild, allMembers, currentPage, numberOfPages)
        memberlistEmbedMessage, err = session.ChannelMessageEdit(msg.ChannelID, memberlistEmbedMessage.ID, "")
        helpers.Relax(err)
        memberlistEmbedMessage, err = session.ChannelMessageEditEmbed(msg.ChannelID, memberlistEmbedMessage.ID, memberlistEmbed)
        helpers.Relax(err)

        if numberOfPages > 1 {
            err = session.MessageReactionAdd(msg.ChannelID, memberlistEmbedMessage.ID, "⬅")
            helpers.Relax(err)
            err = session.MessageReactionAdd(msg.ChannelID, memberlistEmbedMessage.ID, "➡")
            helpers.Relax(err)
        }

        closeHandler := session.AddHandler(func(session *discordgo.Session, reaction *discordgo.MessageReactionAdd) {
            if reaction.MessageID == memberlistEmbedMessage.ID {
                if reaction.UserID == session.State.User.ID {
                    return
                }

                if reaction.UserID == msg.Author.ID {
                    if reaction.Emoji.Name == "➡" {
                        if currentPage+1 <= numberOfPages {
                            currentPage += 1
                            s.setEmbedMemberlistPage(memberlistEmbed, msg.Author, guild, allMembers, currentPage, numberOfPages)
                            memberlistEmbedMessage, err = session.ChannelMessageEditEmbed(msg.ChannelID, memberlistEmbedMessage.ID, memberlistEmbed)
                            helpers.Relax(err)
                        }
                    } else if reaction.Emoji.Name == "⬅" {
                        if currentPage-1 >= 1 {
                            currentPage -= 1
                            s.setEmbedMemberlistPage(memberlistEmbed, msg.Author, guild, allMembers, currentPage, numberOfPages)
                            memberlistEmbedMessage, err = session.ChannelMessageEditEmbed(msg.ChannelID, memberlistEmbedMessage.ID, memberlistEmbed)
                            helpers.Relax(err)
                        }
                    }
                }
                err = session.MessageReactionRemove(reaction.ChannelID, reaction.MessageID, reaction.Emoji.Name, reaction.UserID)
                helpers.Relax(err)
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
    }
}

func (r *Stats) setEmbedEmojiPage(reactionEmbed *discordgo.MessageEmbed, author *discordgo.User, guild *discordgo.Guild, pageN int, maxPagesN int) {
    reactionEmbed.Fields = []*discordgo.MessageEmbedField{}
    pageText := ""
    if maxPagesN > 1 {
        pageText = fmt.Sprintf(" | Page %d of %d", pageN, maxPagesN)
    }
    reactionEmbed.Title = helpers.GetTextF("plugins.stats.reaction-embed-title", author.Username, guild.Name) + pageText
    startEmoteN := (pageN - 1) * 9
    i := startEmoteN
    for {
        if i < len(guild.Emojis) {
            reactionEmbed.Fields = append(reactionEmbed.Fields, &discordgo.MessageEmbedField{
                Name:   fmt.Sprintf("`:%s:`", guild.Emojis[i].Name),
                Value:  fmt.Sprintf("<:%s>", guild.Emojis[i].APIName()),
                Inline: true,
            })
        }
        i++
        if i >= startEmoteN+9 {
            break
        }
    }
    return
}

func (r *Stats) setEmbedMemberlistPage(memberlistEmbed *discordgo.MessageEmbed, author *discordgo.User, guild *discordgo.Guild, allMembers []*discordgo.Member, pageN int, maxPagesN int) {
    memberlistEmbed.Fields = []*discordgo.MessageEmbedField{}
    pageText := ""
    if maxPagesN > 1 {
        pageText = fmt.Sprintf(" | Page %s of %s", humanize.Comma(int64(pageN)), humanize.Comma(int64(maxPagesN)))
    }
    memberlistEmbed.Title = helpers.GetTextF("plugins.stats.memberlist-embed-title", author.Username, guild.Name) + pageText
    memberlistEmbed.Description = ""
    startMemberN := (pageN - 1) * 10
    i := startMemberN
    for {
        if i < len(allMembers) {
            title := fmt.Sprintf("%s#%s", allMembers[i].User.Username, allMembers[i].User.Discriminator)
            if allMembers[i].Nick != "" {
                title = fmt.Sprintf("%s#%s ~ %s", allMembers[i].User.Username, allMembers[i].User.Discriminator, allMembers[i].Nick)
            }

            joinedServerTime, err := discordgo.Timestamp(allMembers[i].JoinedAt).Parse()
            helpers.Relax(err)
            memberlistEmbed.Description += fmt.Sprintf("%d: %s joined %s\n", i+1, title, helpers.SinceInDaysText(joinedServerTime))
        }
        i++
        if i >= startMemberN+10 {
            break
        }
    }
    return
}

func (r *Stats) setVoiceTimeEntry(entry DB_VoiceTime) {
    _, err := rethink.Table("stats_voicetimes").Update(entry).Run(helpers.GetDB())
    helpers.Relax(err)
}

func (r *Stats) getVoiceTimeEntryByOrCreateEmpty(key string, id string) DB_VoiceTime {
    var entryBucket DB_VoiceTime
    listCursor, err := rethink.Table("stats_voicetimes").Filter(
        rethink.Row.Field(key).Eq(id),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.One(&entryBucket)

    // If user has no DB entries create an empty document
    if err == rethink.ErrEmptyResult {
        insert := rethink.Table("stats_voicetimes").Insert(DB_VoiceTime{})
        res, e := insert.RunWrite(helpers.GetDB())
        // If the creation was successful read the document
        if e != nil {
            panic(e)
        } else {
            return r.getVoiceTimeEntryByOrCreateEmpty("id", res.GeneratedKeys[0])
        }
    } else if err != nil {
        panic(err)
    }

    return entryBucket
}

// source: http://stackoverflow.com/a/18695740
func (r *Stats) rankByDuration(durations map[string]time.Duration) VoiceChannelDurationPairList {
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
    Value time.Duration
}
type VoiceChannelDurationPairList []VoiceChannelDurationPair

func (p VoiceChannelDurationPairList) Len() int           { return len(p) }
func (p VoiceChannelDurationPairList) Less(i, j int) bool { return p[i].Value < p[j].Value }
func (p VoiceChannelDurationPairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
