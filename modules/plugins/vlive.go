package plugins

import (
    "encoding/json"
    "errors"
    "fmt"
    "github.com/PuerkitoBio/goquery"
    "github.com/Seklfreak/Robyul2/cache"
    "github.com/Seklfreak/Robyul2/helpers"
    "github.com/Seklfreak/Robyul2/logger"
    "github.com/bwmarrin/discordgo"
    "github.com/dustin/go-humanize"
    rethink "github.com/gorethink/gorethink"
    "regexp"
    "strings"
    "time"
    "sync"
)

var (
    VliveAppId string = "8c6cc7b45d2568fb668be6e05b6e5a3b"
)

const (
    VliveEndpointDecodeChannelCode string = "http://api.vfan.vlive.tv/vproxy/channelplus/decodeChannelCode?app_id=%s&channelCode=%s"
    VliveEndpointChannel           string = "http://api.vfan.vlive.tv/channel.%d?app_id=%s&fields=channel_seq,channel_code,type,channel_name,fan_count,channel_cover_img,channel_profile_img,representative_color,celeb_board"
    VliveEndpointChannelVideoList  string = "http://api.vfan.vlive.tv/vproxy/channelplus/getChannelVideoList?app_id=%s&channelSeq=%d&maxNumOfRows=%d"
    VliveEndpointUpcomingVideoList string = "http://api.vfan.vlive.tv/vproxy/channelplus/getUpcomingVideoList?app_id=%s&channelSeq=%d&maxNumOfRows=%d"
    VliveEndpointNotices           string = "http://notice.vlive.tv/notice/list.json?channel_seq=%d"
    VliveEndpointCeleb             string = "http://api.vfan.vlive.tv/board.%d/posts?app_id=%s"
    VliveFriendlyChannel           string = "http://channels.vlive.tv/%s"
    VliveFriendlyVideo             string = "http://www.vlive.tv/video/%d"
    VliveFriendlyNotice            string = "http://channels.vlive.tv/%s/notice/%d"
    VliveFriendlyCeleb             string = "http://channels.vlive.tv/%s/celeb/%s"
    VliveFriendlySearch            string = "http://www.vlive.tv/search/all?query=%s"
    ChannelIdRegex                 string = "(http(s)?://channels.vlive.tv)?(/)?(channels/)?([A-Z0-9]+)(/video)?"
)

type VLive struct{}

type DB_VLive_Entry struct {
    ID             string            `gorethink:"id,omitempty"`
    ServerID       string            `gorethink:"serverid"`
    ChannelID      string            `gorethink:"channelid"`
    VLiveChannel   DB_VLive_Channel  `gorethink:"vlivechannel"`
    PostedUpcoming []DB_VLive_Video  `gorethink:"posted_upcoming"`
    PostedLive     []DB_VLive_Video  `gorethink:"posted_live"`
    PostedVOD      []DB_VLive_Video  `gorethink:"posted_vod"`
    PostedNotices  []DB_VLive_Notice `gorethink:"posted_notices"`
    PostedCelebs   []DB_VLive_Celeb  `gorethink:"posted_celebs"`
    MentionRoleID  string `gorethink:"mention_role_id"`
}

type DB_VLive_Channel struct {
    Seq           int64  `gorethink:"seq,omitempty" json:"channel_seq"`
    Code          string `gorethink:"code,omitempty" json:"channel_code"`
    Type          string `json:"type"`
    Name          string `json:"channel_name"`
    Followers     int64  `json:"fan_count"`
    CoverImgUrl   string `json:"channel_cover_img"`
    ProfileImgUrl string `json:"channel_profile_img"`
    Color         string `json:"representative_color"`
    TotalVideos   int64  `json:"-"`
    CelebBoard struct {
        BoardID int64 `json:"board_id"`
    } `json:"celeb_board"`
    Upcoming []DB_VLive_Video  `gorethink:"upcoming" json:"-"`
    Live     []DB_VLive_Video  `gorethink:"live" json:"-"`
    VOD      []DB_VLive_Video  `gorethink:"vod" json:"-"`
    Notices  []DB_VLive_Notice `gorethink:"notices" json:"-"`
    Celebs   []DB_VLive_Celeb  `gorethink:"celebs" json:"-"`
    Url      string            `json:"-"`
}

