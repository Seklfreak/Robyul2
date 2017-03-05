package plugins

import (
    "bytes"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "github.com/Jeffail/gabs"
    "github.com/Seklfreak/Robyul2/cache"
    "github.com/Seklfreak/Robyul2/helpers"
    "github.com/Seklfreak/Robyul2/logger"
    "github.com/bwmarrin/discordgo"
    "github.com/dustin/go-humanize"
    rethink "github.com/gorethink/gorethink"
    "github.com/satori/go.uuid"
    "io"
    "net/http"
    "net/url"
    "strconv"
    "strings"
    "time"
)

type Instagram struct{}

type DB_Instagram_Entry struct {
    ID          string              `gorethink:"id,omitempty"`
    ServerID    string              `gorethink:"serverid"`
    ChannelID   string              `gorethink:"channelid"`
    Username    string              `gorethink:"username"`
    PostedPosts []DB_Instagram_Post `gorethink:"posted_posts"`
}

type DB_Instagram_Post struct {
    ID        string `gorethink:"id,omitempty"`
    CreatedAt int    `gorethink:"createdat"`
}

type Instagram_User struct {
    Biography      string `json:"biography"`
    ExternalURL    string `json:"external_url"`
    FollowerCount  int    `json:"follower_count"`
    FollowingCount int    `json:"following_count"`
    FullName       string `json:"full_name"`
    ProfilePic struct {
        URL string `json:"url"`
    } `json:"hd_profile_pic_url_info"`
    IsBusiness bool             `json:"is_business"`
    IsFavorite bool             `json:"is_favorite"`
    IsPrivate  bool             `json:"is_private"`
    IsVerified bool             `json:"is_verified"`
    MediaCount int              `json:"media_count"`
    Pk         int              `json:"pk"`
    Username   string           `json:"username"`
    Posts      []Instagram_Post `json:"-"`
}

type Instagram_Post struct {
    Caption struct {
        Text      string `json:"text"`
        CreatedAt int    `json:"created_at"`
    } `json:"caption"`
    ID string `json:"id"`
    ImageVersions2 struct {
        Candidates []struct {
            Height int    `json:"height"`
            URL    string `json:"url"`
            Width  int    `json:"width"`
        } `json:"candidates"`
    } `json:"image_versions2"`
    MediaType int    `json:"media_type"`
    Code      string `json:"code"`
}

var (
    usedUuid   string
    sessionId  string
    rankToken  string
    httpClient *http.Client
)

const (
    hexColor              string = "#fcaf45"
    apiBaseUrl            string = "https://i.instagram.com/api/v1/%s"
    apiUserAgent          string = "Instagram 9.2.0 Android (18/4.3; 320dpi; 720x1280; Xiaomi; HM 1SW; armani; qcom; en_US)"
    instagramSignKey      string = "012a54f51c49aa8c5c322416ab1410909add32c966bbaa0fe3dc58ac43fd7ede"
    deviceId              string = "android-3deeb2d04b2ab0ee" // TODO: generate a random device id
    instagramFriendlyUser string = "https://www.instagram.com/%s/"
    instagramFriendlyPost string = "https://www.instagram.com/p/%s/"
)

func (m *Instagram) Commands() []string {
    return []string{
        "instagram",
    }
}

