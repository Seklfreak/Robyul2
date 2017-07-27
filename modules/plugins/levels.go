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
    "github.com/Seklfreak/Robyul2/cache"
    "net/url"
    "net/http"
    "encoding/json"
    "encoding/base64"
    "github.com/bradfitz/slice"
    "github.com/lucasb-eyer/go-colorful"
    "image"
    "image/png"
    "image/draw"
    "github.com/nfnt/resize"
    "image/color"
    "github.com/getsentry/raven-go"
    "github.com/andybons/gogif"
    "image/gif"
    "html"
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
        "rep",
        "gif-profile",
    }
}

type DB_Levels_ServerUser struct {
    ID      string  `gorethink:"id,omitempty"`
    UserID  string  `gorethink:"userid"`
    GuildID string  `gorethink:"guildid"`
    Exp     int64   `gorethink:"exp"`
}

type DB_Profile_Background struct {
    Name      string  `gorethink:"id,omitempty"`
    URL       string  `gorethink:"url"`
    CreatedAt time.Time  `gorethink:"createdat"`
}

type DB_Profile_Userdata struct {
    ID                string     `gorethink:"id,omitempty"`
    UserID            string     `gorethink:"userid"`
    Background        string     `gorethink:"background"`
    Title             string     `gorethink:"title"`
    Bio               string     `gorethink:"bio"`
    Rep               int        `gorethink:"rep"`
    LastRepped        time.Time  `gorethink:"last_repped"`
    ActiveBadgeIDs    []string   `gorethink:"active_badgeids"`
    BackgroundColor   string    `gorethink:"background_color"`
    AccentColor       string        `gorethink:"accent_color"`
    TextColor         string          `gorethink:"text_color"`
    BackgroundOpacity string  `gorethink:"background_opacity"`
    DetailOpacity     string  `gorethink:"detail_opacity"`
    Timezone          string `gorethink:"timezone"`
    Birthday          string `gorethink:"birthday"`
}

type DB_Badge struct {
    ID               string     `gorethink:"id,omitempty"`
    CreatedByUserID  string     `gorethink:"createdby_userid"`
    Name             string     `gorethink:"name"`
    Category         string     `gorethink:"category"`
    BorderColor      string     `gorethink:"bordercolor"`
    GuildID          string     `gorethink:"guildid"`
    CreatedAt        time.Time  `gorethink:"createdat"`
    URL              string     `gorethink:"url"`
    LevelRequirement int        `gorethink:"levelrequirement"`
    AllowedUserIDs   []string   `gorethinK:"allowed_userids"`
    DeniedUserIDs    []string   `gorethinK:"allowed_userids"`
}

type Cache_Levels_top struct {
    GuildID string
    Levels  PairList
}

var (
    cachePath                string
    assetsPath               string
    htmlTemplateString       string
    levelsEnv                []string = os.Environ()
    webshotBinary            string
    topCache                 []Cache_Levels_top
    activeBadgePickerUserIDs map[string]string
)

const (
    BadgeLimt int = 12
    TimeAtUserFormat string = "Mon, 15:04"
    TimeBirthdayFormat string = "01/02"
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
    logger.PLUGIN.L("levels", "Started processExpStackLoop")

    go m.cacheTopLoop()
    logger.PLUGIN.L("levels", "Started processCacheTopLoop")

    activeBadgePickerUserIDs = make(map[string]string, 0)
}