type DB_VLive_Video struct {
    Seq       int64  `gorethink:"seq,omitempty" json:"videoSeq"`
    Title     string `json:"title"`
    Plays     int64  `json:"playCount"`
    Likes     int64  `json:"likeCount"`
    Comments  int64  `json:"commentCount"`
    Thumbnail string `json:"thumbnail"`
    Date      string `json:"onAirStartAt"`
    Playtime  int64  `json:"playTime"`
    Type      string `json:"videoType"`
    Url       string `json:"-"`
}

type DB_VLive_Notice struct {
    Number   int64  `gorethink:"number,omitempty" json:"noticeNo"`
    Title    string `json:"title"`
    ImageUrl string `json:"listImageUrl"`
    Summary  string `json:"summary"`
    Url      string `json:"-"`
}

type DB_VLive_Celeb struct {
    ID      string `gorethink:"id,omitempty" json:"post_id"`
    Summary string `json:"body_summary"`
    Url     string `json:"-"`
}

type VLive_Safe_Entries struct {
    entries []DB_VLive_Entry
    mux     sync.Mutex
}

func (r *VLive) Commands() []string {
    return []string{
        "vlive",
    }
}

func (r *VLive) Init(session *discordgo.Session) {
    go r.checkVliveFeedsLoop()
    logger.PLUGIN.L("VLive", "Started vlive loop (0s)")
}
func (r *VLive) checkVliveFeedsLoop() {
    var safeEntries VLive_Safe_Entries

    defer func() {
        helpers.Recover()

        logger.ERROR.L("VLive", "The checkVliveFeedsLoop died. Please investigate! Will be restarted in 60 seconds")
        time.Sleep(60 * time.Second)
        r.checkVliveFeedsLoop()
    }()

    for {
        cursor, err := rethink.Table("vlive").Run(helpers.GetDB())
        helpers.Relax(err)

        err = cursor.All(&safeEntries.entries)
        helpers.Relax(err)

        // TODO: Check multiple entries at once
        for _, entry := range safeEntries.entries {
            safeEntries.mux.Lock()
            changes := false
            logger.VERBOSE.L("vlive", fmt.Sprintf("checking V Live Channel %s for Channel #%s", entry.VLiveChannel.Name, entry.ChannelID))
            updatedVliveChannel, err := r.getVLiveChannelByVliveChannelId(entry.VLiveChannel.Code)
            if err != nil {
                logger.ERROR.L("vlive", fmt.Sprintf("updating vlive channel %s failed: %s", entry.VLiveChannel.Name, err.Error()))
                safeEntries.mux.Unlock()
                continue
            }
            for _, vod := range updatedVliveChannel.VOD {
                videoAlreadyPosted := false
                for _, postedVod := range entry.PostedVOD {
                    if postedVod.Seq == vod.Seq {
                        videoAlreadyPosted = true
                    }
                }
                if videoAlreadyPosted == false {
                    logger.VERBOSE.L("vlive", fmt.Sprintf("Posting VOD: #%d", vod.Seq))
                    entry.PostedVOD = append(entry.PostedVOD, vod)
                    changes = true
                    go r.postVodToChannel(entry, vod, updatedVliveChannel)
                }
            }
            for _, upcoming := range updatedVliveChannel.Upcoming {
                videoAlreadyPosted := false
                for _, postedUpcoming := range entry.PostedUpcoming {
                    if postedUpcoming.Seq == upcoming.Seq {
                        videoAlreadyPosted = true
                    }
                }
                if videoAlreadyPosted == false {
                    logger.VERBOSE.L("vlive", fmt.Sprintf("Posting Upcoming: #%d", upcoming.Seq))
                    entry.PostedUpcoming = append(entry.PostedUpcoming, upcoming)
                    changes = true
                    go r.postUpcomingToChannel(entry, upcoming, updatedVliveChannel)
                }
            }
            for _, live := range updatedVliveChannel.Live {
                videoAlreadyPosted := false
                for _, postedLive := range entry.PostedLive {
                    if postedLive.Seq == live.Seq {
                        videoAlreadyPosted = true
                    }
                }
                if videoAlreadyPosted == false {
                    logger.VERBOSE.L("vlive", fmt.Sprintf("Posting Live: #%d", live.Seq))
                    entry.PostedLive = append(entry.PostedLive, live)
                    changes = true
                    go r.postLiveToChannel(entry, live, updatedVliveChannel)
                }
            }
            for _, notice := range updatedVliveChannel.Notices {
                noticeAlreadyPosted := false
                for _, postedNotice := range entry.PostedNotices {
                    if postedNotice.Number == notice.Number {
                        noticeAlreadyPosted = true
                    }
                }
                if noticeAlreadyPosted == false {
                    logger.VERBOSE.L("vlive", fmt.Sprintf("Posting Notice: #%d", notice.Number))
                    entry.PostedNotices = append(entry.PostedNotices, notice)
                    changes = true
                    go r.postNoticeToChannel(entry, notice, updatedVliveChannel)
                }
            }
            for _, celeb := range updatedVliveChannel.Celebs {
                celebAlreadyPosted := false
                for _, postedCeleb := range entry.PostedCelebs {
                    if postedCeleb.ID == celeb.ID {
                        celebAlreadyPosted = true
                    }
                }
                if celebAlreadyPosted == false {
                    logger.VERBOSE.L("vlive", fmt.Sprintf("Posting Celeb: #%s", celeb.ID))
                    entry.PostedCelebs = append(entry.PostedCelebs, celeb)
                    changes = true
                    go r.postCelebToChannel(entry, celeb, updatedVliveChannel)
                }
            }
            if changes == true {
                r.setEntry(entry)
            }
            safeEntries.mux.Unlock()
        }

        time.Sleep(0 * time.Second)
    }
}