func (m *Instagram) Init(session *discordgo.Session) {
    m.login()

    go func() {
        defer helpers.Recover()

        for {
            var entryBucket []DB_Instagram_Entry
            cursor, err := rethink.Table("instagram").Run(helpers.GetDB())
            helpers.Relax(err)

            err = cursor.All(&entryBucket)
            helpers.Relax(err)

            // TODO: Check multiple entries at once
            for _, entry := range entryBucket {
                changes := false
                logger.VERBOSE.L("instagram", fmt.Sprintf("checking Instagram Account @%s", entry.Username))

                instagramUser := m.lookupInstagramUser(entry.Username)
                if instagramUser.Username == "" {
                    logger.ERROR.L("instagram", fmt.Sprintf("updating instagram account @%s failed", entry.Username))
                    continue
                }

                // https://github.com/golang/go/wiki/SliceTricks#reversing
                for i := len(instagramUser.Posts)/2 - 1; i >= 0; i-- {
                    opp := len(instagramUser.Posts) - 1 - i
                    instagramUser.Posts[i], instagramUser.Posts[opp] = instagramUser.Posts[opp], instagramUser.Posts[i]
                }

                for _, post := range instagramUser.Posts {
                    postAlreadyPosted := false
                    for _, postedPosts := range entry.PostedPosts {
                        if postedPosts.ID == post.ID {
                            postAlreadyPosted = true
                        }
                    }
                    if postAlreadyPosted == false {
                        logger.VERBOSE.L("instagram", fmt.Sprintf("Posting Post: #%s", post.ID))
                        entry.PostedPosts = append(entry.PostedPosts, DB_Instagram_Post{ID: post.ID, CreatedAt: post.Caption.CreatedAt})
                        changes = true
                        go m.postPostToChannel(entry.ChannelID, post, instagramUser)
                    }

                }
                if changes == true {
                    m.setEntry(entry)
                }
            }

            time.Sleep(10 * time.Minute)
        }
    }()

    logger.PLUGIN.L("instagram", "Started Instagram loop (10m)")
}

func (m *Instagram) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    content = strings.Trim(content, " ")
    args := strings.Split(content, " ")
    if len(args) >= 1 {
        switch args[0] {
        case "add": // [p]instagram add <instagram account name (with or without @)> <discord channel>
            helpers.RequireMod(msg, func() {
                session.ChannelTyping(msg.ChannelID)
                // get target channel
                var err error
                var targetChannel *discordgo.Channel
                var targetGuild *discordgo.Guild
                if len(args) >= 3 {
                    targetChannel, err = helpers.GetChannelFromMention(args[len(args)-1])
                    if err != nil {
                        session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
                        return
                    }
                } else {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
                    return
                }
                targetGuild, err = session.Guild(targetChannel.GuildID)
                helpers.Relax(err)
                // get instagram account
                instagramUsername := strings.Replace(args[1], "@", "", 1)
                instagramUser := m.lookupInstagramUser(instagramUsername)
                if instagramUser.Username == "" {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-not-found"))
                    return
                }
                // Create DB Entries
                var dbPosts []DB_Instagram_Post
                for _, post := range instagramUser.Posts {
                    postEntry := DB_Instagram_Post{ID: post.ID, CreatedAt: post.Caption.CreatedAt}
                    dbPosts = append(dbPosts, postEntry)

                }
                // create new entry in db
                entry := m.getEntryByOrCreateEmpty("id", "")
                entry.ServerID = targetChannel.GuildID
                entry.ChannelID = targetChannel.ID
                entry.Username = instagramUser.Username
                entry.PostedPosts = dbPosts
                m.setEntry(entry)

                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-added-success", entry.Username, entry.ChannelID))
                logger.INFO.L("instagram", fmt.Sprintf("Added Instagram Account @%s to Channel %s (#%s) on Guild %s (#%s)", entry.Username, targetChannel.Name, entry.ChannelID, targetGuild.Name, targetGuild.ID))
            })
        case "delete": // [p]instagram delete <id>
            helpers.RequireMod(msg, func() {
                if len(args) >= 2 {
                    session.ChannelTyping(msg.ChannelID)
                    entryId := args[1]
                    entryBucket := m.getEntryBy("id", entryId)
                    if entryBucket.ID != "" {
                        m.deleteEntryById(entryBucket.ID)

                        session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-delete-success", entryBucket.Username))
                        logger.INFO.L("instagram", fmt.Sprintf("Deleted Instagram Account @%s", entryBucket.Username))
                    } else {
                        session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.instagram.account-delete-not-found-error"))
                        return
                    }
                } else {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                    return
                }
            })
        case "list": // [p]instagram list
            currentChannel, err := session.Channel(msg.ChannelID)
            helpers.Relax(err)
            var entryBucket []DB_Instagram_Entry
            listCursor, err := rethink.Table("instagram").Filter(
                rethink.Row.Field("serverid").Eq(currentChannel.GuildID),
            ).Run(helpers.GetDB())
            helpers.Relax(err)
            defer listCursor.Close()
            err = listCursor.All(&entryBucket)

            if err == rethink.ErrEmptyResult || len(entryBucket) <= 0 {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-list-no-accounts-error"))
                return
            } else if err != nil {
                helpers.Relax(err)
            }

            resultMessage := ""
            for _, entry := range entryBucket {
                resultMessage += fmt.Sprintf("`%s`: Instagram Account `@%s` posting to <#%s>\n", entry.ID, entry.Username, entry.ChannelID)
            }
            resultMessage += fmt.Sprintf("Found **%d** Instagram Accounts in total.", len(entryBucket))
            session.ChannelMessageSend(msg.ChannelID, resultMessage) // TODO: Pagify message
        default:
            session.ChannelTyping(msg.ChannelID)
            instagramUsername := strings.Replace(args[0], "@", "", 1)
            instagramUser := m.lookupInstagramUser(instagramUsername)

            if instagramUser.Username == "" {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.instagram.account-not-found"))
                return
            }

            instagramNameModifier := ""
            if instagramUser.IsVerified {
                instagramNameModifier += " :ballot_box_with_check:"
            }
            if instagramUser.IsPrivate {
                instagramNameModifier += " :lock:"
            }
            if instagramUser.IsBusiness {
                instagramNameModifier += " :office:"
            }
            if instagramUser.IsFavorite {
                instagramNameModifier += " :star:"
            }
            accountEmbed := &discordgo.MessageEmbed{
                Title:       helpers.GetTextF("plugins.instagram.account-embed-title", instagramUser.FullName, instagramUser.Username, instagramNameModifier),
                URL:         fmt.Sprintf(instagramFriendlyUser, instagramUser.Username),
                Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: instagramUser.ProfilePic.URL},
                Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.instagram.embed-footer")},
                Description: instagramUser.Biography,
                Fields: []*discordgo.MessageEmbedField{
                    {Name: "Followers", Value: humanize.Comma(int64(instagramUser.FollowerCount)), Inline: true},
                    {Name: "Following", Value: humanize.Comma(int64(instagramUser.FollowingCount)), Inline: true},
                    {Name: "Posts", Value: humanize.Comma(int64(instagramUser.MediaCount)), Inline: true}},
                Color: helpers.GetDiscordColorFromHex(hexColor),
            }
            if instagramUser.ExternalURL != "" {
                accountEmbed.Fields = append(accountEmbed.Fields, &discordgo.MessageEmbedField{
                    Name:   "Website",
                    Value:  instagramUser.ExternalURL,
                    Inline: true,
                })
            }
            _, _ = session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("<%s>", fmt.Sprintf(instagramFriendlyUser, instagramUser.Username)))
            _, err := session.ChannelMessageSendEmbed(msg.ChannelID, accountEmbed)
            helpers.Relax(err)
            return
        }
    } else {
        session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
    }
}

