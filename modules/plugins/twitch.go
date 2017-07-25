package plugins

import (
    "github.com/bwmarrin/discordgo"
    "strings"
    "net/http"
    "fmt"
    "github.com/Seklfreak/Robyul2/helpers"
    "time"
    "encoding/json"
    "bytes"
    "io"
    "github.com/dustin/go-humanize"
    rethink "github.com/gorethink/gorethink"
    "github.com/Seklfreak/Robyul2/logger"
    "github.com/Seklfreak/Robyul2/cache"
    "strconv"
)

type Twitch struct{}

const (
    twitchStatsEndpoint string = "https://api.twitch.tv/kraken/streams/%s"
    twitchHexColor      string = "#6441a5"
)

type DB_TwitchChannel struct {
    ID                string            `gorethink:"id,omitempty"`
    ServerID          string            `gorethink:"serverid"`
    ChannelID         string            `gorethink:"channelid"`
    TwitchChannelName string            `gorethink:"twitchchannelname"`
    IsLive            bool              `gorethink:"islive"`
}

type TwitchStatus struct {
    Stream struct {
        ID          int64 `json:"_id"`
        Game        string `json:"game"`
        Viewers     int `json:"viewers"`
        VideoHeight int `json:"video_height"`
        AverageFps  float64 `json:"average_fps"`
        Delay       int `json:"delay"`
        CreatedAt   time.Time `json:"created_at"`
        IsPlaylist  bool `json:"is_playlist"`
        Preview struct {
            Small    string `json:"small"`
            Medium   string `json:"medium"`
            Large    string `json:"large"`
            Template string `json:"template"`
        } `json:"preview"`
        Channel struct {
            Mature                       bool `json:"mature"`
            Partner                      bool `json:"partner"`
            Status                       string `json:"status"`
            BroadcasterLanguage          string `json:"broadcaster_language"`
            DisplayName                  string `json:"display_name"`
            Game                         string `json:"game"`
            Language                     string `json:"language"`
            ID                           int `json:"_id"`
            Name                         string `json:"name"`
            CreatedAt                    time.Time `json:"created_at"`
            UpdatedAt                    time.Time `json:"updated_at"`
            Delay                        interface{} `json:"delay"`
            Logo                         string `json:"logo"`
            Banner                       interface{} `json:"banner"`
            VideoBanner                  string `json:"video_banner"`
            Background                   interface{} `json:"background"`
            ProfileBanner                string `json:"profile_banner"`
            ProfileBannerBackgroundColor interface{} `json:"profile_banner_background_color"`
            URL                          string `json:"url"`
            Views                        int `json:"views"`
            Followers                    int `json:"followers"`
            Links struct {
                Self          string `json:"self"`
                Follows       string `json:"follows"`
                Commercial    string `json:"commercial"`
                StreamKey     string `json:"stream_key"`
                Chat          string `json:"chat"`
                Features      string `json:"features"`
                Subscriptions string `json:"subscriptions"`
                Editors       string `json:"editors"`
                Teams         string `json:"teams"`
                Videos        string `json:"videos"`
            } `json:"_links"`
        } `json:"channel"`
        Links struct {
            Self string `json:"self"`
        } `json:"_links"`
    } `json:"stream"`
    Links struct {
        Self    string `json:"self"`
        Channel string `json:"channel"`
    } `json:"_links"`
}

func (m *Twitch) Commands() []string {
    return []string{
        "twitch",
    }
}

func (m *Twitch) Init(session *discordgo.Session) {
    go m.checkTwitchFeedsLoop()
    logger.PLUGIN.L("twitch", "Started twitch loop (60s)")
}
func (m *Twitch) checkTwitchFeedsLoop() {
    defer func() {
        helpers.Recover()

        logger.ERROR.L("twitch", "The checkTwitchFeedsLoop died. Please investigate! Will be restarted in 60 seconds")
        time.Sleep(60 * time.Second)
        m.checkTwitchFeedsLoop()
    }()

    for {
        var entryBucket []DB_TwitchChannel
        cursor, err := rethink.Table("twitch").Run(helpers.GetDB())
        helpers.Relax(err)

        err = cursor.All(&entryBucket)
        helpers.Relax(err)

        // TODO: Check multiple entries at once
        for _, entry := range entryBucket {
            changes := false
            logger.VERBOSE.L("twitch", fmt.Sprintf("checking Twitch Channel %s", entry.TwitchChannelName))
            twitchStatus := m.getTwitchStatus(entry.TwitchChannelName)
            if entry.IsLive == false {
                if twitchStatus.Stream.ID != 0 {
                    go m.postTwitchLiveToChannel(entry.ChannelID, twitchStatus)
                    entry.IsLive = true
                    changes = true
                }
            } else {
                if twitchStatus.Stream.ID == 0 {
                    entry.IsLive = false
                    changes = true
                }
            }

            if changes == true {
                m.setEntry(entry)
            }
        }

        time.Sleep(60 * time.Second)
    }
}