func (r *VLive) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    args := strings.Fields(content)
    if len(args) >= 1 {
        switch args[0] {
        case "add": // [p]vlive add <vlive channel name/vlive channel id> <discord channel> [<Name or ID of the role to mention>]
            helpers.RequireMod(msg, func() {
                session.ChannelTyping(msg.ChannelID)
                // get target channel
                var err error
                var targetChannel *discordgo.Channel
                var targetGuild *discordgo.Guild
                if len(args) >= 3 {
                    targetChannel, err = helpers.GetChannelFromMention(args[2])
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

                mentionRole := new(discordgo.Role)
                if len(args) >= 4 {
                    mentionRoleName := args[3]
                    serverRoles, err := session.GuildRoles(targetGuild.ID)
                    helpers.Relax(err)
                    for _, serverRole := range serverRoles {
                        if serverRole.Mentionable == true && (serverRole.Name == mentionRoleName || serverRole.ID == mentionRoleName) {
                            mentionRole = serverRole
                        }
                    }
                    if mentionRole.ID == "" {
                        session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
                        return
                    }
                }
                // try to find channel by search
                vliveChannelId := ""
                if len(args[1]) >= 2 {
                    vliveChannelId, err = r.getVliveChannelIdFromChannelName(args[1])
                }
                if err != nil || vliveChannelId == "" {
                    vliveChannelId = args[1]
                }
                // use input as id instead or use the id from above (if channel found)
                vliveChannel, err := r.getVLiveChannelByVliveChannelId(vliveChannelId)
                if err != nil {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.vlive.channel-not-found"))
                    return
                }
                // create new entry in db
                entry := r.getEntryByOrCreateEmpty("id", "")
                entry.ServerID = targetChannel.GuildID
                entry.ChannelID = targetChannel.ID
                entry.VLiveChannel = vliveChannel
                entry.PostedVOD = vliveChannel.VOD
                entry.PostedUpcoming = vliveChannel.Upcoming
                entry.PostedLive = vliveChannel.Live
                entry.PostedCelebs = vliveChannel.Celebs
                entry.PostedNotices = vliveChannel.Notices
                entry.MentionRoleID = mentionRole.ID
                r.setEntry(entry)

                successMessage := helpers.GetTextF("plugins.vlive.channel-added-success", entry.VLiveChannel.Name, entry.ChannelID)
                if mentionRole.ID != "" {
                    successMessage += helpers.GetTextF("plugins.vlive.channel-added-success-additional-role", mentionRole.Name)
                }
                session.ChannelMessageSend(msg.ChannelID, successMessage)
                logger.INFO.L("vlive", fmt.Sprintf("Added V Live Channel %s (%s) to Channel %s (#%s) on Guild %s (#%s) Mention @%s (#%s)",
                    entry.VLiveChannel.Name, entry.VLiveChannel.Code, targetChannel.Name, entry.ChannelID, targetGuild.Name, targetGuild.ID,
                    mentionRole.Name, mentionRole.ID))
            })
        case "delete", "del": // [p]vlive delete <id>
            helpers.RequireMod(msg, func() {
                if len(args) >= 2 {
                    session.ChannelTyping(msg.ChannelID)
                    entryId := args[1]
                    entryBucket := r.getEntryBy("id", entryId)
                    if entryBucket.ID != "" {
                        r.deleteEntryById(entryBucket.ID)

                        session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.vlive.channel-delete-success", entryBucket.VLiveChannel.Name))
                        logger.INFO.L("vlive", fmt.Sprintf("Deleted V Live Channel %s (%s)", entryBucket.VLiveChannel.Name, entryBucket.VLiveChannel.Code))
                    } else {
                        session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.vlive.channel-delete-not-found-error"))
                        return
                    }

                } else {
                    session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
                    return
                }
            })
        case "list": // [p]vlive list
            currentChannel, err := session.Channel(msg.ChannelID)
            helpers.Relax(err)
            var entryBucket []DB_VLive_Entry
            listCursor, err := rethink.Table("vlive").Filter(
                rethink.Row.Field("serverid").Eq(currentChannel.GuildID),
            ).Run(helpers.GetDB())
            helpers.Relax(err)
            defer listCursor.Close()
            err = listCursor.All(&entryBucket)

            if err == rethink.ErrEmptyResult || len(entryBucket) <= 0 {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.vlive.channel-list-no-channels-error"))
                return
            } else if err != nil {
                helpers.Relax(err)
            }

            resultMessage := ""
            for _, entry := range entryBucket {
                mentionText := ""
                if entry.MentionRoleID != "" {
                    role, err := session.State.Role(currentChannel.GuildID, entry.MentionRoleID)
                    helpers.Relax(err)
                    mentionText += fmt.Sprintf(" mentioning `@%s`", role.Name)
                }
                resultMessage += fmt.Sprintf("`%s`: V Live Channel `%s` posting to <#%s>%s\n", entry.ID, entry.VLiveChannel.Name, entry.ChannelID, mentionText)
            }
            resultMessage += fmt.Sprintf("Found **%d** V Live Channels in total.", len(entryBucket))
            for _, resultPage := range helpers.Pagify(resultMessage, "\n") {
                _, err := session.ChannelMessageSend(msg.ChannelID, resultPage)
                helpers.Relax(err)
            }
        default:
            session.ChannelTyping(msg.ChannelID)
            // try to find channel by search
            var err error
            vliveChannelId := ""
            if len(content) >= 2 {
                vliveChannelId, err = r.getVliveChannelIdFromChannelName(content)
            }
            if err != nil || vliveChannelId == "" {
                vliveChannelId = args[0]
            }
            // use input as id instead or use the id from above (if channel found)
            vliveChannel, err := r.getVLiveChannelByVliveChannelId(vliveChannelId)
            if err != nil || vliveChannel.Name == "" {
                session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.vlive.channel-not-found"))
                return
            }
            channelEmbed := &discordgo.MessageEmbed{
                Title:     helpers.GetTextF("plugins.vlive.channel-embed-title", vliveChannel.Name),
                URL:       vliveChannel.Url,
                Thumbnail: &discordgo.MessageEmbedThumbnail{URL: vliveChannel.ProfileImgUrl},
                Footer:    &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.vlive.embed-footer")},
                Fields: []*discordgo.MessageEmbedField{
                    {Name: "Followers", Value: humanize.Comma(vliveChannel.Followers), Inline: true},
                    {Name: "Videos", Value: humanize.Comma(vliveChannel.TotalVideos), Inline: true}},
                Color: helpers.GetDiscordColorFromHex(vliveChannel.Color),
            }
            if len(vliveChannel.Live) > 0 {
                channelEmbed.Fields = append(channelEmbed.Fields, &discordgo.MessageEmbedField{
                    Name:   helpers.GetTextF("plugins.vlive.channel-embed-name-live", vliveChannel.Live[0].Date),
                    Value:  fmt.Sprintf("**%s**\n%s", vliveChannel.Live[0].Title, vliveChannel.Live[0].Url),
                    Inline: false,
                })
                channelEmbed.Image = &discordgo.MessageEmbedImage{URL: vliveChannel.Live[0].Thumbnail}
            } else if len(vliveChannel.VOD) > 0 {
                channelEmbed.Fields = append(channelEmbed.Fields, &discordgo.MessageEmbedField{
                    Name:   helpers.GetTextF("plugins.vlive.channel-embed-name-vod", vliveChannel.VOD[0].Date),
                    Value:  fmt.Sprintf("**%s**\n**Plays:** %d\n**Likes:** %d\n%s", vliveChannel.VOD[0].Title, vliveChannel.VOD[0].Plays, vliveChannel.VOD[0].Likes, vliveChannel.VOD[0].Url),
                    Inline: false,
                })
                channelEmbed.Image = &discordgo.MessageEmbedImage{URL: vliveChannel.VOD[0].Thumbnail}
            }
            if len(vliveChannel.Upcoming) > 0 {
                channelEmbed.Fields = append(channelEmbed.Fields, &discordgo.MessageEmbedField{
                    Name:   helpers.GetTextF("plugins.vlive.channel-embed-name-upcoming", vliveChannel.Upcoming[0].Date),
                    Value:  fmt.Sprintf("**%s**\n%s", vliveChannel.Upcoming[0].Title, vliveChannel.Upcoming[0].Url),
                    Inline: false,
                })
            }
            _, err = session.ChannelMessageSendEmbedWithMessage(msg.ChannelID, fmt.Sprintf("<%s>", vliveChannel.Url), channelEmbed)
            if err != nil {
                helpers.Relax(err)
            }
            return
        }
    } else {
        session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
    }
}