func (m *Instagram) postPostToChannel(channelID string, post Instagram_Post, instagramUser Instagram_User) {
    instagramNameModifier := ""
    if instagramUser.IsVerified {
        instagramNameModifier += " :ballot_box_with_check:"
    }
    if instagramUser.IsPrivate {
        instagramNameModifier += " :lock:"
    }
    if instagramUser.IsBusiness {
        instagramNameModifier += " :office:"
    }
    if instagramUser.IsFavorite {
        instagramNameModifier += " :star:"
    }

    mediaModifier := "Picture"
    if post.MediaType == 2 {
        mediaModifier = "Video"
    }

    channelEmbed := &discordgo.MessageEmbed{
        Title:       helpers.GetTextF("plugins.instagram.post-embed-title", instagramUser.FullName, instagramUser.Username, instagramNameModifier, mediaModifier),
        URL:         fmt.Sprintf(instagramFriendlyPost, post.Code),
        Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: instagramUser.ProfilePic.URL},
        Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.instagram.embed-footer")},
        Description: post.Caption.Text,
        Color:       helpers.GetDiscordColorFromHex(hexColor),
    }

    if len(post.ImageVersions2.Candidates) > 0 {
        channelEmbed.Image = &discordgo.MessageEmbedImage{URL: post.ImageVersions2.Candidates[0].URL}
    }

    _, _ = cache.GetSession().ChannelMessageSend(channelID, fmt.Sprintf("<%s>", fmt.Sprintf(instagramFriendlyPost, post.Code)))
    _, err := cache.GetSession().ChannelMessageSendEmbed(channelID, channelEmbed)
    if err != nil {
        logger.ERROR.L("vlive", fmt.Sprintf("posting post: #%s to channel: #%s failed: %s", post.ID, channelID, err))
    }
}