func (m *Levels) cacheTopLoop() {
    defer func() {
        helpers.Recover()

        logger.ERROR.L("levels", "The cacheTopLoop died. Please investigate! Will be restarted in 60 seconds")
        time.Sleep(60 * time.Second)
        m.cacheTopLoop()
    }()

    for {
        var newTopCache []Cache_Levels_top

        var levelsUsers []DB_Levels_ServerUser
        listCursor, err := rethink.Table("levels_serverusers").Run(helpers.GetDB())
        helpers.Relax(err)
        defer listCursor.Close()
        err = listCursor.All(&levelsUsers)

        if err == rethink.ErrEmptyResult || len(levelsUsers) <= 0 {
            logger.ERROR.L("levels", "empty result from levels db")
            time.Sleep(60 * time.Second)
            continue
        } else if err != nil {
            logger.ERROR.L("levels", fmt.Sprintf("db error: %s", err.Error()))
            time.Sleep(60 * time.Second)
            continue
        }

        for _, guild := range cache.GetSession().State.Guilds {
            guildExpMap := make(map[string]int64, 0)
            for _, levelsUser := range levelsUsers {
                if levelsUser.GuildID == guild.ID {
                    guildExpMap[levelsUser.UserID] = levelsUser.Exp
                }
            }
            rankedGuildExpMap := m.rankMapByExp(guildExpMap)
            newTopCache = append(newTopCache, Cache_Levels_top{
                GuildID: guild.ID,
                Levels:  rankedGuildExpMap,
            })
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
        newTopCache = append(newTopCache, Cache_Levels_top{
            GuildID: "global",
            Levels:  rankedTotalExpMap,
        })

        topCache = newTopCache

        time.Sleep(30 * time.Minute)
    }
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
    case "rep": // [p]rep <user id/mention>
        session.ChannelTyping(msg.ChannelID)
        args := strings.Fields(content)
        if len(args) <= 0 {
            _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
            helpers.Relax(err)
            return
        }

        userData := m.GetUserUserdata(msg.Author)
        if time.Since(userData.LastRepped).Hours() < 12 {
            timeUntil := time.Until(userData.LastRepped.Add(time.Hour * 12))
            _, err := session.ChannelMessageSend(msg.ChannelID,
                helpers.GetTextF("plugins.levels.rep-error-timelimit",
                    int(math.Floor(timeUntil.Hours())),
                    int(math.Floor(timeUntil.Minutes()))-(int(math.Floor(timeUntil.Hours()))*60)))
            helpers.Relax(err)
            return
        }

        targetUser, err := helpers.GetUserFromMention(args[0])
        if err != nil || targetUser == nil || targetUser.ID == "" {
            _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
            helpers.Relax(err)
            return
        }

        // Don't rep this bot account, other bots, or oneself
        if targetUser.ID == session.State.User.ID {
            _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.rep-error-session"))
            helpers.Relax(err)
            return
        }
        if targetUser.ID == msg.Author.ID {
            _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.rep-error-self"))
            helpers.Relax(err)
            return
        }
        if targetUser.Bot == true {
            _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.rep-error-bot"))
            helpers.Relax(err)
            return
        }

        targetUserData := m.GetUserUserdata(targetUser)
        targetUserData.Rep += 1
        m.setUserUserdata(targetUserData)

        userData.LastRepped = time.Now()
        m.setUserUserdata(userData)

        _, err = session.ChannelMessageSend(msg.ChannelID,
            helpers.GetTextF("plugins.levels.rep-success", targetUser.Username))
        helpers.Relax(err)
        return
    case "profile", "gif-profile": // [p]profile
        if _, ok := activeBadgePickerUserIDs[msg.Author.ID]; ok {
            if activeBadgePickerUserIDs[msg.Author.ID] != msg.ChannelID {
                _, err := session.ChannelMessageSend(
                    msg.ChannelID, helpers.GetText("plugins.levels.badge-picker-session-duplicate"))
                helpers.Relax(err)
            }
            return
        }
        session.ChannelTyping(msg.ChannelID)
        channel, err := helpers.GetChannel(msg.ChannelID)
        helpers.Relax(err)
        guild, err := helpers.GetGuild(channel.GuildID)
        helpers.Relax(err)
        targetUser, err := helpers.GetUser(msg.Author.ID)
        helpers.Relax(err)
        args := strings.Fields(content)
        if len(args) >= 1 && args[0] != "" {
            switch args[0] {
            case "title":
                titleText := " "
                if len(args) >= 2 {
                    titleText = strings.TrimSpace(strings.Replace(content, strings.Join(args[:1], " "), "", 1))
                }

                userUserdata := m.GetUserUserdata(msg.Author)
                userUserdata.Title = titleText
                m.setUserUserdata(userUserdata)

                _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.profile-title-set-success"))
                helpers.Relax(err)
                return
            case "bio":
                bioText := " "
                if len(args) >= 2 {
                    bioText = strings.TrimSpace(strings.Replace(content, strings.Join(args[:1], " "), "", 1))
                }

                userUserdata := m.GetUserUserdata(msg.Author)
                userUserdata.Bio = bioText
                m.setUserUserdata(userUserdata)

                _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.profile-bio-set-success"))
                helpers.Relax(err)
                return
            case "background":
                if len(args) < 2 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.new-profile-background-help"))
                    helpers.Relax(err)
                    return
                }
                switch args[1] {
                case "add":
                    helpers.RequireRobyulMod(msg, func() {
                        if len(args) < 4 {
                            _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                            helpers.Relax(err)
                            return
                        }
                        backgroundName := args[2]
                        backgroundUrl := args[3]

                        picData, err := helpers.NetGetUAWithError(backgroundUrl, helpers.DEFAULT_UA)
                        if err != nil {
                            if _, ok := err.(*url.Error); ok {
                                _, err = session.ChannelMessageSend(msg.ChannelID, "Invalid url.")
                                helpers.Relax(err)
                            } else {
                                helpers.Relax(err)
                            }
                            return
                        }
                        backgroundUrl, err = m.uploadToImgur(picData)
                        if err != nil {
                            if strings.Contains(err.Error(), "Invalid URL") {
                                _, err = session.ChannelMessageSend(msg.ChannelID, "I wasn't able to reupload the picture. Please make sure it is a direct link to the image.")
                                helpers.Relax(err)
                            } else {
                                helpers.Relax(err)
                            }
                            return
                        }

                        if m.ProfileBackgroundNameExists(backgroundName) == true {
                            _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.new-profile-background-add-error-duplicate"))
                            return
                        }

                        err = m.InsertNewProfileBackground(backgroundName, backgroundUrl)
                        if err != nil {
                            helpers.Relax(err)
                        }
                        _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.levels.new-profile-background-add-success", backgroundName))
                        helpers.Relax(err)
                        return
                    })
                    return
                case "delete":
                    helpers.RequireRobyulMod(msg, func() {
                        if len(args) < 3 {
                            _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                            helpers.Relax(err)
                            return
                        }
                        backgroundName := strings.ToLower(args[2])

                        if m.ProfileBackgroundNameExists(backgroundName) == false {
                            _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.profile-background-delete-error-not-found"))
                            return
                        }
                        backgroundUrl := m.GetProfileBackgroundUrl(backgroundName)

                        if helpers.ConfirmEmbed(
                            msg.ChannelID, msg.Author, helpers.GetTextF("plugins.levels.profile-background-delete-confirm",
                                backgroundName, backgroundUrl),
                            "✅", "🚫") == true {
                            err = m.DeleteProfileBackground(backgroundName)
                            helpers.Relax(err)

                            _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.profile-background-delete-success"))
                            helpers.Relax(err)
                        }
                        return
                    })
                    return
                default:
                    if m.ProfileBackgroundNameExists(args[1]) == false {
                        searchResult := m.ProfileBackgroundSearch(args[1])

                        if len(searchResult) <= 0 {
                            _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.profile-background-set-error-not-found"))
                            helpers.Relax(err)
                        } else {
                            backgroundNamesText := ""
                            for _, entry := range searchResult {
                                backgroundNamesText += "`" + entry.Name + "` "
                            }
                            backgroundNamesText = strings.TrimSpace(backgroundNamesText)
                            resultText := helpers.GetText("plugins.levels.profile-background-set-error-not-found") + "\n"
                            resultText += fmt.Sprintf("Maybe I can interest you in one of these backgrounds: %s", backgroundNamesText)

                            _, err = session.ChannelMessageSend(msg.ChannelID, resultText)
                            helpers.Relax(err)
                        }
                        return
                    }

                    userUserdata := m.GetUserUserdata(msg.Author)
                    userUserdata.Background = args[1]
                    m.setUserUserdata(userUserdata)

                    _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.profile-background-set-success"))
                    helpers.Relax(err)
                    return
                }
            case "badge", "badges":
                if len(args) >= 2 {
                    switch args[1] {
                    case "create": // [p]profile badge create <category name> <badge name> <image url> <border color> <level req, -1=not available, 0=everyone> [global, botowner only]
                        helpers.RequireAdmin(msg, func() {
                            session.ChannelTyping(msg.ChannelID)
                            if len(args) < 7 {
                                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                                helpers.Relax(err)
                                return
                            }

                            channel, err := helpers.GetChannel(msg.ChannelID)
                            helpers.Relax(err)

                            newBadge := new(DB_Badge)

                            newBadge.CreatedByUserID = msg.Author.ID
                            newBadge.CreatedAt = time.Now()
                            newBadge.Category = strings.ToLower(args[2])
                            newBadge.Name = strings.ToLower(args[3])
                            newBadge.URL = args[4]                                       // reupload to imgur
                            newBadge.BorderColor = strings.Replace(args[5], "#", "", -1) // check if valid color
                            newBadge.LevelRequirement, err = strconv.Atoi(args[6])
                            if err != nil {
                                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                                helpers.Relax(err)
                                return
                            }
                            newBadge.GuildID = channel.GuildID
                            if len(args) >= 8 {
                                if args[7] == "global" {
                                    if helpers.IsBotAdmin(msg.Author.ID) {
                                        newBadge.GuildID = "global"
                                    } else {
                                        _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                                        helpers.Relax(err)
                                        return
                                    }
                                }
                            }
                            picData, err := helpers.NetGetUAWithError(newBadge.URL, helpers.DEFAULT_UA)
                            if err != nil {
                                if _, ok := err.(*url.Error); ok {
                                    _, err = session.ChannelMessageSend(msg.ChannelID, "Invalid url.")
                                    helpers.Relax(err)
                                } else {
                                    helpers.Relax(err)
                                }
                                return
                            }
                            newBadge.URL, err = m.uploadToImgur(picData)
                            if err != nil {
                                if strings.Contains(err.Error(), "Invalid URL") {
                                    _, err = session.ChannelMessageSend(msg.ChannelID, "I wasn't able to reupload the picture. Please make sure it is a direct link to the image.")
                                    helpers.Relax(err)
                                } else {
                                    helpers.Relax(err)
                                }
                                return
                            }

                            badgeFound := m.GetBadge(newBadge.Category, newBadge.Name, channel.GuildID)
                            if badgeFound.ID != "" {
                                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.create-badge-error-duplicate"))
                                helpers.Relax(err)
                                return
                            }

                            serverBadges := m.GetServerOnlyBadges(channel.GuildID)
                            badgeLimit := helpers.GuildSettingsGetCached(channel.GuildID).LevelsMaxBadges
                            if badgeLimit == 0 {
                                badgeLimit = 20
                            }
                            if len(serverBadges) >= badgeLimit {
                                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.create-badge-error-too-many"))
                                helpers.Relax(err)
                                return
                            }

                            m.InsertBadge(*newBadge)

                            _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.create-badge-success"))
                            helpers.Relax(err)
                            return
                        })
                        return
                    case "delete", "remove": // [p]profile badge delete <category name> <badge name>
                        helpers.RequireAdmin(msg, func() {
                            session.ChannelTyping(msg.ChannelID)
                            if len(args) < 4 {
                                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                                helpers.Relax(err)
                                return
                            }
                            channel, err := helpers.GetChannel(msg.ChannelID)
                            helpers.Relax(err)

                            badgeFound := m.GetBadge(args[2], args[3], channel.GuildID)
                            if badgeFound.ID == "" {
                                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.badge-error-not-found"))
                                helpers.Relax(err)
                                return
                            }
                            if badgeFound.GuildID == "global" && !helpers.IsBotAdmin(msg.Author.ID) {
                                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.delete-badge-error-not-allowed"))
                                helpers.Relax(err)
                                return
                            }

                            m.DeleteBadge(badgeFound.ID)

                            _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.delete-badge-success"))
                            helpers.Relax(err)
                            return
                        })
                        return
                    case "list": // [p]profile badge list [<category name>]
                        session.ChannelTyping(msg.ChannelID)
                        if len(args) >= 3 {
                            categoryName := args[2]

                            channel, err := helpers.GetChannel(msg.ChannelID)
                            helpers.Relax(err)

                            categoryBadges := m.GetCategoryBadges(categoryName, channel.GuildID)

                            if len(categoryBadges) <= 0 {
                                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.list-category-badge-error-none"))
                                helpers.Relax(err)
                                return
                            }

                            slice.Sort(categoryBadges, func(i, j int) bool {
                                return categoryBadges[i].Name < categoryBadges[j].Name
                            })

                            resultText := fmt.Sprintf("__**Badges in %s**__\n", categoryName)
                            for _, badge := range categoryBadges {
                                globalText := ""
                                if badge.GuildID == "global" {
                                    globalText = "GLOBAL "
                                }
                                resultText += fmt.Sprintf("**%s%s**: URL: <%s>, Border Color: #%s, Requirement: %d, Allowed Users: %d, Denied Users %d\n",
                                    globalText, badge.Name, badge.URL, badge.BorderColor, badge.LevelRequirement, len(badge.AllowedUserIDs), len(badge.DeniedUserIDs),
                                )
                            }
                            resultText += fmt.Sprintf("I found %d badges in this category.\n",
                                len(categoryBadges))

                            for _, page := range helpers.Pagify(resultText, "\n") {
                                _, err = session.ChannelMessageSend(msg.ChannelID, page)
                                helpers.Relax(err)
                            }
                            return
                        }

                        channel, err := helpers.GetChannel(msg.ChannelID)
                        helpers.Relax(err)

                        serverBadges := m.GetServerBadges(channel.GuildID)

                        if len(serverBadges) <= 0 {
                            _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.list-badge-error-none"))
                            helpers.Relax(err)
                            return
                        }

                        categoriesCount := make(map[string]int, 0)
                        for _, badge := range serverBadges {
                            if _, ok := categoriesCount[badge.Category]; ok {
                                categoriesCount[badge.Category] += 1
                            } else {
                                categoriesCount[badge.Category] = 1
                            }
                        }

                        sortedKeys := make([]string, len(categoriesCount))
                        i := 0
                        for k := range categoriesCount {
                            sortedKeys[i] = k
                            i++
                        }
                        sort.Strings(sortedKeys)

                        resultText := "__**Badge Categories**__\n"
                        for _, key := range sortedKeys {
                            resultText += fmt.Sprintf("**%s** (%d badges)\n", key, categoriesCount[key])
                        }
                        resultText += fmt.Sprintf("I found %d badge categories on this server.\nUse `_profile badge list <category name>` to view all badges of a category.\n",
                            len(categoriesCount))

                        for _, page := range helpers.Pagify(resultText, "\n") {
                            _, err = session.ChannelMessageSend(msg.ChannelID, page)
                            helpers.Relax(err)
                        }
                        return
                    case "allow": // [p]profile badge allow <user id/mention> <category name> <badge name>
                        helpers.RequireMod(msg, func() {
                            session.ChannelTyping(msg.ChannelID)
                            if len(args) < 5 {
                                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                                helpers.Relax(err)
                                return
                            }

                            targetUser, err := helpers.GetUserFromMention(args[2])
                            if err != nil || targetUser.ID == "" {
                                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
                                return
                            }

                            channel, err := helpers.GetChannel(msg.ChannelID)
                            helpers.Relax(err)

                            badgeToAllow := m.GetBadge(args[3], args[4], channel.GuildID)
                            if badgeToAllow.ID == "" {
                                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.badge-error-not-found"))
                                helpers.Relax(err)
                                return
                            }

                            if badgeToAllow.GuildID == "global" && !helpers.IsBotAdmin(msg.Author.ID) {
                                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.edit-badge-error-not-allowed"))
                                helpers.Relax(err)
                                return
                            }

                            isAlreadyAllowed := false
                            for _, userAllowedID := range badgeToAllow.AllowedUserIDs {
                                if userAllowedID == targetUser.ID {
                                    isAlreadyAllowed = true
                                }
                            }

                            if isAlreadyAllowed == false {
                                badgeToAllow.AllowedUserIDs = append(badgeToAllow.AllowedUserIDs, targetUser.ID)
                                m.UpdateBadge(badgeToAllow)

                                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.levels.allow-badge-success-allowed",
                                    targetUser.Username, badgeToAllow.Name, badgeToAllow.Category))
                                helpers.Relax(err)
                                return
                            } else {
                                allowedUserIDsWithout := make([]string, 0)
                                for _, userAllowedID := range badgeToAllow.AllowedUserIDs {
                                    if userAllowedID != targetUser.ID {
                                        allowedUserIDsWithout = append(allowedUserIDsWithout, userAllowedID)
                                    }
                                }
                                badgeToAllow.AllowedUserIDs = allowedUserIDsWithout
                                m.UpdateBadge(badgeToAllow)

                                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.levels.allow-badge-success-not-allowed",
                                    targetUser.Username, badgeToAllow.Name, badgeToAllow.Category))
                                helpers.Relax(err)
                                return
                            }
                        })
                        return
                    case "deny": // [p]profile badge deny <user id/mention> <category name> <badge name>
                        helpers.RequireMod(msg, func() {
                            session.ChannelTyping(msg.ChannelID)
                            if len(args) < 5 {
                                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                                helpers.Relax(err)
                                return
                            }

                            targetUser, err := helpers.GetUserFromMention(args[2])
                            if err != nil || targetUser.ID == "" {
                                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
                                return
                            }

                            channel, err := helpers.GetChannel(msg.ChannelID)
                            helpers.Relax(err)

                            badgeToDeny := m.GetBadge(args[3], args[4], channel.GuildID)
                            if badgeToDeny.ID == "" {
                                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.badge-error-not-found"))
                                helpers.Relax(err)
                                return
                            }

                            if badgeToDeny.GuildID == "global" && !helpers.IsBotAdmin(msg.Author.ID) {
                                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.edit-badge-error-not-allowed"))
                                helpers.Relax(err)
                                return
                            }

                            isAlreadyDenied := false
                            for _, userDeniedID := range badgeToDeny.DeniedUserIDs {
                                if userDeniedID == targetUser.ID {
                                    isAlreadyDenied = true
                                }
                            }

                            if isAlreadyDenied == false {
                                badgeToDeny.DeniedUserIDs = append(badgeToDeny.DeniedUserIDs, targetUser.ID)
                                m.UpdateBadge(badgeToDeny)

                                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.levels.deny-badge-success-denied",
                                    targetUser.Username, badgeToDeny.Name, badgeToDeny.Category))
                                helpers.Relax(err)
                                return
                            } else {
                                deniedUserIDsWithout := make([]string, 0)
                                for _, userDeniedID := range badgeToDeny.DeniedUserIDs {
                                    if userDeniedID != targetUser.ID {
                                        deniedUserIDsWithout = append(deniedUserIDsWithout, userDeniedID)
                                    }
                                }
                                badgeToDeny.DeniedUserIDs = deniedUserIDsWithout
                                m.UpdateBadge(badgeToDeny)

                                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.levels.deny-badge-success-not-denied",
                                    targetUser.Username, badgeToDeny.Name, badgeToDeny.Category))
                                helpers.Relax(err)
                                return
                            }
                        })
                        return
                    case "move": // [p]profile badge move <category name> <badge name> <#>
                        session.ChannelTyping(msg.ChannelID)
                        if len(args) < 5 {
                            _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                            helpers.Relax(err)
                            return
                        }
                        categoryName := args[2]
                        badgeName := args[3]
                        newSpot, err := strconv.Atoi(args[4])
                        if err != nil {
                            _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                            helpers.Relax(err)
                            return
                        }

                        userData := m.GetUserUserdata(msg.Author)

                        idToMove := ""
                        for _, badgeID := range userData.ActiveBadgeIDs {
                            badge := m.GetBadgeByID(badgeID)
                            if badge.Category == categoryName && badge.Name == badgeName {
                                idToMove = badge.ID
                            }
                        }

                        if idToMove == "" {
                            _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.badge-error-not-found"))
                            helpers.Relax(err)
                            return
                        }

                        newBadgeList := make([]string, 0)
                        badgeAdded := false
                        for _, badgeID := range userData.ActiveBadgeIDs {
                            if len(newBadgeList)+1 == newSpot {
                                newBadgeList = append(newBadgeList, idToMove)
                                badgeAdded = true
                            }
                            if badgeID != idToMove {
                                newBadgeList = append(newBadgeList, badgeID)
                            }
                        }
                        if badgeAdded == false && len(newBadgeList) < BadgeLimt {
                            newBadgeList = append(newBadgeList, idToMove)
                        }
                        userData.ActiveBadgeIDs = newBadgeList
                        m.setUserUserdata(userData)

                        _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.move-badge-success"))
                        helpers.Relax(err)

                        return
                    }
                }
                session.ChannelTyping(msg.ChannelID)

                availableBadges := m.GetBadgesAvailable(msg.Author)

                if len(availableBadges) <= 0 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.badge-error-none"))
                    helpers.Relax(err)
                    return
                }

                userData := m.GetUserUserdata(msg.Author)
                newActiveBadgeIDs := make([]string, 0)
                for _, activeBadgeID := range userData.ActiveBadgeIDs {
                    for _, availableBadge := range availableBadges {
                        if availableBadge.ID == activeBadgeID {
                            newActiveBadgeIDs = append(newActiveBadgeIDs, activeBadgeID)
                        }
                    }
                }
                userData.ActiveBadgeIDs = newActiveBadgeIDs

                shownBadges := make([]DB_Badge, 0)
                for _, badge := range availableBadges {
                    if badge.GuildID == "global" || badge.GuildID == channel.GuildID {
                        shownBadges = append(shownBadges, badge)
                    }
                }

                inCategory := ""
                stoppedLoop := false
                lastBotMessageID := make([]string, 0)
                closeHandler := session.AddHandler(func(session *discordgo.Session, loopMessage *discordgo.MessageCreate) {
                    if stoppedLoop == false && loopMessage.Author.ID == msg.Author.ID && loopMessage.ChannelID == msg.ChannelID {
                        cleanedContent := strings.Replace(loopMessage.Content, "_profile ", "", 1)
                        cleanedContent = strings.Replace(cleanedContent, "category ", "", 1)
                        cleanedContent = strings.Replace(cleanedContent, "badge ", "", 1)
                        loopArgs := strings.Fields(strings.ToLower(cleanedContent))
                    BadgePickLoop:
                        for {
                            if len(loopArgs) > 0 {
                                switch loopArgs[0] {
                                case "stop", "exit":
                                    m.setUserUserdata(userData)
                                    m.DeleteMessages(msg.ChannelID, lastBotMessageID)
                                    _, err := session.ChannelMessageSend(msg.ChannelID,
                                        fmt.Sprintf("**@%s** I saved your badges. Check out your new shiny profile with `_profile` :sparkles: \n", msg.Author.Username))
                                    helpers.Relax(err)
                                    stoppedLoop = true
                                    newActiveBadgePickerUserIDs := make(map[string]string, 0)
                                    for activeBadgePickerUserID, activeBadgePickerChannelID := range activeBadgePickerUserIDs {
                                        if activeBadgePickerUserID != msg.Author.ID {
                                            newActiveBadgePickerUserIDs[activeBadgePickerUserID] = activeBadgePickerChannelID
                                        }
                                    }
                                    activeBadgePickerUserIDs = newActiveBadgePickerUserIDs
                                    return
                                case "reset":
                                    userData.ActiveBadgeIDs = make([]string, 0)
                                    loopArgs = []string{"categories"}
                                    continue BadgePickLoop
                                    return
                                case "categories":
                                    inCategory = ""
                                    m.DeleteMessages(msg.ChannelID, lastBotMessageID)
                                    lastBotMessageID = m.BadgePickerPrintCategories(msg.Author, msg.ChannelID, shownBadges, userData.ActiveBadgeIDs, availableBadges)
                                    return
                                default:
                                    if inCategory == "" {
                                        for _, badge := range shownBadges {
                                            if badge.Category == strings.ToLower(loopArgs[0]) {
                                                m.DeleteMessages(msg.ChannelID, lastBotMessageID)
                                                inCategory = strings.ToLower(loopArgs[0])
                                                lastBotMessageID = m.BadgePickerPrintBadges(msg.Author, msg.ChannelID, shownBadges, userData.ActiveBadgeIDs, inCategory, availableBadges)
                                                return
                                            }
                                        }
                                        m.DeleteMessages(msg.ChannelID, lastBotMessageID)
                                        message, err := session.ChannelMessageSend(msg.ChannelID,
                                            fmt.Sprintf("**@%s** I wasn't able to find a category with that name.\n%s", msg.Author.Username, m.BadgePickerHelpText()))
                                        helpers.Relax(err)
                                        lastBotMessageID = []string{message.ID}
                                        return
                                    } else {
                                        for _, badge := range shownBadges {
                                            if badge.Category == inCategory && badge.Name == strings.ToLower(loopArgs[0]) {
                                                for _, activeBadgeID := range userData.ActiveBadgeIDs {
                                                    if activeBadgeID == badge.ID {
                                                        newActiveBadges := make([]string, 0)
                                                        for _, newActiveBadgeID := range userData.ActiveBadgeIDs {
                                                            if newActiveBadgeID != badge.ID {
                                                                newActiveBadges = append(newActiveBadges, newActiveBadgeID)
                                                            }
                                                        }
                                                        loopArgs = []string{"categories"}
                                                        userData.ActiveBadgeIDs = newActiveBadges
                                                        continue BadgePickLoop
                                                    }
                                                }
                                                if len(userData.ActiveBadgeIDs) >= BadgeLimt {
                                                    m.DeleteMessages(msg.ChannelID, lastBotMessageID)
                                                    message, err := session.ChannelMessageSend(msg.ChannelID,
                                                        fmt.Sprintf("**@%s** You are already got enough emotes.\n%s", msg.Author.Username, m.BadgePickerHelpText()))
                                                    helpers.Relax(err)
                                                    lastBotMessageID = []string{message.ID}
                                                    return
                                                }

                                                loopArgs = []string{"categories"}
                                                userData.ActiveBadgeIDs = append(userData.ActiveBadgeIDs, badge.ID)
                                                if len(userData.ActiveBadgeIDs) >= BadgeLimt {
                                                    m.setUserUserdata(userData)
                                                    m.DeleteMessages(msg.ChannelID, lastBotMessageID)
                                                    _, err := session.ChannelMessageSend(msg.ChannelID,
                                                        fmt.Sprintf("**@%s** I saved your emotes. Check out your new shiny profile with `_profile` :sparkles: \n",
                                                            msg.Author.Username))
                                                    helpers.Relax(err)
                                                    stoppedLoop = true
                                                    newActiveBadgePickerUserIDs := make(map[string]string, 0)
                                                    for activeBadgePickerUserID, activeBadgePickerChannelID := range activeBadgePickerUserIDs {
                                                        if activeBadgePickerUserID != msg.Author.ID {
                                                            newActiveBadgePickerUserIDs[activeBadgePickerUserID] = activeBadgePickerChannelID
                                                        }
                                                    }
                                                    activeBadgePickerUserIDs = newActiveBadgePickerUserIDs
                                                    return
                                                }
                                                continue BadgePickLoop
                                            }
                                        }
                                        m.DeleteMessages(msg.ChannelID, lastBotMessageID)
                                        message, err := session.ChannelMessageSend(msg.ChannelID,
                                            fmt.Sprintf("**@%s** I wasn't able to find a badge with that name.\n%s", msg.Author.Username, m.BadgePickerHelpText()))
                                        helpers.Relax(err)
                                        lastBotMessageID = []string{message.ID}
                                        return
                                    }
                                }
                            }
                            return
                        }
                    }
                })
                activeBadgePickerUserIDs[msg.Author.ID] = msg.ChannelID
                lastBotMessageID = m.BadgePickerPrintCategories(msg.Author, msg.ChannelID, shownBadges, userData.ActiveBadgeIDs, availableBadges)
                time.Sleep(5 * time.Minute)
                closeHandler()
                if stoppedLoop == false {
                    m.setUserUserdata(userData)
                    newActiveBadgePickerUserIDs := make(map[string]string, 0)
                    for activeBadgePickerUserID, activeBadgePickerChannelID := range activeBadgePickerUserIDs {
                        if activeBadgePickerUserID != msg.Author.ID {
                            newActiveBadgePickerUserIDs[activeBadgePickerUserID] = activeBadgePickerChannelID
                        }
                    }
                    activeBadgePickerUserIDs = newActiveBadgePickerUserIDs

                    m.DeleteMessages(msg.ChannelID, lastBotMessageID)
                    _, err := session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("**@%s** I stopped the badge picking and saved your badges because of the time limit.\nUse `_profile badge` if you want to pick more badges.",
                        msg.Author.Username))
                    helpers.Relax(err)
                }
                return
            case "color", "colourb":
                session.ChannelTyping(msg.ChannelID)
                if len(args) < 2 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                    helpers.Relax(err)
                    return
                }

                userUserdata := m.GetUserUserdata(msg.Author)

                switch args[1] {
                case "background", "box":
                    if len(args) >= 3 {
                        userUserdata.BackgroundColor = strings.Replace(args[2], "#", "", -1)
                    } else {
                        userUserdata.BackgroundColor = ""
                    }
                case "accent":
                    if len(args) >= 3 {
                        userUserdata.AccentColor = strings.Replace(args[2], "#", "", -1)
                    } else {
                        userUserdata.AccentColor = ""
                    }
                case "text":
                    if len(args) >= 3 {
                        userUserdata.TextColor = strings.Replace(args[2], "#", "", -1)
                    } else {
                        userUserdata.TextColor = ""
                    }
                default:
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                    helpers.Relax(err)
                    return
                }
                m.setUserUserdata(userUserdata)

                _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.profile-color-set-success"))
                helpers.Relax(err)
                return
            case "opacity":
                session.ChannelTyping(msg.ChannelID)
                if len(args) < 2 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                    helpers.Relax(err)
                    return
                }

                userUserdata := m.GetUserUserdata(msg.Author)

                opacityText := "0.5"
                if len(args) >= 3 {
                    opacity, err := strconv.ParseFloat(args[2], 64)
                    if err != nil {
                        _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                        helpers.Relax(err)
                        return
                    }
                    opacityText = fmt.Sprintf("%.1f", opacity)
                }

                switch args[1] {
                case "background", "box":
                    userUserdata.BackgroundOpacity = opacityText
                case "details", "detail":
                    userUserdata.DetailOpacity = opacityText
                default:
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                    helpers.Relax(err)
                    return
                }

                m.setUserUserdata(userUserdata)

                _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.profile-opacity-set-success"))
                helpers.Relax(err)
                return
            case "timezone":
                if len(args) < 2 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.profile-timezone-list"))
                    helpers.Relax(err)
                    return
                }

                loc, err := time.LoadLocation(args[1])
                if err != nil {
                    _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.profile-timezone-set-error")+"\n"+helpers.GetText("plugins.levels.profile-timezone-list"))
                    helpers.Relax(err)
                    return
                }

                userUserdata := m.GetUserUserdata(msg.Author)
                userUserdata.Timezone = loc.String()
                m.setUserUserdata(userUserdata)

                _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.levels.profile-timezone-set-success",
                loc.String(), time.Now().In(loc).Format(TimeAtUserFormat)))
                helpers.Relax(err)
                return
            case "birthday":
                var err error

                newBirthday := ""
                if len(args) >= 2 {
                    _, err = time.Parse(TimeBirthdayFormat, args[1])
                    if err != nil {
                        _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.profile-birthday-set-error-format"))
                        helpers.Relax(err)
                        return
                    }
                    newBirthday = args[1]
                }

                userUserdata := m.GetUserUserdata(msg.Author)
                userUserdata.Birthday = newBirthday
                m.setUserUserdata(userUserdata)

                _, err = session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.profile-birthday-set-success"))
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

        targetMember, err := helpers.GetGuildMember(channel.GuildID, targetUser.ID)
        helpers.Relax(err)

        gifP := false
        if command == "gif-profile" {
            gifP = true
        }

        jpgBytes, ext, err := m.GetProfile(targetMember, guild, gifP)
        helpers.Relax(err)

        _, err = session.ChannelFileSendWithMessage(
            msg.ChannelID,
            fmt.Sprintf("<@%s> Profile for %s", msg.Author.ID, targetUser.Username),
            fmt.Sprintf("%s-Robyul.%s", targetUser.ID, ext), bytes.NewReader(jpgBytes))
        helpers.Relax(err)

        return
    case "level", "levels": // [p]level <user> or [p]level top
        session.ChannelTyping(msg.ChannelID)
        targetUser, err := helpers.GetUser(msg.Author.ID)
        helpers.Relax(err)
        args := strings.Fields(content)

        channel, err := helpers.GetChannel(msg.ChannelID)
        helpers.Relax(err)

        if len(args) >= 1 && args[0] != "" {
            switch args[0] {
            case "leaderboard", "top": // [p]level top
                // TODO: use cached top list
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

                    currentMember, err := helpers.GetGuildMember(channel.GuildID, levelsServersUsers[i-offset].UserID)
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
                var rankedTotalExpMap PairList
                for _, serverCache := range topCache {
                    if serverCache.GuildID == "global" {
                        rankedTotalExpMap = serverCache.Levels
                    }
                }

                if len(rankedTotalExpMap) <= 0 {
                    _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.levels.no-stats-available-yet"))
                    helpers.Relax(err)
                    return
                }

                globalTopLevelEmbed := &discordgo.MessageEmbed{
                    Color: 0x0FADED,
                    Title: helpers.GetText("plugins.levels.global-top-server-embed-title"),
                    //Description: "",
                    Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetTextF("plugins.levels.embed-footer", len(session.State.Guilds))},
                    Fields: []*discordgo.MessageEmbedField{},
                }

                i := 0
                for _, userRanked := range rankedTotalExpMap {
                    currentUser, err := helpers.GetUser(userRanked.Key)
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
                                    ignoredUser, err := helpers.GetGuildMember(channel.GuildID, ignoredUserID)
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
                    channel, err := helpers.GetChannel(msg.ChannelID)
                    helpers.Relax(err)
                    guild, err := helpers.GetGuild(channel.GuildID)
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
                        if guildChannelCurrent.Type == discordgo.ChannelTypeGuildVoice {
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

        currentMember, err := helpers.GetGuildMember(channel.GuildID, levelThisServerUser.UserID)
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

func (l *Levels) DeleteMessages(channelID string, messages []string) {
    for _, message := range messages {
        cache.GetSession().ChannelMessageDelete(channelID, message)
    }
}

func (l *Levels) BadgePickerPrintCategories(user *discordgo.User, channeID string, availableBadges []DB_Badge, activeBadgeIDs []string, allBadges []DB_Badge) []string {
    categoriesCount := make(map[string]int, 0)
    for _, badge := range availableBadges {
        if _, ok := categoriesCount[badge.Category]; ok {
            categoriesCount[badge.Category] += 1
        } else {
            categoriesCount[badge.Category] = 1
        }
    }

    sortedKeys := make([]string, len(categoriesCount))
    i := 0
    for k := range categoriesCount {
        sortedKeys[i] = k
        i++
    }
    sort.Strings(sortedKeys)

    resultText := l.BadgePickerActiveText(user.Username, activeBadgeIDs, allBadges)
    resultText += "Choose a category name:\n"
    for _, key := range sortedKeys {
        resultText += fmt.Sprintf("__%s__ (%d badges)\n", key, categoriesCount[key])
    }
    resultText += l.BadgePickerHelpText()

    session := cache.GetSession()
    messageIDs := make([]string, 0)
    for _, page := range helpers.Pagify(resultText, "\n") {
        message, err := session.ChannelMessageSend(channeID, page)
        helpers.Relax(err)
        messageIDs = append(messageIDs, message.ID)
    }
    return messageIDs
}

func (l *Levels) BadgePickerPrintBadges(user *discordgo.User, channeID string, availableBadges []DB_Badge, activeBadgeIDs []string, categoryName string, allBadges []DB_Badge) []string {
    categoryName = strings.ToLower(categoryName)

    resultText := l.BadgePickerActiveText(user.Username, activeBadgeIDs, allBadges)
    resultText += "Choose a badge name:\n"
    for _, badge := range availableBadges {
        if badge.Category == categoryName {
            isActive := false
            for _, activeBadgeID := range activeBadgeIDs {
                if activeBadgeID == badge.ID {
                    isActive = true
                }
            }
            activeText := ""
            if isActive {
                activeText = " _**in use**_"
            }
            resultText += fmt.Sprintf("__%s__%s\n", badge.Name, activeText)
        }
    }
    resultText += l.BadgePickerHelpText()

    session := cache.GetSession()
    messageIDs := make([]string, 0)
    for _, page := range helpers.Pagify(resultText, "\n") {
        message, err := session.ChannelMessageSend(channeID, page)
        helpers.Relax(err)
        messageIDs = append(messageIDs, message.ID)
    }
    return messageIDs
}

func (l *Levels) BadgePickerActiveText(username string, activeBadgeIDs []string, availableBadges []DB_Badge) string {
    spaceLeft := BadgeLimt - len(activeBadgeIDs)
    text := fmt.Sprintf("**@%s** You can pick %d more badge(s) to display on your profile.\nYou are currently displaying:", username, spaceLeft)
    if len(activeBadgeIDs) > 0 {
        for _, badgeID := range activeBadgeIDs {
            for _, badge := range availableBadges {
                if badge.ID == badgeID {
                    text += fmt.Sprintf(" `%s (%s)`", badge.Name, badge.Category)
                }
            }
        }
    } else {
        text += " No badges"
    }
    text += ".\n\n"
    return text
}

func (l *Levels) BadgePickerHelpText() string {
    return "\nSay `categories` to display all categories, `category name` to choose a category, `badge name` to choose a badge, `reset` to remove all badges displayed on your profile, `exit` to exit and save. To remove a badge from your Profile pick the badge again.\n"
}

func (l *Levels) InsertNewProfileBackground(backgroundName string, backgroundUrl string) error {
    newEntry := new(DB_Profile_Background)
    newEntry.Name = strings.ToLower(backgroundName)
    newEntry.URL = backgroundUrl
    newEntry.CreatedAt = time.Now()

    insert := rethink.Table("profile_backgrounds").Insert(newEntry)
    _, err := insert.RunWrite(helpers.GetDB())
    return err
}

func (l *Levels) DeleteProfileBackground(backgroundName string) error {
    if backgroundName != "" {
        _, err := rethink.Table("profile_backgrounds").Filter(
            rethink.Row.Field("id").Eq(backgroundName),
        ).Delete().RunWrite(helpers.GetDB())
        return err
    }
    return nil
}

func (l *Levels) ProfileBackgroundSearch(searchText string) []DB_Profile_Background {
    var entryBucket []DB_Profile_Background
    listCursor, err := rethink.Table("profile_backgrounds").Filter(func(profile rethink.Term) rethink.Term {
        return profile.Field("id").Match(fmt.Sprintf("(?i)%s", searchText))
    }).Limit(5).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.All(&entryBucket)

    if err == rethink.ErrEmptyResult {
        return entryBucket
    } else if err != nil {
        helpers.Relax(err)
    }

    return entryBucket
}

func (l *Levels) ProfileBackgroundNameExists(backgroundName string) bool {
    var entryBucket DB_Profile_Background
    listCursor, err := rethink.Table("profile_backgrounds").Filter(
        rethink.Row.Field("id").Eq(strings.ToLower(backgroundName)),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.One(&entryBucket)

    if err == rethink.ErrEmptyResult {
        var entryBucket DB_Profile_Background
        listCursor, err := rethink.Table("profile_backgrounds").Filter(func(profile rethink.Term) rethink.Term {
            return profile.Field("id").Match(fmt.Sprintf("(?i)^%s$", backgroundName))
        }).Run(helpers.GetDB())
        defer listCursor.Close()
        err = listCursor.One(&entryBucket)

        if err == rethink.ErrEmptyResult {
            return false
        } else if err != nil {
            helpers.Relax(err)
        }

        return true
    } else if err != nil {
        helpers.Relax(err)
    }

    return true
}

func (l *Levels) GetProfileBackgroundUrl(backgroundName string) string {
    if backgroundName == "" {
        return "http://i.imgur.com/I9b74U9.jpg"
    }

    var entryBucket DB_Profile_Background
    listCursor, err := rethink.Table("profile_backgrounds").Filter(
        rethink.Row.Field("id").Eq(strings.ToLower(backgroundName)),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.One(&entryBucket)

    if err == rethink.ErrEmptyResult {
        var entryBucket DB_Profile_Background
        listCursor, err := rethink.Table("profile_backgrounds").Filter(func(profile rethink.Term) rethink.Term {
            return profile.Field("id").Match(fmt.Sprintf("(?i)%s", backgroundName))
        }).Run(helpers.GetDB())
        defer listCursor.Close()
        err = listCursor.One(&entryBucket)

        if err == rethink.ErrEmptyResult {
            return "http://i.imgur.com/I9b74U9.jpg" // Default Robyul Background
        } else if err != nil {
            helpers.Relax(err)
        }

        return entryBucket.URL
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

func (l *Levels) GetServerOnlyBadges(guildID string) []DB_Badge {
    var entryBucket []DB_Badge
    listCursor, err := rethink.Table("profile_badge").Filter(
        rethink.Row.Field("guildid").Eq(guildID),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.All(&entryBucket)

    if err != nil {
        if err != rethink.ErrEmptyResult {
            helpers.Relax(err)
        }
    }

    return entryBucket
}

func (l *Levels) GetServerBadges(guildID string) []DB_Badge {
    var entryBucket []DB_Badge
    var globalEntryBucket []DB_Badge
    listCursor, err := rethink.Table("profile_badge").Filter(
        rethink.Row.Field("guildid").Eq(guildID),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.All(&entryBucket)

    listCursor, err = rethink.Table("profile_badge").Filter(
        rethink.Row.Field("guildid").Eq("global"),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.All(&globalEntryBucket)

    for _, globalEntry := range globalEntryBucket {
        entryBucket = append(entryBucket, globalEntry)
    }

    if err != nil {
        if err != rethink.ErrEmptyResult {
            helpers.Relax(err)
        }
    }

    return entryBucket
}

func (l *Levels) GetCategoryBadges(category string, guildID string) []DB_Badge {
    var entryBucket []DB_Badge
    result := make([]DB_Badge, 0)
    listCursor, err := rethink.Table("profile_badge").Filter(
        rethink.Row.Field("category").Eq(category),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.All(&entryBucket)

    if err == rethink.ErrEmptyResult {
        return result
    } else if err != nil {
        helpers.Relax(err)
    }

    for _, badge := range entryBucket {
        if badge.GuildID == guildID || badge.GuildID == "global" {
            result = append(result, badge)
        }
    }

    return result
}

func (l *Levels) GetBadge(category string, name string, guildID string) DB_Badge {
    var entryBucket []DB_Badge
    var emptyBadge DB_Badge
    listCursor, err := rethink.Table("profile_badge").Filter(
        rethink.Row.Field("category").Eq(strings.ToLower(category)),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.All(&entryBucket)

    if err == rethink.ErrEmptyResult {
        return emptyBadge
    } else if err != nil {
        helpers.Relax(err)
    }

    for _, badge := range entryBucket {
        if strings.ToLower(badge.Name) == strings.ToLower(name) && (badge.GuildID == guildID || badge.GuildID == "global") {
            return badge
        }
    }

    return emptyBadge
}

func (l *Levels) GetBadgeByID(badgeID string) DB_Badge {
    var badgeBucket DB_Badge
    listCursor, err := rethink.Table("profile_badge").Filter(
        rethink.Row.Field("id").Eq(badgeID),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.One(&badgeBucket)

    if err == rethink.ErrEmptyResult {
        return badgeBucket
    } else if err != nil {
        helpers.Relax(err)
    }

    return badgeBucket
}

func (l *Levels) GetBadgesAvailable(user *discordgo.User) []DB_Badge {
    guildsToCheck := make([]string, 0)
    guildsToCheck = append(guildsToCheck, "global")

    session := cache.GetSession()

    for _, guild := range session.State.Guilds {
        if _, err := helpers.GetGuildMember(guild.ID, user.ID); err == nil {
            guildsToCheck = append(guildsToCheck, guild.ID)
        }
    }

    var allBadges []DB_Badge
    for _, guildToCheck := range guildsToCheck {
        var entryBucket []DB_Badge
        listCursor, err := rethink.Table("profile_badge").Filter(
            rethink.Row.Field("guildid").Eq(guildToCheck),
        ).Run(helpers.GetDB())
        defer listCursor.Close()
        if err != nil {
            continue
        }
        err = listCursor.All(&entryBucket)
        if err != nil {
            continue
        }
        for _, entryBadge := range entryBucket {
            allBadges = append(allBadges, entryBadge)
        }
    }

    levelCache := make(map[string]int, 0)

    var availableBadges []DB_Badge
    for _, foundBadge := range allBadges {
        isAllowed := false

        // Level Check
        if foundBadge.LevelRequirement < 0 { // Available for no one?
            isAllowed = false
        } else if foundBadge.LevelRequirement == 0 { // Available for everyone?
            isAllowed = true
        } else if foundBadge.LevelRequirement > 0 { // Meets min level?
            if _, ok := levelCache[foundBadge.GuildID]; ok {
                if foundBadge.LevelRequirement <= levelCache[foundBadge.GuildID] {
                    isAllowed = true
                } else {
                    isAllowed = false
                }
            } else {
                levelCache[foundBadge.GuildID] = l.GetLevelForUser(user.ID, foundBadge.GuildID)
                if foundBadge.LevelRequirement <= levelCache[foundBadge.GuildID] {
                    isAllowed = true
                } else {
                    isAllowed = false
                }
            }
        }

        // User is in allowed user list?
        for _, allowedUserID := range foundBadge.AllowedUserIDs {
            if allowedUserID == user.ID {
                isAllowed = true
            }
        }

        // User is in denied user list?
        for _, deniedUserID := range foundBadge.DeniedUserIDs {
            if deniedUserID == user.ID {
                isAllowed = false
            }
        }

        if isAllowed == true {
            availableBadges = append(availableBadges, foundBadge)
        }
    }

    return availableBadges
}

func (l *Levels) GetBadgesAvailableServer(user *discordgo.User, serverID string) []DB_Badge {
    guildsToCheck := make([]string, 0)
    guildsToCheck = append(guildsToCheck, "global")

    if _, err := helpers.GetGuildMember(serverID, user.ID); err == nil {
        guildsToCheck = append(guildsToCheck, serverID)
    }

    var allBadges []DB_Badge
    for _, guildToCheck := range guildsToCheck {
        var entryBucket []DB_Badge
        listCursor, err := rethink.Table("profile_badge").Filter(
            rethink.Row.Field("guildid").Eq(guildToCheck),
        ).Run(helpers.GetDB())
        defer listCursor.Close()
        if err != nil {
            continue
        }
        err = listCursor.All(&entryBucket)
        if err != nil {
            continue
        }
        for _, entryBadge := range entryBucket {
            allBadges = append(allBadges, entryBadge)
        }
    }

    var availableBadges []DB_Badge
    for _, foundBadge := range allBadges {
        isAllowed := false

        // Level Check
        if foundBadge.LevelRequirement < 0 { // Available for no one?
            isAllowed = false
        } else if foundBadge.LevelRequirement == 0 { // Available for everyone?
            isAllowed = true
        } else if foundBadge.LevelRequirement > 0 { // Meets min level=
            if foundBadge.LevelRequirement <= l.GetLevelForUser(user.ID, foundBadge.GuildID) {
                isAllowed = true
            } else {
                isAllowed = false
            }
        }

        // User is in allowed user list?
        for _, allowedUserID := range foundBadge.AllowedUserIDs {
            if allowedUserID == user.ID {
                isAllowed = true
            }
        }

        // User is in denied user list?
        for _, deniedUserID := range foundBadge.DeniedUserIDs {
            if deniedUserID == user.ID {
                isAllowed = false
            }
        }

        if isAllowed == true {
            availableBadges = append(availableBadges, foundBadge)
        }
    }

    return availableBadges
}

func (l *Levels) GetBadgesAvailableQuick(user *discordgo.User) []DB_Badge {
    guildsToCheck := make([]string, 0)
    guildsToCheck = append(guildsToCheck, "global")

    session := cache.GetSession()

    for _, guild := range session.State.Guilds {
        if _, err := helpers.GetGuildMember(guild.ID, user.ID); err == nil {
            guildsToCheck = append(guildsToCheck, guild.ID)
        }
    }

    var allBadges []DB_Badge
    for _, guildToCheck := range guildsToCheck {
        var entryBucket []DB_Badge
        listCursor, err := rethink.Table("profile_badge").Filter(
            rethink.Row.Field("guildid").Eq(guildToCheck),
        ).Run(helpers.GetDB())
        defer listCursor.Close()
        if err != nil {
            continue
        }
        err = listCursor.All(&entryBucket)
        if err != nil {
            continue
        }
        for _, entryBadge := range entryBucket {
            allBadges = append(allBadges, entryBadge)
        }
    }

    var availableBadges []DB_Badge
    for _, foundBadge := range allBadges {
        isAllowed := true

        // User is in allowed user list?
        for _, allowedUserID := range foundBadge.AllowedUserIDs {
            if allowedUserID == user.ID {
                isAllowed = true
            }
        }

        // User is in denied user list?
        for _, deniedUserID := range foundBadge.DeniedUserIDs {
            if deniedUserID == user.ID {
                isAllowed = false
            }
        }

        if isAllowed == true {
            availableBadges = append(availableBadges, foundBadge)
        }
    }

    return availableBadges
}

func (l *Levels) GetLevelForUser(userID string, guildID string) int {
    var levelsServersUser []DB_Levels_ServerUser
    listCursor, err := rethink.Table("levels_serverusers").Filter(
        rethink.Row.Field("userid").Eq(userID),
    ).Run(helpers.GetDB())
    helpers.Relax(err)
    defer listCursor.Close()
    err = listCursor.All(&levelsServersUser)

    if err == rethink.ErrEmptyResult {
        return 0
    } else if err != nil {
        helpers.Relax(err)
    }

    if guildID == "global" {
        totalExp := int64(0)
        for _, levelsServerUser := range levelsServersUser {
            totalExp += levelsServerUser.Exp
        }
        return l.getLevelFromExp(totalExp)
    } else {
        for _, levelsServerUser := range levelsServersUser {
            if levelsServerUser.GuildID == guildID {
                return l.getLevelFromExp(levelsServerUser.Exp)
            }
        }
    }

    return 0
}

func (l *Levels) InsertBadge(entry DB_Badge) {
    insert := rethink.Table("profile_badge").Insert(entry)
    _, err := insert.RunWrite(helpers.GetDB())
    if err != nil {
        helpers.Relax(err)
    }
    return
}

func (l *Levels) UpdateBadge(entry DB_Badge) {
    if entry.ID != "" {
        _, err := rethink.Table("profile_badge").Update(entry).Run(helpers.GetDB())
        helpers.Relax(err)
    }
}

func (l *Levels) DeleteBadge(badgeID string) {
    _, err := rethink.Table("profile_badge").Filter(
        rethink.Row.Field("id").Eq(badgeID),
    ).Delete().RunWrite(helpers.GetDB())
    helpers.Relax(err)
    return
}

func (l *Levels) setUserUserdata(entry DB_Profile_Userdata) {
    _, err := rethink.Table("profile_userdata").Update(entry).Run(helpers.GetDB())
    helpers.Relax(err)
}

func (m *Levels) GetProfile(member *discordgo.Member, guild *discordgo.Guild, gifP bool) ([]byte, string, error) {
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

    serverRank := "N/A"
    globalRank := "N/A"
    for _, serverCache := range topCache {
        if serverCache.GuildID == "global" {
            for i, pair := range serverCache.Levels {
                if pair.Key == member.User.ID {
                    globalRank = strconv.Itoa(i + 1)
                }
            }
        } else if serverCache.GuildID == guild.ID {
            for i, pair := range serverCache.Levels {
                if pair.Key == member.User.ID {
                    serverRank = strconv.Itoa(i + 1)
                }
            }
        }
    }

    userData := m.GetUserUserdata(member.User)

    avatarUrl := helpers.GetAvatarUrl(member.User)
    avatarUrlGif := ""
    if avatarUrl != "" {
        avatarUrl = strings.Replace(avatarUrl, "size=1024", "size=128", -1)
        if strings.Contains(avatarUrl, "gif") {
            avatarUrlGif = avatarUrl
        }
        avatarUrl = strings.Replace(avatarUrl, "gif", "png", -1)
        avatarUrl = strings.Replace(avatarUrl, "jpg", "png", -1)
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

    badgesToDisplay := make([]DB_Badge, 0)
    availableBadges := m.GetBadgesAvailableQuick(member.User)
    for _, activeBadgeID := range userData.ActiveBadgeIDs {
        for _, availableBadge := range availableBadges {
            if activeBadgeID == availableBadge.ID {
                badgesToDisplay = append(badgesToDisplay, availableBadge)
            }
        }
    }
    badgesHTML := ""
    for _, badge := range badgesToDisplay {
        badgesHTML += fmt.Sprintf("<img src=\"%s\" style=\"border: 2px solid #%s;\">", badge.URL, badge.BorderColor)
    }

    backgroundColor, err := colorful.Hex("#" + m.GetBackgroundColor(userData))
    if err != nil {
        backgroundColor, err = colorful.Hex("#000000")
        if err != nil {
            helpers.Relax(err)
        }
    }
    backgroundColorString := fmt.Sprintf("rgba(%d, %d, %d, %s)",
        int(backgroundColor.R*255), int(backgroundColor.G*255), int(backgroundColor.B*255),
        m.GetBackgroundOpacity(userData))
    detailColorString := fmt.Sprintf("rgba(0, 0, 0, %s)",
        m.GetDetailOpacity(userData))

    userTimeText := ""
    if userData.Timezone != "" {
        userLocation, err := time.LoadLocation(userData.Timezone)
        if err == nil {
            userTimeText = "<i class=\"fa fa-clock-o\" aria-hidden=\"true\"></i> " + time.Now().In(userLocation).Format(TimeAtUserFormat)
        }
    }

    userBirthdayText := ""
    isBirthday := false
    if userData.Birthday != "" {
        userLocation, err := time.LoadLocation("Etc/UTC")
        if err == nil {
            if userData.Timezone != "" {
                userLocationUser, err := time.LoadLocation(userData.Timezone)
                if err == nil {
                    userLocation = userLocationUser
                }
            }
            birthdayTime, err := time.ParseInLocation(TimeBirthdayFormat, userData.Birthday, userLocation)
            birthdayTime = birthdayTime.AddDate(time.Now().Year(), 0, 0)
            if err == nil {
                if time.Now().In(userLocation).Sub(birthdayTime).Hours() <= 23 && time.Now().In(userLocation).Sub(birthdayTime).Hours() > 0 {
                    isBirthday = true
                }
            }
        }

        userBirthdayText = "<i class=\"fa fa-birthday-cake\" aria-hidden=\"true\"></i> " + userData.Birthday
        if isBirthday {
            userBirthdayText = "<i class=\"fa fa-birthday-cake\" aria-hidden=\"true\"></i> Today!"
        }
    }

    tempTemplateHtml := strings.Replace(htmlTemplateString, "{USER_USERNAME}", html.EscapeString(member.User.Username), -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_NICKNAME}", html.EscapeString(member.Nick), -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_AND_NICKNAME}", html.EscapeString(userAndNick), -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_AVATAR_URL}", html.EscapeString(avatarUrl), -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_TITLE}", html.EscapeString(title), -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_BIO}", html.EscapeString(bio), -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_SERVER_LEVEL}", strconv.Itoa(m.getLevelFromExp(levelThisServerUser.Exp)), -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_SERVER_RANK}", serverRank, -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_SERVER_LEVEL_PERCENT}", strconv.Itoa(m.getProgressToNextLevelFromExp(levelThisServerUser.Exp)), -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_GLOBAL_LEVEL}", strconv.Itoa(m.getLevelFromExp(totalExp)), -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_GLOBAL_RANK}", globalRank, -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_BACKGROUND_URL}", m.GetProfileBackgroundUrl(userData.Background), -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_REP}", strconv.Itoa(userData.Rep), -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_BADGES_HTML}", badgesHTML, -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_BACKGROUND_COLOR}", html.EscapeString(backgroundColorString), -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_ACCENT_COLOR}", "#"+m.GetAccentColor(userData), -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_DETAIL_COLOR}", html.EscapeString(detailColorString), -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_TEXT_COLOR}", "#"+m.GetTextColor(userData), -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_TIME}", userTimeText, -1)
    tempTemplateHtml = strings.Replace(tempTemplateHtml, "{USER_BIRTHDAY}", userBirthdayText, -1)

    err = ioutil.WriteFile(tempTemplatePath, []byte(tempTemplateHtml), 0644)
    if err != nil {
        return []byte{}, "", err
    }

    start := time.Now()

    cmdArgs := []string{
        tempTemplatePath,
        "--window-size=400/300",
        //"--default-white-background",
        //"--quality=99",
        "--stream-type=png",
        "--timeout=15000",
        "--p:disk-cache=true",
        "--p:disk-cache-path="+cachePath,
    }
    // fmt.Println(webshotBinary, strings.Join(cmdArgs, " "))
    imgCmd := exec.Command(webshotBinary, cmdArgs...)
    imgCmd.Env = levelsEnv
    imageBytes, err := imgCmd.Output()
    if err != nil {
        return []byte{}, "", err
    }

    elapsed := time.Since(start)
    logger.VERBOSE.L("levels", fmt.Sprintf("took screenshot of profile in %s", elapsed.String()))

    err = os.Remove(tempTemplatePath)
    if err != nil {
        return []byte{}, "", err
    }

    metrics.LevelImagesGenerated.Add(1)

    if avatarUrlGif != "" && gifP == true {
        outGif := &gif.GIF{}

        decodedFirstFrame, err := png.Decode(bytes.NewReader(imageBytes))
        if err != nil {
            raven.SetUserContext(&raven.User{
                Username: member.User.Username + "#" + member.User.Discriminator,
            })
            raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
            goto ReturnImageBytes
        }
        buf := bytes.Buffer{}
        err = gif.Encode(&buf, decodedFirstFrame, nil)
        if err != nil {
            raven.SetUserContext(&raven.User{
                Username: member.User.Username + "#" + member.User.Discriminator,
            })
            raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
            goto ReturnImageBytes
        }
        firstFrame, err := gif.Decode(&buf)
        if err != nil {
            raven.SetUserContext(&raven.User{
                Username: member.User.Username + "#" + member.User.Discriminator,
            })
            raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
            goto ReturnImageBytes
        }

        avatarGifBytes, err := helpers.NetGetUAWithError(avatarUrlGif, helpers.DEFAULT_UA)
        if err != nil {
            raven.SetUserContext(&raven.User{
                Username: member.User.Username + "#" + member.User.Discriminator,
            })
            raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
            goto ReturnImageBytes
        }

        avatarGif, err := gif.DecodeAll(bytes.NewReader(avatarGifBytes))
        if err != nil {
            raven.SetUserContext(&raven.User{
                Username: member.User.Username + "#" + member.User.Discriminator,
            })
            raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
            goto ReturnImageBytes
        }

        fullRect := image.Rect(0, 0, 400, 300)
        pm := image.NewPaletted(fullRect, nil)
        q := gogif.MedianCutQuantizer{NumColor: 256}
        q.Quantize(pm, fullRect, decodedFirstFrame, image.ZP)
        draw.FloydSteinberg.Draw(pm, fullRect, firstFrame, image.ZP)

        outGif.Image = append(outGif.Image, pm)
        outGif.Delay = append(outGif.Delay, avatarGif.Delay[0])

        resizeRect := image.Rect(0, 0, 128, 128)
        cutImage := image.NewRGBA(resizeRect)
        resizedRect := image.Rect(4, 64, 4+80, 64+80)

        for i, avatarGifFrame := range avatarGif.Image {
            resizedImage := resize.Resize(80, 80, avatarGifFrame, resize.NearestNeighbor)
            draw.DrawMask(
                cutImage, resizedImage.Bounds(), resizedImage, image.ZP,
                &circle{image.Pt(40, 40), 40}, image.ZP, draw.Over)
            avatarGifFrame.Palette = append(avatarGifFrame.Palette, image.Transparent)
            paletteHasTransparency := false
            newPalette := make([]color.Color, 0)
            for i, color := range avatarGifFrame.Palette {
                if color == image.Transparent {
                    paletteHasTransparency = true
                }
                newPalette = append(newPalette, color)
                if i == 254 && paletteHasTransparency == false {
                    newPalette = append(newPalette, image.Transparent)
                    break
                }
            }
            pm = image.NewPaletted(resizedRect, newPalette)
            draw.FloydSteinberg.Draw(pm, resizedRect, cutImage, image.ZP)
            outGif.Image = append(outGif.Image, pm)
            outGif.Delay = append(outGif.Delay, avatarGif.Delay[i])
        }

        outGif.LoopCount = -1

        buf = bytes.Buffer{}
        err = gif.EncodeAll(&buf, outGif)
        if err != nil {
            raven.SetUserContext(&raven.User{
                Username: member.User.Username + "#" + member.User.Discriminator,
            })
            raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
            goto ReturnImageBytes
        }
        return buf.Bytes(), "gif", nil
    }
ReturnImageBytes:
    return imageBytes, "png", nil
}

type circle struct {
    p image.Point
    r int
}

func (c *circle) ColorModel() color.Model {
    return color.AlphaModel
}

func (c *circle) Bounds() image.Rectangle {
    return image.Rect(c.p.X-c.r, c.p.Y-c.r, c.p.X+c.r, c.p.Y+c.r)
}

func (c *circle) At(x, y int) color.Color {
    xx, yy, rr := float64(x-c.p.X)+0.5, float64(y-c.p.Y)+0.5, float64(c.r)
    if xx*xx+yy*yy < rr*rr {
        return color.Alpha{255}
    }
    return color.Alpha{0}
}

func (m *Levels) GetBackgroundColor(userUserdata DB_Profile_Userdata) string {
    if userUserdata.BackgroundColor != "" {
        return userUserdata.BackgroundColor
    } else {
        return "000000"
    }
}

func (m *Levels) GetAccentColor(userUserdata DB_Profile_Userdata) string {
    if userUserdata.AccentColor != "" {
        return userUserdata.AccentColor
    } else {
        return "46d42e"
    }
}

func (m *Levels) GetTextColor(userUserdata DB_Profile_Userdata) string {
    if userUserdata.TextColor != "" {
        return userUserdata.TextColor
    } else {
        return "ffffff"
    }
}

func (m *Levels) GetBackgroundOpacity(userUserdata DB_Profile_Userdata) string {
    if userUserdata.BackgroundOpacity != "" {
        return userUserdata.BackgroundOpacity
    } else {
        return "0.5"
    }
}

func (m *Levels) GetDetailOpacity(userUserdata DB_Profile_Userdata) string {
    if userUserdata.DetailOpacity != "" {
        return userUserdata.DetailOpacity
    } else {
        return "0.5"
    }
}

func (m *Levels) OnMessage(content string, msg *discordgo.Message, session *discordgo.Session) {
    go m.ProcessMessage(msg, session)
}

func (m *Levels) ProcessMessage(msg *discordgo.Message, session *discordgo.Session) {
    channel, err := helpers.GetChannel(msg.ChannelID)
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
    expLevelNext := m.getExpForLevel(m.getLevelFromExp(exp)+1) - m.getExpForLevel(m.getLevelFromExp(exp))
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

func (l *Levels) uploadToImgur(picData []byte) (string, error) {
    parameters := url.Values{"image": {base64.StdEncoding.EncodeToString(picData)}}

    req, err := http.NewRequest("POST", imgurApiUploadBaseUrl, strings.NewReader(parameters.Encode()))
    if err != nil {
        return "", err
    }
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    req.Header.Set("Authorization", "Client-ID "+helpers.GetConfig().Path("imgur.client_id").Data().(string))
    res, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", err
    }

    var imgurResponse ImgurResponse
    json.NewDecoder(res.Body).Decode(&imgurResponse)
    if imgurResponse.Success == false {
        return "", errors.New(fmt.Sprintf("Imgur API Error: %d (%s)", imgurResponse.Status, fmt.Sprintf("%#v", imgurResponse.Data.Error)))
    } else {
        logger.VERBOSE.L("levels", "uploaded a picture to imgur: "+imgurResponse.Data.Link)
        return imgurResponse.Data.Link, nil
    }
}