func (r *VLive) getVliveChannelIdFromChannelName(channelSearchName string) (string, error) {
    friendlySearch := fmt.Sprintf(VliveFriendlySearch, channelSearchName)
    doc, err := goquery.NewDocument(friendlySearch)
    if err != nil {
        return "", err
    }
    finalId := ""
    doc.Find(".ct_box").Each(func(i int, s *goquery.Selection) {
        //name := s.Find(".name").Text()
        url, _ := s.Attr("href")
        re := regexp.MustCompile(ChannelIdRegex)
        result := re.FindStringSubmatch(url)
        if !strings.HasSuffix(result[5], " +") {
            finalId = result[5]
            return
        }
    })

    if finalId == "" {
        return "", errors.New("Channel not found!")
    } else {
        return finalId, nil
    }

}

func (r *VLive) getVLiveChannelByVliveChannelId(channelId string) (DB_VLive_Channel, error) {
    var vliveChannel DB_VLive_Channel

    defer func() {
        err := recover()

        if err != nil {
            logger.ERROR.L("vlive", fmt.Sprintf("updating vlive channel %s failed: %s", channelId, err))
        }
    }()

    endpointDecodeChannelCode := fmt.Sprintf(VliveEndpointDecodeChannelCode, VliveAppId, channelId)
    jsonGabs := helpers.GetJSON(endpointDecodeChannelCode)
    resN, ok := jsonGabs.Path("result.channelSeq").Data().(float64)
    if ok == false {
        return vliveChannel, errors.New("Unable to get channel seq")
    }
    vliveChannelSeq := int(resN)

    endpointChannel := fmt.Sprintf(VliveEndpointChannel, vliveChannelSeq, VliveAppId)
    resB := helpers.NetGet(endpointChannel)

    json.Unmarshal(resB, &vliveChannel)
    vliveChannel.Url = fmt.Sprintf(VliveFriendlyChannel, channelId)

    // Get VODs and LIVEs
    var vliveVideo DB_VLive_Video
    endpointChannelVideoList := fmt.Sprintf(VliveEndpointChannelVideoList, VliveAppId, vliveChannelSeq, 10)
    jsonGabs = helpers.GetJSON(endpointChannelVideoList)

    resN, ok = jsonGabs.Path("result.totalVideoCount").Data().(float64)
    if ok == true {
        vliveChannel.TotalVideos = int64(resN)
    }

    videoListChildren, err := jsonGabs.Path("result.videoList").Children()
    if err == nil {
        for _, videoListEntry := range videoListChildren {
            err = json.Unmarshal([]byte(videoListEntry.String()), &vliveVideo)
            if err != nil {
                return vliveChannel, err
            }
            vliveVideo.Url = fmt.Sprintf(VliveFriendlyVideo, vliveVideo.Seq)
            if vliveVideo.Type == "LIVE" {
                vliveChannel.Live = append(vliveChannel.VOD, vliveVideo)
            } else {
                vliveChannel.VOD = append(vliveChannel.VOD, vliveVideo)
            }
        }
    }
    // Get Upcomings
    endpointUpcomingVideoList := fmt.Sprintf(VliveEndpointUpcomingVideoList, VliveAppId, vliveChannelSeq, 10)
    jsonGabs = helpers.GetJSON(endpointUpcomingVideoList)
    videoListChildren, err = jsonGabs.Path("result.videoList").Children()
    if err == nil {
        for _, videoListEntry := range videoListChildren {
            err = json.Unmarshal([]byte(videoListEntry.String()), &vliveVideo)
            if err != nil {
                return vliveChannel, err
            }
            vliveChannel.Upcoming = append(vliveChannel.Upcoming, vliveVideo)
        }

    }
    // Get Notices
    var vliveNotice DB_VLive_Notice
    endpointNotices := fmt.Sprintf(VliveEndpointNotices, vliveChannelSeq)
    jsonGabs = helpers.GetJSON(endpointNotices)
    noticesChildren, err := jsonGabs.Path("data").Children()
    if err == nil {
        for _, noticeEntry := range noticesChildren {
            err = json.Unmarshal([]byte(noticeEntry.String()), &vliveNotice)
            if err != nil {
                return vliveChannel, err
            }
            vliveNotice.Url = fmt.Sprintf(VliveFriendlyNotice, channelId, vliveNotice.Number)
            vliveChannel.Notices = append(vliveChannel.Notices, vliveNotice)
        }
    }
    // Get Celeb
    if vliveChannel.CelebBoard.BoardID != 0 {
        var vliveCeleb DB_VLive_Celeb
        endpointCeleb := fmt.Sprintf(VliveEndpointCeleb, vliveChannel.CelebBoard.BoardID, VliveAppId)
        jsonGabs = helpers.GetJSON(endpointCeleb)
        celebsChildren, err := jsonGabs.Path("data").Children()
        if err == nil {
            for _, celebEntry := range celebsChildren {
                err = json.Unmarshal([]byte(celebEntry.String()), &vliveCeleb)
                if err != nil {
                    return vliveChannel, err
                }
                vliveCeleb.Url = fmt.Sprintf(VliveFriendlyCeleb, channelId, vliveCeleb.ID)
                vliveChannel.Celebs = append(vliveChannel.Celebs, vliveCeleb)
            }
        }
    }

    return vliveChannel, nil
}