func (m *Twitch) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    args := strings.Fields(content)
    if len(args) >= 1 {
        switch args[0] {
        case "add": // [p]twitch add <twitch channel name> <channel>
            helpers.RequireMod(msg, func() {
                session.ChannelTyping(msg.ChannelID)
                // get target channel
                var err error
                var targetChannel *discordgo.Channel
                var targetGuild *discordgo.Guild
                var targetTwitchChannelName string
                if len(args) >= 3 {
                    targetChannel, err = helpers.GetChannelFromMention(msg, args[2])
                    if err != nil {
                        session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
                        return
                    }
                    targetTwitchChannelName = args[1]
                } else {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
                    return
                }
                targetGuild, err = helpers.GetGuild(targetChannel.GuildID)
                helpers.Relax(err)
                // create new entry in db
                entry := m.getEntryByOrCreateEmpty("id", "")
                entry.ServerID = targetChannel.GuildID
                entry.ChannelID = targetChannel.ID
                entry.TwitchChannelName = targetTwitchChannelName
                m.setEntry(entry)

                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.twitch.channel-added-success", targetTwitchChannelName, entry.ChannelID))
                logger.INFO.L("twitch", fmt.Sprintf("Added Twitch Channel %s to Channel %s (#%s) on Guild %s (#%s)", targetTwitchChannelName, targetChannel.Name, entry.ChannelID, targetGuild.Name, targetGuild.ID))
            })
        case "delete", "del": // [p]twitch delete <id>
            helpers.RequireMod(msg, func() {
                if len(args) >= 2 {
                    session.ChannelTyping(msg.ChannelID)
                    entryId := args[1]
                    entryBucket := m.getEntryBy("id", entryId)
                    if entryBucket.ID != "" {
                        m.deleteEntryById(entryBucket.ID)

                        session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.twitch.channel-delete-success", entryBucket.TwitchChannelName))
                        logger.INFO.L("twitch", fmt.Sprintf("Deleted Twitch Channel %s", entryBucket.TwitchChannelName))
                    } else {
                        session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.twitch.channel-delete-not-found-error"))
                        return
                    }

                } else {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                    return
                }
            })
        case "list": // [p]twitch list
            currentChannel, err := helpers.GetChannel(msg.ChannelID)
            helpers.Relax(err)
            var entryBucket []DB_TwitchChannel
            listCursor, err := rethink.Table("twitch").Filter(
                rethink.Row.Field("serverid").Eq(currentChannel.GuildID),
            ).Run(helpers.GetDB())
            helpers.Relax(err)
            defer listCursor.Close()
            err = listCursor.All(&entryBucket)

            if err == rethink.ErrEmptyResult || len(entryBucket) <= 0 {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.twitch.channel-list-no-channels-error"))
                return
            } else if err != nil {
                helpers.Relax(err)
            }

            resultMessage := ""
            for _, entry := range entryBucket {
                resultMessage += fmt.Sprintf("`%s`: Twitch Channel `%s` posting to <#%s>\n", entry.ID, entry.TwitchChannelName, entry.ChannelID)
            }
            resultMessage += fmt.Sprintf("Found **%d** Twitch Channels in total.", len(entryBucket))
            for _, resultPage := range helpers.Pagify(resultMessage, "\n") {
                _, err := session.ChannelMessageSend(msg.ChannelID, resultPage)
                helpers.Relax(err)
            }
        default:
            if args[0] == "" {
                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
                helpers.Relax(err)
                return
            }
            session.ChannelTyping(msg.ChannelID)
            twitchStatus := m.getTwitchStatus(args[0])
            if twitchStatus.Stream.ID == 0 {
                _, err := session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.twitch.no-channel-information"))
                helpers.Relax(err)
                return
            } else {
                twitchChannelEmbed := &discordgo.MessageEmbed{
                    Title:  helpers.GetTextF("plugins.twitch.channel-embed-title", twitchStatus.Stream.Channel.DisplayName, twitchStatus.Stream.Channel.Name),
                    URL:    twitchStatus.Stream.Channel.URL,
                    Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.twitch.embed-footer")},
                    Fields: []*discordgo.MessageEmbedField{
                        {Name: "Viewers", Value: humanize.Comma(int64(twitchStatus.Stream.Viewers)), Inline: true},
                        {Name: "Followers", Value: humanize.Comma(int64(twitchStatus.Stream.Channel.Followers)), Inline: true},
                        {Name: "Total Views", Value: humanize.Comma(int64(twitchStatus.Stream.Channel.Views)), Inline: true}},
                    Color: helpers.GetDiscordColorFromHex(twitchHexColor),
                }
                if twitchStatus.Stream.Channel.Logo != "" {
                    twitchChannelEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: twitchStatus.Stream.Channel.Logo}
                }
                if twitchStatus.Stream.Channel.VideoBanner != "" {
                    twitchChannelEmbed.Image = &discordgo.MessageEmbedImage{URL: twitchStatus.Stream.Channel.VideoBanner}
                }
                if twitchStatus.Stream.Game != "" {
                    twitchChannelEmbed.Description = fmt.Sprintf("playing **%s**", twitchStatus.Stream.Game)
                }
                _, err := session.ChannelMessageSendEmbed(msg.ChannelID, twitchChannelEmbed)
                helpers.Relax(err)
                return
            }
        }
    }
}

