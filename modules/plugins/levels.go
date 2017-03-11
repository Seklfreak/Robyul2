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
    "github.com/bradfitz/slice"
    "strconv"
    "github.com/fogleman/gg"
    "github.com/Seklfreak/Robyul2/logger"
    "bytes"
    "net/http"
    "image"
    "image/gif"
    "image/jpeg"
    "github.com/nfnt/resize"
    "github.com/Seklfreak/Robyul2/metrics"
)

type Levels struct {
    sync.RWMutex

    buckets map[string]int8
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

func (m *Levels) Init(session *discordgo.Session) {
    m.BucketInit()
}

// @TODO: Global Top 10
func (m *Levels) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    switch command {
    case "profile": // [p]profile
        session.ChannelTyping(msg.ChannelID)
        channel, err := session.Channel(msg.ChannelID)
        helpers.Relax(err)
        targetUser, err := session.User(msg.Author.ID)
        helpers.Relax(err)
        helpers.Relax(err)
        targetMember, err := session.GuildMember(channel.GuildID, targetUser.ID)
        args := strings.Split(content, " ")
        if len(args) >= 1 && args[0] != "" {
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

        var levelThisServerUser DB_Levels_ServerUser
        var totalExp int64
        for _, levelsServerUser := range levelsServersUser {
            if levelsServerUser.GuildID == channel.GuildID {
                levelThisServerUser = levelsServerUser
            }
            totalExp += levelsServerUser.Exp
        }

        avatarUrl := helpers.GetAvatarUrl(targetUser)

        client := &http.Client{}
        request, err := http.NewRequest("GET", avatarUrl, nil)
        if err != nil {
            panic(err)
        }
        request.Header.Set("User-Agent", helpers.DEFAULT_UA)
        response, err := client.Do(request)
        helpers.Relax(err)
        defer response.Body.Close()

        var avatarImage image.Image

        if strings.Contains(avatarUrl, ".gif") {
            avatarImage, err = gif.Decode(response.Body)
            helpers.Relax(err)
        } else {
            avatarImage, err = jpeg.Decode(response.Body)
            helpers.Relax(err)
        }

        usernameText := strings.ToUpper(targetUser.Username)
        if targetMember.Nick != "" {
            usernameText += fmt.Sprintf(" (%s)", targetMember.Nick)
        }

        dc := gg.NewContext(300, 300)
        // load fonts
        // @TODO: Asset path
        err = dc.LoadFontFace("_assets/2593-UnDotum.ttf", 20)
        helpers.Relax(err)
        // draw grey background
        //dc.SetRGBA255(0, 0, 0, 32)
        dc.SetRGB255(230, 230, 230)
        dc.Clear()
        // draw username box + username
        dc.DrawRectangle(50, 89, 245, 22)
        dc.SetRGB255(100, 100, 100)
        dc.Fill()
        dc.SetRGB255(255, 255, 255)
        dc.DrawStringAnchored(usernameText, 100, 107, 0, 0)
        // draw user title
        dc.DrawRectangle(95, 111, 200, 22)
        dc.SetRGBA255(100, 100, 100, 128)
        dc.Fill()
        dc.SetRGB255(255, 255, 255)
        dc.DrawStringAnchored(strings.ToUpper("<USER TITLE>"), 100, 129, 0, 0)
        // draw round user profile picture
        dc.DrawCircle(50, 90, 44)
        dc.SetRGB255(100, 100, 100)
        dc.Fill()
        avatarImage = resize.Resize(80, 80, avatarImage, resize.NearestNeighbor)
        dc.DrawCircle(50, 90, 40)
        dc.Clip()
        dc.DrawImage(avatarImage, 10, 50)
        dc.ResetClip()
        // draw levels
        dc.DrawRectangle(95, 135, 200, 22)
        dc.SetRGBA255(100, 100, 100, 128)
        dc.Fill()
        dc.SetRGB255(255, 255, 255)
        err = dc.LoadFontFace("_assets/2593-UnDotum.ttf", 8)
        helpers.Relax(err)
        dc.DrawStringAnchored(strings.ToUpper("Level"), 97, 143, 0, 0)
        err = dc.LoadFontFace("_assets/Roboto/Roboto-Bold.ttf", 15)
        helpers.Relax(err)
        dc.DrawStringAnchored(strconv.Itoa(m.getLevelFromExp(levelThisServerUser.Exp)), 106.5, 155, 0.5, 0)
        dc.DrawRectangle(121, 137, 73, 18)
        dc.SetRGBA255(100, 100, 100, 128)
        dc.Fill()
        dc.DrawRectangle(121, 137, float64(73)/float64(100)*float64(m.getProgressToNextLevelFromExp(levelThisServerUser.Exp)), 18)
        dc.SetRGBA255(65, 125, 100, 215)
        dc.Fill()
        dc.SetRGB255(255, 255, 255)
        err = dc.LoadFontFace("_assets/2593-UnDotum.ttf", 8)
        helpers.Relax(err)
        dc.DrawStringAnchored(strings.ToUpper("Global"), 196.5, 143, 0, 0)
        err = dc.LoadFontFace("_assets/Roboto/Roboto-Bold.ttf", 15)
        helpers.Relax(err)
        dc.DrawStringAnchored(strconv.Itoa(m.getLevelFromExp(totalExp)), 210.5, 155, 0.5, 0)
        dc.DrawRectangle(226, 137, 67, 18)
        dc.SetRGBA255(100, 100, 100, 128)
        dc.Fill()
        dc.DrawRectangle(226, 137, float64(67)/float64(100)*float64(m.getProgressToNextLevelFromExp(totalExp)), 18)
        dc.SetRGBA255(65, 125, 100, 215)
        dc.Fill()

        var buffer bytes.Buffer
        err = dc.EncodePNG(&buffer)
        helpers.Relax(err)

        _, err = session.ChannelFileSendWithMessage(
            msg.ChannelID,
            fmt.Sprintf("Profile for %s", targetUser.Username),
            fmt.Sprintf("%s.png", targetUser.ID), bytes.NewReader(buffer.Bytes()))
        helpers.Relax(err)

        metrics.LevelImagesGenerated.Add(1)

        return
    case "level", "levels": // [p]level <user> or [p]level top
        session.ChannelTyping(msg.ChannelID)
        targetUser, err := session.User(msg.Author.ID)
        helpers.Relax(err)
        args := strings.Split(content, " ")

        channel, err := session.Channel(msg.ChannelID)
        helpers.Relax(err)

        if len(args) >= 1 && args[0] != "" {
            switch args[0] {
            case "leaderboard", "top": // [p]level top
                var levelsServersUsers []DB_Levels_ServerUser
                listCursor, err := rethink.Table("levels_serverusers").Filter(
                    rethink.Row.Field("guildid").Eq(channel.GuildID),
                ).Run(helpers.GetDB())
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

                slice.Sort(levelsServersUsers, func(i, j int) bool {
                    return levelsServersUsers[i].Exp > levelsServersUsers[j].Exp
                })

                topLevelEmbed := &discordgo.MessageEmbed{
                    Color: 0x0FADED,
                    Title: helpers.GetText("plugins.levels.top-server-embed-title"),
                    //Description: "",
                    Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.levels.embed-footer", len(session.State.Guilds))},
                    Fields: []*discordgo.MessageEmbedField{},
                }

                i := 0
                for _, levelsServersUser := range levelsServersUsers {
                    currentMember, err := session.GuildMember(channel.GuildID, levelsServersUser.UserID)
                    if err != nil {
                        continue
                    }
                    fullUsername := currentMember.User.Username
                    if currentMember.Nick != "" {
                        fullUsername += " ~ " + currentMember.Nick
                    }
                    helpers.Relax(err)
                    topLevelEmbed.Fields = append(topLevelEmbed.Fields, &discordgo.MessageEmbedField{
                        Name:   fmt.Sprintf("#%d: %s", i+1, fullUsername),
                        Value:  fmt.Sprintf("Level: %d", m.getLevelFromExp(levelsServersUser.Exp)),
                        Inline: false,
                    })
                    i++
                    if i >= 10 {
                        break
                    }
                }

                _, err = session.ChannelMessageSendEmbed(msg.ChannelID, topLevelEmbed)
                helpers.Relax(err)
                return
            case "process-history": // [p]level process-history
                helpers.RequireBotAdmin(msg, func() {
                    session.ChannelTyping(msg.ChannelID)
                    channel, err := session.Channel(msg.ChannelID)
                    helpers.Relax(err)
                    guild, err := session.Guild(channel.GuildID)
                    helpers.Relax(err)
                    // pause new message processing for that guild
                    temporaryIgnoredGuilds = append(temporaryIgnoredGuilds, channel.GuildID)
                    _, err = session.ChannelMessageSend(msg.ChannelID, "Temporary disabled EXP Processing for this server while processing the Message History.")
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
                    _, err = session.ChannelMessageSend(msg.ChannelID, "Resetted the EXP for every User on this server.")
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
                        _, err = session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Started processing Messages for Channel <#%s>.", guildChannelCurrent.ID))
                        helpers.Relax(err)
                        lastBefore := ""
                        for {
                            messages, err := session.ChannelMessages(guildChannelCurrent.ID, 100, lastBefore, "")
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
                        _, err = session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Completed processing Messages for Channel <#%s>.", guildChannelCurrent.ID))
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
                    _, err = session.ChannelMessageSend(msg.ChannelID, "Enabled EXP Processing for this server again.")
                    helpers.Relax(err)
                    _, err = session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("<@%s> Done!", msg.Author.ID))
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

        userLevelEmbed := &discordgo.MessageEmbed{
            Color: 0x0FADED,
            Title: helpers.GetTextF("plugins.levels.user-embed-title", fullUsername),
            //Description: "",
            Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.levels.embed-footer", len(session.State.Guilds))},
            Fields: []*discordgo.MessageEmbedField{
                &discordgo.MessageEmbedField{
                    Name:   "Level",
                    Value:  strconv.Itoa(m.getLevelFromExp(levelThisServerUser.Exp)),
                    Inline: true,
                },
                &discordgo.MessageEmbedField{
                    Name:   "Level Progress",
                    Value:  strconv.Itoa(m.getProgressToNextLevelFromExp(levelThisServerUser.Exp)) + " %",
                    Inline: true,
                },
                &discordgo.MessageEmbedField{
                    Name:   ":white_circle:",
                    Value:  ":white_circle:",
                    Inline: true,
                },
                &discordgo.MessageEmbedField{
                    Name:   "Global Level",
                    Value:  strconv.Itoa(m.getLevelFromExp(totalExp)),
                    Inline: true,
                },
                &discordgo.MessageEmbedField{
                    Name:   "Global Level Progress",
                    Value:  strconv.Itoa(m.getProgressToNextLevelFromExp(totalExp)) + " %",
                    Inline: true,
                },
                &discordgo.MessageEmbedField{
                    Name:   ":white_circle:",
                    Value:  ":white_circle:",
                    Inline: true,
                },
            },
        }

        _, err = session.ChannelMessageSendEmbed(msg.ChannelID, userLevelEmbed)
        helpers.Relax(err)
        return
    }

}

func (m *Levels) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {
    m.ProcessMessage(msg, session)
}

func (m *Levels) ProcessMessage(msg *discordgo.Message, session *discordgo.Session) {
    channel, err := session.Channel(msg.ChannelID)
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
    // check if bucket is empty
    if !m.BucketHasKeys(channel.GuildID + msg.Author.ID) {
        //m.BucketSet(channel.GuildID+msg.Author.ID, -1)
        return
    }

    err = m.BucketDrain(1, channel.GuildID+msg.Author.ID)
    helpers.Relax(err)

    levelsServerUser := m.getLevelsServerUserOrCreateNew(channel.GuildID, msg.Author.ID)
    levelsServerUser.Exp += m.getRandomExpForMessage()
    m.setLevelsServerUser(levelsServerUser)
}

func (m *Levels) OnGuildMemberAdd(member *discordgo.Member, session *discordgo.Session) {

}

func (m *Levels) OnGuildMemberRemove(member *discordgo.Member, session *discordgo.Session) {

}

func (m *Levels) getLevelsServerUserOrCreateNew(guildid string, userid string) DB_Levels_ServerUser {
    var levelsServerUser DB_Levels_ServerUser
    listCursor, err := rethink.Table("levels_serverusers").Filter(
        rethink.Row.Field("guildid").Eq(guildid),
    ).Filter(
        rethink.Row.Field("userid").Eq(userid),
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
    _, err := rethink.Table("levels_serverusers").Update(entry).Run(helpers.GetDB())
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
    expLevelNext := m.getExpForLevel((m.getLevelFromExp(exp) + 1)) - m.getExpForLevel(m.getLevelFromExp(exp))
    return int(expLevelCurrently / (expLevelNext / 100))
}

func (m *Levels) getRandomExpForMessage() int64 {
    min := 10
    max := 15
    rand.Seed(time.Now().Unix())
    return int64(rand.Intn(max-min) + min)
}

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
    b.Lock()
    b.buckets[user] = value
    b.Unlock()
}