func (r *VLive) postVodToChannel(entry DB_VLive_Entry, vod DB_VLive_Video, vliveChannel DB_VLive_Channel) {
    channelEmbed := &discordgo.MessageEmbed{
        Title:       helpers.GetTextF("plugins.vlive.channel-embed-title-vod", vliveChannel.Name),
        URL:         vod.Url,
        Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: vliveChannel.ProfileImgUrl},
        Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.vlive.embed-footer")},
        Description: fmt.Sprintf("**%s**", vod.Title),
        Image:       &discordgo.MessageEmbedImage{URL: vod.Thumbnail},
        Color:       helpers.GetDiscordColorFromHex(vliveChannel.Color),
    }
    mentionText := ""
    if entry.MentionRoleID != "" {
        mentionText = fmt.Sprintf("<@&%s>\n", entry.MentionRoleID)
    }
    _, err := cache.GetSession().ChannelMessageSendEmbedWithMessage(entry.ChannelID, mentionText+fmt.Sprintf("<%s>", vod.Url), channelEmbed)
    if err != nil {
        logger.ERROR.L("vlive", fmt.Sprintf("posting vod: #%d to channel: #%s failed: %s", vod.Seq, entry.ChannelID, err))
    }
}

func (r *VLive) postUpcomingToChannel(entry DB_VLive_Entry, vod DB_VLive_Video, vliveChannel DB_VLive_Channel) {
    channelEmbed := &discordgo.MessageEmbed{
        Title:       helpers.GetTextF("plugins.vlive.channel-embed-title-upcoming", vliveChannel.Name, vod.Date),
        URL:         vod.Url,
        Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: vliveChannel.ProfileImgUrl},
        Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.vlive.embed-footer")},
        Description: fmt.Sprintf("**%s**", vod.Title),
        Image:       &discordgo.MessageEmbedImage{URL: vod.Thumbnail},
        Color:       helpers.GetDiscordColorFromHex(vliveChannel.Color),
    }
    mentionText := ""
    if entry.MentionRoleID != "" {
        mentionText = fmt.Sprintf("<@&%s>\n", entry.MentionRoleID)
    }
    postText := ""
    if vod.Url != "" {
        postText = fmt.Sprintf("<%s>", vod.Url)
    }
    _, err := cache.GetSession().ChannelMessageSendEmbedWithMessage(entry.ChannelID, mentionText+postText, channelEmbed)
    if err != nil {
        logger.ERROR.L("vlive", fmt.Sprintf("posting upcoming: #%d to channel: #%s failed: %s", vod.Seq, entry.ChannelID, err))
    }
}

