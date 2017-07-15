package plugins

import (
    "github.com/bwmarrin/discordgo"
    "strings"
    "github.com/Seklfreak/Robyul2/helpers"
    rethink "github.com/gorethink/gorethink"
    "math"
    "time"
    "math/rand"
    "sync"
    "errors"
    "github.com/Seklfreak/Robyul2/ratelimits"
    "fmt"
    "strconv"
    "github.com/Seklfreak/Robyul2/logger"
    "github.com/Seklfreak/Robyul2/metrics"
    "sort"
    "gopkg.in/oleiade/lane.v1"
    "os/exec"
    "os"
    "bytes"
    "io/ioutil"
)

type Levels struct {
    sync.RWMutex

    buckets map[string]int8
}

type ProcessExpInfo struct {
    GuildID string
    UserID  string
}

var (
    LevelsBucket = &ratelimits.BucketContainer{}

    // How many keys a bucket may contain when created
    BUCKET_INITIAL_FILL int8 = 1

    // The maximum amount of keys a user may possess
    BUCKET_UPPER_BOUND int8 = 1

    // How often new keys drip into the buckets
    DROP_INTERVAL = 60 * time.Second

    // How many keys may drop at a time
    DROP_SIZE int8 = 1

    temporaryIgnoredGuilds []string

    expStack *lane.Stack = lane.NewStack()
)

func (m *Levels) Commands() []string {
    return []string{
        "level",
        "levels",
        "profile",
    }
}

type DB_Levels_ServerUser struct {
    ID      string  `gorethink:"id,omitempty"`
    UserID  string  `gorethink:"userid"`
    GuildID string  `gorethink:"guildid"`
    Exp     int64   `gorethink:"exp"`
}

type DB_Profile_Background struct {
    Name    string  `gorethink:"id,omitempty"`
    URL     string  `gorethink:"url"`
}

type DB_Profile_Userdata struct {
    ID      string  `gorethink:"id,omitempty"`
    UserID  string  `gorethink:"userid"`
    Background  string  `gorethink:"background"`
    Title string `gorethink:"title"`
    Bio string `gorethink:"bio"`
}

var (
    cachePath string
    assetsPath string
    htmlTemplateString string
    levelsEnv []string = os.Environ()
    webshotBinary string
)

func (m *Levels) Init(session *discordgo.Session) {
    m.BucketInit()

    cachePath = helpers.GetConfig().Path("cache_folder").Data().(string)
    assetsPath = helpers.GetConfig().Path("assets_folder").Data().(string)
    htmlTemplate, err := ioutil.ReadFile(assetsPath + "profile.html")
    helpers.Relax(err)
    htmlTemplateString = string(htmlTemplate)
    webshotBinary, err = exec.LookPath("webshot")
    helpers.Relax(err)

    go m.processExpStackLoop()
    logger.PLUGIN.L("VLive", "Started processExpStackLoop")
}

func (m *Levels) processExpStackLoop() {
    defer func() {
        helpers.Recover()

        logger.ERROR.L("levels", "The processExpStackLoop died. Please investigate! Will be restarted in 60 seconds")
        time.Sleep(60 * time.Second)
        m.processExpStackLoop()
    }()

    for {
        if !expStack.Empty() {
            expItem := expStack.Pop().(ProcessExpInfo)
            levelsServerUser := m.getLevelsServerUserOrCreateNew(expItem.GuildID, expItem.UserID)
            levelsServerUser.Exp += m.getRandomExpForMessage()
            m.setLevelsServerUser(levelsServerUser)
        } else {
            time.Sleep(1 * time.Second)
        }
    }
}