func (m *Instagram) signDataValue(data string) string {
    key := hmac.New(sha256.New, []byte(instagramSignKey))
    key.Write([]byte(data))
    return fmt.Sprintf("ig_sig_key_version=%s&signed_body=%s.%s", "4", hex.EncodeToString(key.Sum(nil)), url.QueryEscape(data))
}

func (m *Instagram) applyHeaders(request *http.Request) {
    request.Header.Add("Connection", "close")
    request.Header.Add("Accept", "*/*")
    request.Header.Add("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
    request.Header.Add("Cookie2", "$Version=1")
    request.Header.Add("Accept-Language", "en-US")
    request.Header.Add("User-Agent", apiUserAgent)
    if sessionId != "" {
        request.Header.Add("Cookie", fmt.Sprintf("sessionid=%s", sessionId))
    }
}

// quick port of https://github.com/LevPasha/Instagram-API-python
func (m *Instagram) login() {
    usedUuid = uuid.NewV4().String()
    // get csrf token
    signupEndpoint := fmt.Sprintf(apiBaseUrl, fmt.Sprintf("si/fetch_headers/?challenge_type=signup&guid=%s", usedUuid))
    httpClient = &http.Client{}
    request, err := http.NewRequest("GET", signupEndpoint, nil)
    helpers.Relax(err)
    m.applyHeaders(request)
    response, err := httpClient.Do(request)
    helpers.Relax(err)
    defer response.Body.Close()
    csrfToken := ""
    for _, cookie := range response.Cookies() {
        if cookie.Name == "csrftoken" {
            csrfToken = cookie.Value
        }
    }
    if csrfToken == "" {
        helpers.Relax(errors.New("Unable to get CSRF Token while trying to authenticate to instagram."))
    }
    // login
    loginEndpoint := fmt.Sprintf(apiBaseUrl, "accounts/login/")
    jsonParsed, err := gabs.ParseJSON([]byte(fmt.Sprintf(
        `{"phone_id": "%s",
    "_csrftoken": "%s",
    "username": "%s",
    "guid": "%s",
    "device_id": "%s",
    "password": "%s",
    "login_attempt_count": "0"}`, uuid.NewV4().String(), csrfToken, helpers.GetConfig().Path("instagram.username").Data().(string), usedUuid, deviceId, helpers.GetConfig().Path("instagram.password").Data().(string))))
    helpers.Relax(err)
    request, err = http.NewRequest("POST", loginEndpoint, strings.NewReader(m.signDataValue(jsonParsed.String())))
    m.applyHeaders(request)
    response, err = httpClient.Do(request)
    helpers.Relax(err)
    defer response.Body.Close()
    csrfToken = ""
    sessionId = ""
    for _, cookie := range response.Cookies() {
        if cookie.Name == "csrftoken" {
            csrfToken = cookie.Value
        }
        if cookie.Name == "sessionid" {
            sessionId = cookie.Value
        }
    }
    if csrfToken == "" {
        helpers.Relax(errors.New("Unable to get CSRF Token while trying to authenticate to instagram."))
    }
    if sessionId == "" {
        helpers.Relax(errors.New("Unable to get Session ID while trying to authenticate to instagram."))
    }
    if response.StatusCode != 200 {
        helpers.Relax(errors.New(fmt.Sprintf("Instagram login failed, unexpected status code: %d", response.StatusCode)))
    }
    buf := bytes.NewBuffer(nil)
    _, err = io.Copy(buf, response.Body)
    helpers.Relax(err)
    jsonResult, err := gabs.ParseJSON(buf.Bytes())
    helpers.Relax(err)
    usernameIdFloat, ok := jsonResult.Path("logged_in_user.pk").Data().(float64)
    if ok == false {
        helpers.Relax(errors.New("Unable to get username id from instagram login reply"))
    }
    usernameId := strconv.FormatFloat(usernameIdFloat, 'f', 0, 64)
    usernameLoggedIn, ok := jsonResult.Path("logged_in_user.username").Data().(string)
    if ok == false {
        helpers.Relax(errors.New("Unable to get username from instagram login reply"))
    }
    logger.VERBOSE.L("instagram", fmt.Sprintf("logged in as @%s", usernameLoggedIn))
    rankToken = fmt.Sprintf("%s_%s", usernameId, usedUuid)
}

func (m *Instagram) lookupInstagramUser(username string) Instagram_User {
    var instagramUser Instagram_User

    userEndpoint := fmt.Sprintf(apiBaseUrl, fmt.Sprintf("users/%s/usernameinfo/", username))
    request, err := http.NewRequest("GET", userEndpoint, nil)
    helpers.Relax(err)
    m.applyHeaders(request)
    response, err := httpClient.Do(request)
    helpers.Relax(err)
    buf := bytes.NewBuffer(nil)
    _, err = io.Copy(buf, response.Body)
    helpers.Relax(err)
    jsonResult, err := gabs.ParseJSON(buf.Bytes())
    helpers.Relax(err)
    json.Unmarshal([]byte(jsonResult.Path("user").String()), &instagramUser)

    userFeedEndpoint := fmt.Sprintf(apiBaseUrl, fmt.Sprintf("feed/user/%s/?max_id=%s&min_timestamp=%s&rank_token=%s&ranked_content=true", strconv.Itoa(instagramUser.Pk), "", "", rankToken))
    request, err = http.NewRequest("GET", userFeedEndpoint, nil)
    helpers.Relax(err)
    m.applyHeaders(request)
    response, err = httpClient.Do(request)
    helpers.Relax(err)
    buf = bytes.NewBuffer(nil)
    _, err = io.Copy(buf, response.Body)
    helpers.Relax(err)
    jsonResult, err = gabs.ParseJSON(buf.Bytes())
    helpers.Relax(err)

    var instagramPosts []Instagram_Post
    instagramPostsJsons, err := jsonResult.Path("items").Children()
    helpers.Relax(err)
    for _, instagramPostJson := range instagramPostsJsons {
        var instagramPost Instagram_Post
        json.Unmarshal([]byte(instagramPostJson.String()), &instagramPost)
        instagramPosts = append(instagramPosts, instagramPost)
    }
    instagramUser.Posts = instagramPosts

    return instagramUser
}

func (m *Instagram) getEntryBy(key string, id string) DB_Instagram_Entry {
    var entryBucket DB_Instagram_Entry
    listCursor, err := rethink.Table("instagram").Filter(
        rethink.Row.Field(key).Eq(id),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.One(&entryBucket)

    if err == rethink.ErrEmptyResult {
        return entryBucket
    } else if err != nil {
        panic(err)
    }

    return entryBucket
}

func (m *Instagram) getEntryByOrCreateEmpty(key string, id string) DB_Instagram_Entry {
    var entryBucket DB_Instagram_Entry
    listCursor, err := rethink.Table("instagram").Filter(
        rethink.Row.Field(key).Eq(id),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.One(&entryBucket)

    // If user has no DB entries create an empty document
    if err == rethink.ErrEmptyResult {
        insert := rethink.Table("instagram").Insert(DB_Instagram_Entry{})
        res, e := insert.RunWrite(helpers.GetDB())
        // If the creation was successful read the document
        if e != nil {
            panic(e)
        } else {
            return m.getEntryByOrCreateEmpty("id", res.GeneratedKeys[0])
        }
    } else if err != nil {
        panic(err)
    }

    return entryBucket
}

func (m *Instagram) setEntry(entry DB_Instagram_Entry) {
    _, err := rethink.Table("instagram").Update(entry).Run(helpers.GetDB())
    helpers.Relax(err)
}

func (m *Instagram) deleteEntryById(id string) {
    _, err := rethink.Table("instagram").Filter(
        rethink.Row.Field("id").Eq(id),
    ).Delete().RunWrite(helpers.GetDB())
    helpers.Relax(err)
}