func (r *VLive) postLiveToChannel(entry DB_VLive_Entry, vod DB_VLive_Video, vliveChannel DB_VLive_Channel) {
    channelEmbed := &discordgo.MessageEmbed{
        Title:       helpers.GetTextF("plugins.vlive.channel-embed-title-live", vliveChannel.Name),
        URL:         vod.Url,
        Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: vliveChannel.ProfileImgUrl},
        Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.vlive.embed-footer")},
        Description: fmt.Sprintf("**%s**", vod.Title),
        Image:       &discordgo.MessageEmbedImage{URL: vod.Thumbnail},
        Color:       helpers.GetDiscordColorFromHex(vliveChannel.Color),
    }
    mentionText := ""
    if entry.MentionRoleID != "" {
        mentionText = fmt.Sprintf("<@&%s>\n", entry.MentionRoleID)
    }
    _, err := cache.GetSession().ChannelMessageSendEmbedWithMessage(entry.ChannelID, mentionText+fmt.Sprintf("<%s>", vod.Url), channelEmbed)
    if err != nil {
        logger.ERROR.L("vlive", fmt.Sprintf("posting live: #%d to channel: #%s failed: %s", vod.Seq, entry.ChannelID, err))
    }
}

func (r *VLive) postNoticeToChannel(entry DB_VLive_Entry, notice DB_VLive_Notice, vliveChannel DB_VLive_Channel) {
    channelEmbed := &discordgo.MessageEmbed{
        Title:       helpers.GetTextF("plugins.vlive.channel-embed-title-notice", vliveChannel.Name),
        URL:         notice.Url,
        Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: vliveChannel.ProfileImgUrl},
        Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.vlive.embed-footer")},
        Description: fmt.Sprintf("**%s**", notice.Title),
        Image:       &discordgo.MessageEmbedImage{URL: notice.ImageUrl},
        Color:       helpers.GetDiscordColorFromHex(vliveChannel.Color),
    }
    mentionText := ""
    if entry.MentionRoleID != "" {
        mentionText = fmt.Sprintf("<@&%s>\n", entry.MentionRoleID)
    }
    _, err := cache.GetSession().ChannelMessageSendEmbedWithMessage(entry.ChannelID, mentionText+fmt.Sprintf("<%s>", notice.Url), channelEmbed)
    if err != nil {
        logger.ERROR.L("vlive", fmt.Sprintf("posting notice: #%d to channel: #%s failed: %s", notice.Number, entry.ChannelID, err))
    }
}

