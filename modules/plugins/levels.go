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
    "github.com/dustin/go-humanize"
    "strconv"
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
)

func (m *Levels) Commands() []string {
    return []string{
        "level",
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

func (m *Levels) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    switch command {
    case "level": // [p]level <user> or [p]level top
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
                    //Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.stats.voicestats-embed-footer")},
                    Fields: []*discordgo.MessageEmbedField{},
                }

                i := 0
                for _, levelsServersUser := range levelsServersUsers {
                    currentMember, err := session.GuildMember(channel.GuildID, levelsServersUser.UserID)
                    fullUsername := currentMember.User.Username
                    if currentMember.Nick != "" {
                        fullUsername += " ~ " + currentMember.Nick
                    }
                    helpers.Relax(err)
                    topLevelEmbed.Fields = append(topLevelEmbed.Fields, &discordgo.MessageEmbedField{
                        Name:   fmt.Sprintf("#%d: %s", i+1, fullUsername),
                        Value:  fmt.Sprintf("Level: %d, Experience: %d EXP", m.getLevelFromExp(levelsServersUser.Exp), levelsServersUser.Exp),
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
            //Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.stats.voicestats-embed-footer")},
            Fields: []*discordgo.MessageEmbedField{
                &discordgo.MessageEmbedField{
                    Name:   "Level",
                    Value:  strconv.Itoa(m.getLevelFromExp(levelThisServerUser.Exp)),
                    Inline: true,
                },
                &discordgo.MessageEmbedField{
                    Name:   "Experience",
                    Value:  humanize.Comma(levelThisServerUser.Exp) + " EXP",
                    Inline: true,
                },
                &discordgo.MessageEmbedField{
                    Name:   ":white_circle: ",
                    Value:  ":white_circle: ",
                    Inline: true,
                },
                &discordgo.MessageEmbedField{
                    Name:   "Global Level",
                    Value:  strconv.Itoa(m.getLevelFromExp(totalExp)),
                    Inline: true,
                },
                &discordgo.MessageEmbedField{
                    Name:   "Global Experience",
                    Value:  humanize.Comma(totalExp) + " EXP",
                    Inline: true,
                },
                &discordgo.MessageEmbedField{
                    Name:   ":white_circle: ",
                    Value:  ":white_circle: ",
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
    channel, err := session.Channel(msg.ChannelID)
    helpers.Relax(err)
    // ignore bot messages
    if msg.Author.Bot == true {
        return
    }
    // ignore commands
    prefix := helpers.GetPrefixForServer(channel.GuildID)
    if prefix != "" {
        if strings.HasPrefix(content, prefix) {
            return
        }
    }
    // check if bucket is empty
    if !m.BucketHasKeys(channel.GuildID + msg.Author.ID) {
        m.BucketSet(channel.GuildID+msg.Author.ID, -1)
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