func (m *Levels) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    switch command {
    case "profile": // [p]profile
        session.ChannelTyping(msg.ChannelID)
        channel, err := session.Channel(msg.ChannelID)
        helpers.Relax(err)
        guild, err := session.Guild(channel.GuildID)
        helpers.Relax(err)
        targetUser, err := session.User(msg.Author.ID)
        helpers.Relax(err)
        args := strings.Fields(content)
        if len(args) >= 1 && args[0] != "" {
            switch args[0] {
            case "title":
                if len(args) < 2 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                    helpers.Relax(err)
                    return
                }
                titleText := strings.TrimSpace(strings.Replace(content, strings.Join(args[:1], " "), "", 1))
                if len(titleText) <= 0 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                    helpers.Relax(err)
                    return
                }

                userUserdata := m.GetUserUserdata(msg.Author)
                userUserdata.Title = titleText
                m.setUserUserdata(userUserdata)

                _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.profile-title-set-success"))
                helpers.Relax(err)
                return
            case "bio":
                if len(args) < 2 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                    helpers.Relax(err)
                    return
                }
                bioText := strings.TrimSpace(strings.Replace(content, strings.Join(args[:1], " "), "", 1))
                if len(bioText) <= 0 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                    helpers.Relax(err)
                    return
                }

                userUserdata := m.GetUserUserdata(msg.Author)
                userUserdata.Bio = bioText
                m.setUserUserdata(userUserdata)

                _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.profile-bio-set-success"))
                helpers.Relax(err)
                return
            case "background":
                if len(args) < 2 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                    helpers.Relax(err)
                    return
                }
                switch args[1] {
                case "add":
                    helpers.RequireBotAdmin(msg, func() {
                        if len(args) < 4 {
                            _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                            helpers.Relax(err)
                            return
                        }
                        backgroundName := args[2]
                        backgroundUrl := args[3]

                        err := m.InsertNewProfileBackground(backgroundName, backgroundUrl)
                        if err != nil {
                            if strings.Contains(err.Error(), "Duplicate primary key") {
                                _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.new-profile-background-add-error-duplicate"))
                                return
                            } else {
                                helpers.Relax(err)
                            }
                        }
                        _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.levels.new-profile-background-add-success", backgroundName))
                        helpers.Relax(err)
                        return
                    })
                    return
                default:
                    if m.ProfileBackgroundNameExists(args[1]) == false {
                        _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.profile-background-set-error-not-found"))
                        helpers.Relax(err)

                        return
                    }

                    userUserdata := m.GetUserUserdata(msg.Author)
                    userUserdata.Background = args[1]
                    m.setUserUserdata(userUserdata)

                    _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.profile-background-set-success"))
                    helpers.Relax(err)
                    return
                }
            }

            targetUser, err = helpers.GetUserFromMention(args[0])
            if targetUser == nil || targetUser.ID == "" {
                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                helpers.Relax(err)
                return
            }
        }

        targetMember, err := session.GuildMember(channel.GuildID, targetUser.ID)
        helpers.Relax(err)

        jpgBytes, err := m.GetProfile(targetMember, guild)
        helpers.Relax(err)

        _, err = session.ChannelFileSendWithMessage(
            msg.ChannelID,
            fmt.Sprintf("<@%s> Profile for %s", msg.Author.ID, targetUser.Username),
            fmt.Sprintf("%s-Robyul.png", targetUser.ID), bytes.NewReader(jpgBytes))
        helpers.Relax(err)

        return
    case "level", "levels": // [p]level <user> or [p]level top
        session.ChannelTyping(msg.ChannelID)
        targetUser, err := session.User(msg.Author.ID)
        helpers.Relax(err)
        args := strings.Fields(content)

        channel, err := session.Channel(msg.ChannelID)
        helpers.Relax(err)

        if len(args) >= 1 && args[0] != "" {
            switch args[0] {
            case "leaderboard", "top": // [p]level top
                var levelsServersUsers []DB_Levels_ServerUser
                listCursor, err := rethink.Table("levels_serverusers").Filter(
                    rethink.Row.Field("guildid").Eq(channel.GuildID),
                ).OrderBy(rethink.Desc("exp")).Limit(10).Run(helpers.GetDB())
                helpers.Relax(err)
                defer listCursor.Close()
                err = listCursor.All(&levelsServersUsers)

                if err == rethink.ErrEmptyResult || len(levelsServersUsers) <= 0 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.top-server-no-stats"))
                    helpers.Relax(err)
                    return
                } else if err != nil {
                    helpers.Relax(err)
                }

                topLevelEmbed := &discordgo.MessageEmbed{
                    Color: 0x0FADED,
                    Title: helpers.GetText("plugins.levels.top-server-embed-title"),
                    //Description: "",
                    //Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.levels.embed-footer", len(session.State.Guilds))},
                    Fields: []*discordgo.MessageEmbedField{},
                }

                displayRanking := 1
                offset := 0
                for i := 0; displayRanking <= 10; i++ {
                    //fmt.Println("displayRanking:", displayRanking, "i:", i, "offset:", offset)
                    if len(levelsServersUsers) <= i-offset {
                        offset += i
                        listCursor, err := rethink.Table("levels_serverusers").Filter(
                            rethink.Row.Field("guildid").Eq(channel.GuildID),
                        ).OrderBy(rethink.Desc("exp")).Skip(offset).Limit(5).Run(helpers.GetDB())
                        helpers.Relax(err)
                        defer listCursor.Close()
                        err = listCursor.All(&levelsServersUsers)
                    }
                    if len(levelsServersUsers) <= i-offset {
                        break
                    }

                    currentMember, err := session.GuildMember(channel.GuildID, levelsServersUsers[i-offset].UserID)
                    if err != nil {
                        logger.ERROR.L("levels", fmt.Sprintf("error fetching member data for user #%s: %s", levelsServersUsers[i-offset].UserID, err.Error()))
                        continue
                    }
                    fullUsername := currentMember.User.Username
                    if currentMember.Nick != "" {
                        fullUsername += " ~ " + currentMember.Nick
                    }
                    topLevelEmbed.Fields = append(topLevelEmbed.Fields, &discordgo.MessageEmbedField{
                        Name:   fmt.Sprintf("#%d: %s", displayRanking, fullUsername),
                        Value:  fmt.Sprintf("Level: %d", m.getLevelFromExp(levelsServersUsers[i-offset].Exp)), // + fmt.Sprintf(", EXP: %d", levelsServersUsers[i-offset].Exp),
                        Inline: false,
                    })
                    displayRanking++
                }

                _, err = session.ChannelMessageSendEmbed(msg.ChannelID, topLevelEmbed)
                helpers.Relax(err)
                return
            case "global-leaderboard", "global-top", "globaltop":
                var levelsUsers []DB_Levels_ServerUser
                listCursor, err := rethink.Table("levels_serverusers").Run(helpers.GetDB())
                helpers.Relax(err)
                defer listCursor.Close()
                err = listCursor.All(&levelsUsers)

                if err == rethink.ErrEmptyResult || len(levelsUsers) <= 0 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.top-server-no-stats"))
                    helpers.Relax(err)
                    return
                } else if err != nil {
                    helpers.Relax(err)
                }

                totalExpMap := make(map[string]int64, 0)
                for _, levelsUser := range levelsUsers {
                    if _, ok := totalExpMap[levelsUser.UserID]; ok {
                        totalExpMap[levelsUser.UserID] += levelsUser.Exp
                    } else {
                        totalExpMap[levelsUser.UserID] = levelsUser.Exp
                    }
                }

                rankedTotalExpMap := m.rankMapByExp(totalExpMap)

                globalTopLevelEmbed := &discordgo.MessageEmbed{
                    Color: 0x0FADED,
                    Title: helpers.GetText("plugins.levels.global-top-server-embed-title"),
                    //Description: "",
                    Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.levels.embed-footer", len(session.State.Guilds))},
                    Fields: []*discordgo.MessageEmbedField{},
                }

                i := 0
                for _, userRanked := range rankedTotalExpMap {
                    currentUser, err := session.User(userRanked.Key)
                    if err != nil {
                        logger.ERROR.L("levels", fmt.Sprintf("error fetching user data for user #%s: %s", userRanked.Key, err.Error()))
                        continue
                    }
                    fullUsername := currentUser.Username
                    globalTopLevelEmbed.Fields = append(globalTopLevelEmbed.Fields, &discordgo.MessageEmbedField{
                        Name:   fmt.Sprintf("#%d: %s", i+1, fullUsername),
                        Value:  fmt.Sprintf("Global Level: %d", m.getLevelFromExp(userRanked.Value)),
                        Inline: false,
                    })
                    i++
                    if i >= 10 {
                        break
                    }
                }

                _, err = session.ChannelMessageSendEmbed(msg.ChannelID, globalTopLevelEmbed)
                helpers.Relax(err)
                return
            case "reset":
                if len(args) >= 2 {
                    switch args[1] {
                    case "user": // [p]levels reset user <user>
                        if len(args) < 3 {
                            _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                            helpers.Relax(err)
                            return
                        }

                        helpers.RequireAdmin(msg, func() {
                            targetUser, err = helpers.GetUserFromMention(args[2])
                            if targetUser == nil || targetUser.ID == "" {
                                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                                helpers.Relax(err)
                                return
                            }

                            levelsServerUser := m.getLevelsServerUserOrCreateNew(channel.GuildID, targetUser.ID)
                            levelsServerUser.Exp = 0
                            m.setLevelsServerUser(levelsServerUser)

                            _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.user-resetted"))
                            helpers.Relax(err)
                            return
                        })
                        return
                    }
                }
                return
            case "ignore":
                    if len(args) >= 2 {
                        switch args[1] {
                        case "list": // [p]levels ignore list
                            helpers.RequireMod(msg, func() {
                                settings := helpers.GuildSettingsGetCached(channel.GuildID)

                                ignoredMessage := "**Ignored Users:**"
                                if len(settings.LevelsIgnoredUserIDs) > 0 {
                                    for i, ignoredUserID := range settings.LevelsIgnoredUserIDs {
                                        ignoredUser, err := session.State.Member(channel.GuildID, ignoredUserID)
                                        if err != nil {
                                            ignoredMessage += " N/A"
                                        } else {
                                            ignoredMessage += " " + ignoredUser.User.Username + "#" + ignoredUser.User.Discriminator
                                        }
                                        if i+1 < len(settings.LevelsIgnoredUserIDs) {
                                            ignoredMessage += ","
                                        }
                                    }
                                } else {
                                    ignoredMessage += " None"
                                }

                                ignoredMessage += "\n**Ignored Channels:**"
                                if len(settings.LevelsIgnoredChannelIDs) > 0 {
                                    for i, ignoredChannelID := range settings.LevelsIgnoredChannelIDs {
                                        ignoredMessage += " <#" + ignoredChannelID + ">"
                                        if i+1 < len(settings.LevelsIgnoredChannelIDs) {
                                            ignoredMessage += ","
                                        }
                                    }
                                } else {
                                    ignoredMessage += " None"
                                }

                                for _, page := range helpers.Pagify(ignoredMessage, " ") {
                                    _, err = session.ChannelMessageSend(msg.ChannelID, page)
                                    helpers.Relax(err)
                                }
                                return
                            })
                            return
                        case "user": // [p]levels ignore user <user>
                            if len(args) < 3 {
                                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                                helpers.Relax(err)
                                return
                            }

                            helpers.RequireAdmin(msg, func() {
                                targetUser, err = helpers.GetUserFromMention(args[2])
                                if targetUser == nil || targetUser.ID == "" {
                                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                                    helpers.Relax(err)
                                    return
                                }

                                settings := helpers.GuildSettingsGetCached(channel.GuildID)

                                for i, ignoredUserID := range settings.LevelsIgnoredUserIDs {
                                    if ignoredUserID == targetUser.ID {
                                        settings.LevelsIgnoredUserIDs = append(settings.LevelsIgnoredUserIDs[:i], settings.LevelsIgnoredUserIDs[i+1:]...)
                                        err = helpers.GuildSettingsSet(channel.GuildID, settings)
                                        helpers.Relax(err)

                                        _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.ignore-user-removed"))
                                        helpers.Relax(err)
                                        return
                                    }
                                }

                                settings.LevelsIgnoredUserIDs = append(settings.LevelsIgnoredUserIDs, targetUser.ID)
                                err = helpers.GuildSettingsSet(channel.GuildID, settings)
                                helpers.Relax(err)

                                _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.ignore-user-added"))
                                helpers.Relax(err)
                                return
                            })
                            return
                        case "channel": // [p]levels ignore channel <channel>
                            if len(args) < 3 {
                                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                                helpers.Relax(err)
                                return
                            }

                            helpers.RequireAdmin(msg, func() {
                                targetChannel, err := helpers.GetChannelFromMention(msg, args[2])
                                if targetChannel == nil || targetChannel.ID == "" {
                                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                                    helpers.Relax(err)
                                    return
                                }

                                settings := helpers.GuildSettingsGetCached(channel.GuildID)

                                for i, ignoredChannelID := range settings.LevelsIgnoredChannelIDs {
                                    if ignoredChannelID == targetChannel.ID {
                                        settings.LevelsIgnoredChannelIDs = append(settings.LevelsIgnoredChannelIDs[:i], settings.LevelsIgnoredChannelIDs[i+1:]...)
                                        err = helpers.GuildSettingsSet(channel.GuildID, settings)
                                        helpers.Relax(err)

                                        _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.ignore-channel-removed"))
                                        helpers.Relax(err)
                                        return
                                    }
                                }

                                settings.LevelsIgnoredChannelIDs = append(settings.LevelsIgnoredChannelIDs, targetChannel.ID)
                                err = helpers.GuildSettingsSet(channel.GuildID, settings)
                                helpers.Relax(err)

                                _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.ignore-channel-added"))
                                helpers.Relax(err)
                                return
                            })
                            return
                        }
                    }
                return
            case "process-history": // [p]level process-history
                helpers.RequireBotAdmin(msg, func() {
                    dmChannel, err := session.UserChannelCreate(msg.Author.ID)
                    helpers.Relax(err)
                    session.ChannelTyping(msg.ChannelID)
                    channel, err := session.Channel(msg.ChannelID)
                    helpers.Relax(err)
                    guild, err := session.Guild(channel.GuildID)
                    helpers.Relax(err)
                    _, err = session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("<@%s> Check your DMs.", msg.Author.ID))
                    // pause new message processing for that guild
                    temporaryIgnoredGuilds = append(temporaryIgnoredGuilds, channel.GuildID)
                    _, err = session.ChannelMessageSend(dmChannel.ID, fmt.Sprintf("Temporary disabled EXP Processing for `%s` while processing the Message History.", guild.Name))
                    helpers.Relax(err)
                    // reset accounts on this server
                    var levelsServersUsers []DB_Levels_ServerUser
                    listCursor, err := rethink.Table("levels_serverusers").Filter(
                        rethink.Row.Field("guildid").Eq(channel.GuildID),
                    ).Run(helpers.GetDB())
                    helpers.Relax(err)
                    defer listCursor.Close()
                    err = listCursor.All(&levelsServersUsers)
                    for _, levelsServerUser := range levelsServersUsers {
                        levelsServerUser.Exp = 0
                        m.setLevelsServerUser(levelsServerUser)
                    }
                    _, err = session.ChannelMessageSend(dmChannel.ID, fmt.Sprintf("Resetted the EXP for every User on `%s`.", guild.Name))
                    helpers.Relax(err)
                    // process history
                    //var wg sync.WaitGroup
                    //wg.Add(len(guild.Channels))
                    for _, guildChannel := range guild.Channels {
                        guildChannelCurrent := guildChannel
                        //go func() {
                        prefix := helpers.GetPrefixForServer(guildChannelCurrent.GuildID)
                        expForUsers := make(map[string]int64)
                        //defer wg.Done()
                        if guildChannelCurrent.Type == "voice" {
                            continue
                        }

                        logger.VERBOSE.L("levels", fmt.Sprintf("Started processing of Channel #%s (#%s) on Guild %s (#%s)",
                            guildChannelCurrent.Name, guildChannelCurrent.ID, guild.Name, guild.ID))
                        // (asynchronous)
                        _, err = session.ChannelMessageSend(dmChannel.ID, fmt.Sprintf("Started processing Messages for Channel <#%s>.", guildChannelCurrent.ID))
                        helpers.Relax(err)
                        lastBefore := ""
                        for {
                            messages, err := session.ChannelMessages(guildChannelCurrent.ID, 100, lastBefore, "", "")
                            if err != nil {
                                logger.ERROR.L("levels", err.Error())
                                break
                            }
                            logger.VERBOSE.L("levels", fmt.Sprintf("Processing %d messages for Channel #%s (#%s) from before \"%s\" on Guild %s (#%s)",
                                len(messages), guildChannelCurrent.Name, guildChannelCurrent.ID, lastBefore, guild.Name, guild.ID))
                            if len(messages) <= 0 {
                                break
                            }
                            for _, message := range messages {
                                // ignore bot messages
                                if message.Author.Bot == true {
                                    continue
                                }
                                // ignore commands
                                if prefix != "" {
                                    if strings.HasPrefix(message.Content, prefix) {
                                        continue
                                    }
                                }
                                if _, ok := expForUsers[message.Author.ID]; ok {
                                    expForUsers[message.Author.ID] += 5
                                } else {
                                    expForUsers[message.Author.ID] = 5
                                }

                            }
                            lastBefore = messages[len(messages)-1].ID
                        }

                        for userId, expForuser := range expForUsers {
                            levelsServerUser := m.getLevelsServerUserOrCreateNew(guildChannelCurrent.GuildID, userId)
                            levelsServerUser.Exp += expForuser
                            m.setLevelsServerUser(levelsServerUser)
                        }

                        logger.VERBOSE.L("levels", fmt.Sprintf("Completed processing of Channel #%s (#%s) on Guild %s (#%s)",
                            guildChannelCurrent.Name, guildChannelCurrent.ID, guild.Name, guild.ID))
                        _, err = session.ChannelMessageSend(dmChannel.ID, fmt.Sprintf("Completed processing Messages for Channel <#%s>.", guildChannelCurrent.ID))
                        helpers.Relax(err)
                        //}()
                    }
                    //fmt.Println("Waiting for all channels")
                    //wg.Wait()
                    // enable new message processing again
                    var newTemporaryIgnoredGuilds []string
                    for _, temporaryIgnoredGuild := range temporaryIgnoredGuilds {
                        if temporaryIgnoredGuild != channel.GuildID {
                            newTemporaryIgnoredGuilds = append(newTemporaryIgnoredGuilds, temporaryIgnoredGuild)
                        }
                    }
                    temporaryIgnoredGuilds = newTemporaryIgnoredGuilds
                    _, err = session.ChannelMessageSend(dmChannel.ID, fmt.Sprintf("Enabled EXP Processing for `%s` again.", guild.Name))
                    helpers.Relax(err)
                    _, err = session.ChannelMessageSend(dmChannel.ID, "Done!")
                    helpers.Relax(err)
                    return
                })
                return
            }
            targetUser, err = helpers.GetUserFromMention(args[0])
            if targetUser == nil || targetUser.ID == "" {
                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                helpers.Relax(err)
                return
            }
        }

        var levelsServersUser []DB_Levels_ServerUser
        listCursor, err := rethink.Table("levels_serverusers").Filter(
            rethink.Row.Field("userid").Eq(targetUser.ID),
        ).Run(helpers.GetDB())
        helpers.Relax(err)
        defer listCursor.Close()
        err = listCursor.All(&levelsServersUser)

        if err == rethink.ErrEmptyResult {
            _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.level-no-stats"))
            helpers.Relax(err)
            return
        } else if err != nil {
            helpers.Relax(err)
        }

        var levelThisServerUser DB_Levels_ServerUser
        var totalExp int64
        for _, levelsServerUser := range levelsServersUser {
            if levelsServerUser.GuildID == channel.GuildID {
                levelThisServerUser = levelsServerUser
            }
            totalExp += levelsServerUser.Exp
        }

        if totalExp <= 0 {
            _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.level-no-stats"))
            helpers.Relax(err)
            return
        }

        currentMember, err := session.GuildMember(channel.GuildID, levelThisServerUser.UserID)
        fullUsername := currentMember.User.Username
        if currentMember.Nick != "" {
            fullUsername += " ~ " + currentMember.Nick
        }

        zeroWidthWhitespace, err := strconv.Unquote(`'\u200b'`)
        helpers.Relax(err)

        userLevelEmbed := &discordgo.MessageEmbed{
            Color: 0x0FADED,
            Title: helpers.GetTextF("plugins.levels.user-embed-title", fullUsername),
            //Description: "",
            Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.levels.embed-footer", len(session.State.Guilds))},
            Fields: []*discordgo.MessageEmbedField{
                {
                    Name:   "Level",
                    Value:  strconv.Itoa(m.getLevelFromExp(levelThisServerUser.Exp)),
                    Inline: true,
                },
                {
                    Name:   "Level Progress",
                    Value:  strconv.Itoa(m.getProgressToNextLevelFromExp(levelThisServerUser.Exp)) + " %",
                    Inline: true,
                },
                {
                    Name:   zeroWidthWhitespace,
                    Value:  zeroWidthWhitespace,
                    Inline: true,
                },
                {
                    Name:   "Global Level",
                    Value:  strconv.Itoa(m.getLevelFromExp(totalExp)),
                    Inline: true,
                },
                {
                    Name:   "Global Level Progress",
                    Value:  strconv.Itoa(m.getProgressToNextLevelFromExp(totalExp)) + " %",
                    Inline: true,
                },
                {
                    Name:   zeroWidthWhitespace,
                    Value:  zeroWidthWhitespace,
                    Inline: true,
                },
            },
        }

        _, err = session.ChannelMessageSendEmbed(msg.ChannelID, userLevelEmbed)
        helpers.Relax(err)
        return
    }

}