func (r *VLive) postCelebToChannel(entry DB_VLive_Entry, celeb DB_VLive_Celeb, vliveChannel DB_VLive_Channel) {
    channelEmbed := &discordgo.MessageEmbed{
        Title:       helpers.GetTextF("plugins.vlive.channel-embed-title-celeb", vliveChannel.Name),
        URL:         celeb.Url,
        Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: vliveChannel.ProfileImgUrl},
        Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.vlive.embed-footer")},
        Description: fmt.Sprintf("%s ...", celeb.Summary),
        Color:       helpers.GetDiscordColorFromHex(vliveChannel.Color),
    }
    mentionText := ""
    if entry.MentionRoleID != "" {
        mentionText = fmt.Sprintf("<@&%s>\n", entry.MentionRoleID)
    }
    _, err := cache.GetSession().ChannelMessageSendEmbedWithMessage(entry.ChannelID, mentionText+fmt.Sprintf("<%s>", celeb.Url), channelEmbed)
    if err != nil {
        logger.ERROR.L("vlive", fmt.Sprintf("posting celeb: #%s to channel: #%s failed: %s", celeb.ID, entry.ChannelID, err))
    }
}

func (r *VLive) getEntryBy(key string, id string) DB_VLive_Entry {
    var entryBucket DB_VLive_Entry
    listCursor, err := rethink.Table("vlive").Filter(
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

func (r *VLive) getEntryByOrCreateEmpty(key string, id string) DB_VLive_Entry {
    var entryBucket DB_VLive_Entry
    listCursor, err := rethink.Table("vlive").Filter(
        rethink.Row.Field(key).Eq(id),
    ).Run(helpers.GetDB())
    defer listCursor.Close()
    err = listCursor.One(&entryBucket)

    // If user has no DB entries create an empty document
    if err == rethink.ErrEmptyResult {
        insert := rethink.Table("vlive").Insert(DB_VLive_Entry{})
        res, e := insert.RunWrite(helpers.GetDB())
        // If the creation was successful read the document
        if e != nil {
            panic(e)
        } else {
            return r.getEntryByOrCreateEmpty("id", res.GeneratedKeys[0])
        }
    } else if err != nil {
        panic(err)
    }

    return entryBucket
}

func (r *VLive) setEntry(entry DB_VLive_Entry) {
    _, err := rethink.Table("vlive").Update(entry).Run(helpers.GetDB())
    helpers.Relax(err)
}

func (r *VLive) deleteEntryById(id string) {
    _, err := rethink.Table("vlive").Filter(
        rethink.Row.Field("id").Eq(id),
    ).Delete().RunWrite(helpers.GetDB())
    helpers.Relax(err)
}