func (m *Twitch) getTwitchStatus(name string) TwitchStatus {
    var twitchStatus TwitchStatus

    client := &http.Client{}

    request, err := http.NewRequest("GET", fmt.Sprintf(twitchStatsEndpoint, name), nil)
    if err != nil {
        panic(err)
    }

    request.Header.Set("User-Agent", helpers.DEFAULT_UA)
    request.Header.Set("Client-ID", helpers.GetConfig().Path("twitch.token").Data().(string))

    response, err := client.Do(request)
    helpers.Relax(err)

    defer response.Body.Close()

    buf := bytes.NewBuffer(nil)
    _, err = io.Copy(buf, response.Body)
    helpers.Relax(err)

    json.Unmarshal(buf.Bytes(), &twitchStatus)
    return twitchStatus
}

func (m *Twitch) getEntryBy(key string, id string) DB_TwitchChannel {
    var entryBucket DB_TwitchChannel
    listCursor, err := rethink.Table("twitch").Filter(
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

func (m *Twitch) getEntryByOrCreateEmpty(key string, id string) DB_TwitchChannel {
    var entryBucket DB_TwitchChannel
    listCursor, err := rethink.Table("twitch").Filter(
        rethink.Row.Field(key).Eq(id),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.One(&entryBucket)

    // If user has no DB entries create an empty document
    if err == rethink.ErrEmptyResult {
        insert := rethink.Table("twitch").Insert(DB_TwitchChannel{})
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

func (m *Twitch) setEntry(entry DB_TwitchChannel) {
    _, err := rethink.Table("twitch").Update(entry).Run(helpers.GetDB())
    helpers.Relax(err)
}

func (m *Twitch) deleteEntryById(id string) {
    _, err := rethink.Table("twitch").Filter(
        rethink.Row.Field("id").Eq(id),
    ).Delete().RunWrite(helpers.GetDB())
    helpers.Relax(err)
}

func (m *Twitch) postTwitchLiveToChannel(channelID string, twitchStatus TwitchStatus) {
    twitchStreamName := twitchStatus.Stream.Channel.DisplayName
    if strings.ToLower(twitchStatus.Stream.Channel.Name) != strings.ToLower(twitchStatus.Stream.Channel.DisplayName) {
        twitchStreamName += fmt.Sprintf(" (%s)", twitchStatus.Stream.Channel.Name)
    }

    twitchChannelEmbed := &discordgo.MessageEmbed{
        Title:  helpers.GetTextF("plugins.twitch.wentlive-embed-title", twitchStreamName),
        URL:    twitchStatus.Stream.Channel.URL,
        Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.twitch.embed-footer")},
        Fields: []*discordgo.MessageEmbedField{
            {Name: "Followers", Value: humanize.Comma(int64(twitchStatus.Stream.Channel.Followers)), Inline: true},
            {Name: "Total Views", Value: humanize.Comma(int64(twitchStatus.Stream.Channel.Views)), Inline: true}},
        Color: helpers.GetDiscordColorFromHex(twitchHexColor),
    }
    if twitchStatus.Stream.Channel.Logo != "" {
        twitchChannelEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: twitchStatus.Stream.Channel.Logo}
    }
    if twitchStatus.Stream.Preview.Medium != "" {
        twitchChannelEmbed.Image = &discordgo.MessageEmbedImage{URL: twitchStatus.Stream.Preview.Medium + "?" + strconv.FormatInt(time.Now().Unix(), 10)}
        fmt.Println(twitchStatus.Stream.Preview.Medium + "?" + strconv.FormatInt(time.Now().Unix(), 10))
    }
    if twitchStatus.Stream.Game != "" {
        twitchChannelEmbed.Description = fmt.Sprintf("playing **%s**", twitchStatus.Stream.Game)
    }
    _, err := cache.GetSession().ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
        Content: fmt.Sprintf("<%s>", twitchStatus.Stream.Channel.URL),
        Embed: twitchChannelEmbed,
    })
    helpers.Relax(err)
}