func (l *Levels) InsertNewProfileBackground(backgroundName string, backgroundUrl string) error {
    newEntry := new(DB_Profile_Background)
    newEntry.Name = backgroundName
    newEntry.URL = backgroundUrl

    insert := rethink.Table("profile_backgrounds").Insert(newEntry)
    _, err := insert.RunWrite(helpers.GetDB())
    return err
}

func (l *Levels) ProfileBackgroundNameExists(backgroundName string) bool {
    var entryBucket DB_Profile_Background
    listCursor, err := rethink.Table("profile_backgrounds").Filter(
        rethink.Row.Field("id").Eq(backgroundName),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.One(&entryBucket)

    if err == rethink.ErrEmptyResult {
        return false
    } else if err != nil {
        helpers.Relax(err)
    }

    return true
}

func (l *Levels) GetProfileBackgroundUrl(backgroundName string) string {
    var entryBucket DB_Profile_Background
    listCursor, err := rethink.Table("profile_backgrounds").Filter(
        rethink.Row.Field("id").Eq(backgroundName),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.One(&entryBucket)

    if err == rethink.ErrEmptyResult {
        return "http://i.imgur.com/I9b74U9.jpg" // Default Robyul Background
    } else if err != nil {
        helpers.Relax(err)
    }

    return entryBucket.URL
}

func (l *Levels) GetUserUserdata(user *discordgo.User) DB_Profile_Userdata {
    var entryBucket DB_Profile_Userdata
    listCursor, err := rethink.Table("profile_userdata").Filter(
        rethink.Row.Field("userid").Eq(user.ID),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.One(&entryBucket)

    if err == rethink.ErrEmptyResult {
        entryBucket.UserID = user.ID
        insert := rethink.Table("profile_userdata").Insert(entryBucket)
        _, err := insert.RunWrite(helpers.GetDB())
        if err != nil {
            helpers.Relax(err)
        } else {
            return l.GetUserUserdata(user)
        }
        return entryBucket
    } else if err != nil {
        helpers.Relax(err)
    }

    return entryBucket
}

func (l *Levels) setUserUserdata(entry DB_Profile_Userdata) {
    _, err := rethink.Table("profile_userdata").Update(entry).Run(helpers.GetDB())
    helpers.Relax(err)
}

func (m *Levels) GetProfile(member *discordgo.Member, guild *discordgo.Guild) ([]byte, error) {
    tempTemplatePath := cachePath + strconv.FormatInt(time.Now().UnixNano(), 10) + member.User.ID + ".html"

    var levelsServersUser []DB_Levels_ServerUser
    listCursor, err := rethink.Table("levels_serverusers").Filter(
        rethink.Row.Field("userid").Eq(member.User.ID),
    ).Run(helpers.GetDB())
    helpers.Relax(err)
    defer listCursor.Close()
    err = listCursor.All(&levelsServersUser)

    var levelThisServerUser DB_Levels_ServerUser
    var totalExp int64
    for _, levelsServerUser := range levelsServersUser {
        if levelsServerUser.GuildID == guild.ID {
            levelThisServerUser = levelsServerUser
        }
        totalExp += levelsServerUser.Exp
    }

    userData := m.GetUserUserdata(member.User)

    avatarUrl := helpers.GetAvatarUrl(member.User)
    if avatarUrl != "" {
        avatarUrl = strings.Replace(avatarUrl, "gif", "jpg", -1)
        avatarUrl = strings.Replace(avatarUrl, "size=1024", "size=256", -1)
    }
    if avatarUrl == "" {
        avatarUrl = "http://i.imgur.com/osAqNL6.png"
    }
    userAndNick := member.User.Username
    if member.Nick != "" {
        userAndNick = fmt.Sprintf("%s (%s)", member.User.Username, member.Nick)
    }
    title := userData.Title
    if title == "" {
        title = "Robyul's friend"
    }
    bio := userData.Bio
    if bio == "" {
        bio = "Robyul would like to know more about me!"
    }

    tempTemplateHtml := strings.Replace(htmlTemplateString,"{USER_USERNAME}", member.User.Username, -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml,"{USER_NICKNAME}", member.Nick, -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml,"{USER_AND_NICKNAME}", userAndNick, -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml,"{USER_AVATAR_URL}", avatarUrl, -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml,"{USER_TITLE}", title, -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml,"{USER_BIO}", bio, -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml,"{USER_SERVER_LEVEL}", strconv.Itoa(m.getLevelFromExp(levelThisServerUser.Exp)), -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml,"{USER_GLOBAL_LEVEL}", strconv.Itoa(m.getLevelFromExp(totalExp)), -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml,"{USER_BACKGROUND_URL}", m.GetProfileBackgroundUrl(userData.Background), -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml,"{USER_REP}", "WIP", -1) // TODO: <-
    tempTemplateHtml = strings.Replace(tempTemplateHtml,"{TIME}", "WIP", -1) // TODO: <-
    tempTemplateHtml = strings.Replace(tempTemplateHtml,"{USER_BDAY}", "WIP", -1) // TODO: <-
    tempTemplateHtml = strings.Replace(tempTemplateHtml,"{USER_BIO}", "Work In Progress", -1) // TODO: <-

    err = ioutil.WriteFile(tempTemplatePath, []byte(tempTemplateHtml), 0644)
    if err != nil {
        return []byte{}, err
    }

    cmdArgs := []string{
        tempTemplatePath,
        "--window-size=400/300",
        "--default-white-background",
        //"--quality=99",
        "--stream-type=png",
        "--timeout=100000",
        "--p:disk-cache=true",
    }
    imgCmd := exec.Command(webshotBinary, cmdArgs...)
    imgCmd.Env = levelsEnv
    imageBytes, err := imgCmd.Output()
    if err != nil {
        return []byte{}, err
    }

    err = os.Remove(tempTemplatePath)
    if err != nil {
        return []byte{}, err
    }

    metrics.LevelImagesGenerated.Add(1)

    return imageBytes, nil
}

func (m *Levels) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {
    go m.ProcessMessage(msg, session)
}

func (m *Levels) ProcessMessage(msg *discordgo.Message, session *discordgo.Session) {
    channel, err := session.State.Channel(msg.ChannelID)
    helpers.Relax(err)
    // ignore temporary ignored guilds
    for _, temporaryIgnoredGuild := range temporaryIgnoredGuilds {
        if temporaryIgnoredGuild == channel.GuildID {
            return
        }
    }
    // ignore bot messages
    if msg.Author.Bot == true {
        return
    }
    // ignore commands
    prefix := helpers.GetPrefixForServer(channel.GuildID)
    if prefix != "" {
        if strings.HasPrefix(msg.Content, prefix) {
            return
        }
    }

    settings := helpers.GuildSettingsGetCached(channel.GuildID)
    for _, ignoredChannelID := range settings.LevelsIgnoredChannelIDs {
        if ignoredChannelID == msg.ChannelID {
            return
        }
    }
    for _, ignoredUserID := range settings.LevelsIgnoredUserIDs {
        for ignoredUserID == msg.Author.ID {
            return
        }
    }

    // check if bucket is empty
    if !m.BucketHasKeys(channel.GuildID + msg.Author.ID) {
        //m.BucketSet(channel.GuildID+msg.Author.ID, -1)
        return
    }

    err = m.BucketDrain(1, channel.GuildID+msg.Author.ID)
    helpers.Relax(err)

    expStack.Push(ProcessExpInfo{UserID: msg.Author.ID, GuildID: channel.GuildID})
}

func (m *Levels) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {

}

func (m *Levels) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {

}

func (m *Levels) getLevelsServerUserOrCreateNew(guildid string, userid string) DB_Levels_ServerUser {
    var levelsServerUser DB_Levels_ServerUser
    listCursor, err := rethink.Table("levels_serverusers").GetAllByIndex(
        "userid", userid,
    ).Filter(
        rethink.Row.Field("guildid").Eq(guildid),
    ).Run(helpers.GetDB())
    helpers.Relax(err)
    defer listCursor.Close()
    err = listCursor.One(&levelsServerUser)

    if err == rethink.ErrEmptyResult {
        insert := rethink.Table("levels_serverusers").Insert(DB_Levels_ServerUser{GuildID: guildid, UserID: userid})
        _, e := insert.RunWrite(helpers.GetDB())
        if e != nil {
            panic(e)
        } else {
            return m.getLevelsServerUserOrCreateNew(guildid, userid)
        }
    } else if err != nil {
        panic(err)
    }

    return levelsServerUser
}

func (m *Levels) setLevelsServerUser(entry DB_Levels_ServerUser) {
    _, err := rethink.Table("levels_serverusers").Get(entry.ID).Update(entry).Run(helpers.GetDB())
    helpers.Relax(err)
}

func (m *Levels) getLevelFromExp(exp int64) int {
    calculatedLevel := 0.1 * math.Sqrt(float64(exp))

    return int(math.Floor(calculatedLevel))
}

func (m *Levels) getExpForLevel(level int) int64 {
    if level <= 0 {
        return 0
    }

    calculatedExp := math.Pow(float64(level)/0.1, 2)
    return int64(calculatedExp)
}

func (m *Levels) getProgressToNextLevelFromExp(exp int64) int {
    expLevelCurrently := exp - m.getExpForLevel(m.getLevelFromExp(exp))
    expLevelNext := m.getExpForLevel(m.getLevelFromExp(exp) + 1) - m.getExpForLevel(m.getLevelFromExp(exp))
    return int(expLevelCurrently / (expLevelNext / 100))
}

func (m *Levels) getRandomExpForMessage() int64 {
    min := 10
    max := 15
    rand.Seed(time.Now().Unix())
    return int64(rand.Intn(max-min) + min)
}

func (m *Levels) rankMapByExp(exp map[string]int64) PairList {
    pl := make(PairList, len(exp))
    i := 0
    for k, v := range exp {
        pl[i] = Pair{k, v}
        i++
    }
    sort.Sort(sort.Reverse(pl))
    return pl
}

type Pair struct {
    Key   string
    Value int64
}

type PairList []Pair

func (p PairList) Len() int           { return len(p) }
func (p PairList) Less(i, j int) bool { return p[i].Value < p[j].Value }
func (p PairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func (b *Levels) BucketInit() {
    b.Lock()
    b.buckets = make(map[string]int8)
    b.Unlock()

    go b.BucketRefiller()
}

// Refills user buckets in a set interval
func (b *Levels) BucketRefiller() {
    for {
        b.Lock()
        for user, keys := range b.buckets {
            // Chill zone
            if keys == -1 {
                b.buckets[user]++
                continue
            }

            // Chill zone exit
            if keys == 0 {
                b.buckets[user] = BUCKET_INITIAL_FILL
                continue
            }

            // More free keys for nice users :3
            if keys < BUCKET_UPPER_BOUND {
                b.buckets[user] += DROP_SIZE
                continue
            }
        }
        b.Unlock()

        time.Sleep(DROP_INTERVAL)
    }
}

// Check if the user has a bucket. If not create one
func (b *Levels) CreateBucketIfNotExists(user string) {
    if b.buckets == nil {
        return
    }

    b.RLock()
    _, e := b.buckets[user]
    b.RUnlock()

    if !e {
        b.Lock()
        b.buckets[user] = BUCKET_INITIAL_FILL
        b.Unlock()
    }
}

// Drains $amount from $user if he has enough keys left
func (b *Levels) BucketDrain(amount int8, user string) error {
    b.CreateBucketIfNotExists(user)

    // Check if there are enough keys left
    b.RLock()
    userAmount := b.buckets[user]
    b.RUnlock()

    if amount > userAmount {
        return errors.New("No keys left")
    }

    // Remove keys from bucket
    b.Lock()
    b.buckets[user] -= amount
    b.Unlock()

    return nil
}

// Check if the user still has keys
func (b *Levels) BucketHasKeys(user string) bool {
    b.CreateBucketIfNotExists(user)

    b.RLock()
    defer b.RUnlock()

    return b.buckets[user] > 0
}

func (b *Levels) BucketGet(user string) int8 {
    b.RLock()
    defer b.RUnlock()

    return b.buckets[user]
}

func (b *Levels) BucketSet(user string, value int8) {
    if b.buckets == nil {
        return
    }

    b.Lock()
    b.buckets[user] = value
    b.Unlock()
}

func (b *Levels) OnReactionAdd(reaction *discordgo.MessageReactionAdd, session *discordgo.Session) {

}
func (b *Levels) OnReactionRemove(reaction *discordgo.MessageReactionRemove, session *discordgo.Session) {

}
func (b *Levels) OnGuildBanAdd(user *discordgo.GuildBanAdd, session *discordgo.Session) {

}
func (b *Levels) OnGuildBanRemove(user *discordgo.GuildBanRemove, session *discordgo.Session) {

}
