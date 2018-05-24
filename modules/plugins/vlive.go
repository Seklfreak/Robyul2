package plugins

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/globalsign/mgo/bson"
	redisCache "github.com/go-redis/cache"
)

var (
	VliveAppId = "8c6cc7b45d2568fb668be6e05b6e5a3b"
)

const (
	VliveEndpointDecodeChannelCode = "http://api.vfan.vlive.tv/vproxy/channelplus/decodeChannelCode?app_id=%s&channelCode=%s"
	VliveEndpointChannel           = "http://api.vfan.vlive.tv/channel.%d?app_id=%s&fields=channel_seq,channel_code,type,channel_name,fan_count,channel_cover_img,channel_profile_img,representative_color,celeb_board"
	VliveEndpointChannelVideoList  = "http://api.vfan.vlive.tv/vproxy/channelplus/getChannelVideoList?app_id=%s&channelSeq=%d&maxNumOfRows=%d"
	VliveEndpointUpcomingVideoList = "http://api.vfan.vlive.tv/vproxy/channelplus/getUpcomingVideoList?app_id=%s&channelSeq=%d&maxNumOfRows=%d"
	VliveEndpointNotices           = "http://notice.vlive.tv/notice/list.json?channel_seq=%d"
	VliveEndpointCeleb             = "http://api.vfan.vlive.tv/board.%d/posts?app_id=%s"
	VliveFriendlyChannel           = "http://channels.vlive.tv/%s"
	VliveFriendlyVideo             = "http://www.vlive.tv/video/%d"
	VliveFriendlyNotice            = "http://channels.vlive.tv/%s/notice/%d"
	VliveFriendlyCeleb             = "http://channels.vlive.tv/%s/celeb/%s"
	VliveFriendlySearch            = "http://www.vlive.tv/search/all?query=%s"
	ChannelIdRegex                 = "(http(s)?://channels.vlive.tv)?(/)?(channels/)?([A-Z0-9]+)(/video)?"
	VLiveWorkers                   = 15
)

type VLive struct{}

func (r *VLive) Commands() []string {
	return []string{
		"vlive",
	}
}

func (r *VLive) Init(session *discordgo.Session) {
	go r.checkVliveFeedsLoop()
	cache.GetLogger().WithField("module", "vlive").Info("Started vlive loop (0s)")
}
func (r *VLive) checkVliveFeedsLoop() {
	var entries []models.VliveEntry
	var bundledEntries map[string][]models.VliveEntry

	defer helpers.Recover()
	defer func() {
		go func() {
			cache.GetLogger().WithField("module", "vlive").Error("The checkVliveFeedsLoop died. Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			r.checkVliveFeedsLoop()
		}()
	}()

	for {
		bundledEntries = make(map[string][]models.VliveEntry, 0)

		err := helpers.MDbIterWithoutLogging(helpers.MdbCollection(models.VliveTable).Find(nil)).All(&entries)
		helpers.Relax(err)

		for _, entry := range entries {
			// check if channel exists
			channel, err := helpers.GetChannelWithoutApi(entry.ChannelID)
			if err != nil || channel == nil || channel.ID == "" {
				continue
			}

			// check if we can send messages
			channelPermission, err := cache.GetSession().State.UserChannelPermissions(cache.GetSession().State.User.ID, channel.ID)
			if err != nil {
				continue
			}

			if channelPermission&discordgo.PermissionSendMessages != discordgo.PermissionSendMessages ||
				channelPermission&discordgo.PermissionEmbedLinks != discordgo.PermissionEmbedLinks {
				continue
			}

			if _, ok := bundledEntries[entry.VLiveChannel.Code]; ok {
				bundledEntries[entry.VLiveChannel.Code] = append(bundledEntries[entry.VLiveChannel.Code], entry)
			} else {
				bundledEntries[entry.VLiveChannel.Code] = []models.VliveEntry{entry}
			}
		}

		cache.GetLogger().WithField("module", "vlive").Info(fmt.Sprintf("checking %d channels for %d feeds with %d workers", len(bundledEntries), len(entries), VLiveWorkers))
		start := time.Now()

		jobs := make(chan map[string][]models.VliveEntry, 0)
		results := make(chan int, 0)

		workerEntries := make(map[int]map[string][]models.VliveEntry, 0)
		for w := 1; w <= VLiveWorkers; w++ {
			go r.feedWorker(w, jobs, results)
			workerEntries[w] = make(map[string][]models.VliveEntry)
		}

		lastWorker := 1
		for code, codeEntries := range bundledEntries {
			workerEntries[lastWorker][code] = codeEntries
			lastWorker++
			if lastWorker > VLiveWorkers {
				lastWorker = 1
			}
		}

		for _, workerEntry := range workerEntries {
			jobs <- workerEntry
		}
		close(jobs)

		for a := 1; a <= VLiveWorkers; a++ {
			<-results
		}
		elapsed := time.Since(start)
		cache.GetLogger().WithField("module", "vlive").Info(fmt.Sprintf("checked %d channels for %d feeds with %d workers, took %s", len(bundledEntries), len(entries), VLiveWorkers, elapsed))
		metrics.VliveRefreshTime.Set(elapsed.Seconds())

		if len(entries) <= 10 {
			time.Sleep(60 * time.Second)
		}
	}
}

func (r *VLive) feedWorker(id int, jobs <-chan map[string][]models.VliveEntry, results chan<- int) {
	defer helpers.Recover()

	for job := range jobs {
		//cache.GetLogger().WithField("module", "vlive").WithField("worker", id).Infof("worker %d started for %d channels", id, len(job))
		for channelCode, entries := range job {
			//cache.GetLogger().WithField("module", "vlive").WithField("worker", id).Info(fmt.Sprintf("checking V Live Channel %s for %d channels", entries[0].VLiveChannel.Name, len(entries)))
			updatedVliveChannel, err := r.getVLiveChannelByVliveChannelId(channelCode)
			if err != nil {
				cache.GetLogger().WithField("module", "vlive").WithField("worker", id).Warnf("updating vlive channel %s failed: %s", channelCode, err.Error())
				continue
			}
			for _, entry := range entries {
				changes := false

				for _, vod := range updatedVliveChannel.VOD {
					// don't post playlists
					if vod.Type == "PLAYLIST" {
						continue
					}
					videoAlreadyPosted := false
					for _, postedVod := range entry.PostedVOD {
						if postedVod.Seq == vod.Seq {
							videoAlreadyPosted = true
						}
					}
					if videoAlreadyPosted == false {
						cache.GetLogger().WithField("module", "vlive").WithField("worker", id).Info(fmt.Sprintf("Posting VOD: #%d", vod.Seq))
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
						cache.GetLogger().WithField("module", "vlive").WithField("worker", id).Info(fmt.Sprintf("Posting Upcoming: #%d", upcoming.Seq))
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
						cache.GetLogger().WithField("module", "vlive").WithField("worker", id).Info(fmt.Sprintf("Posting Live: #%d", live.Seq))
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
						cache.GetLogger().WithField("module", "vlive").WithField("worker", id).Info(fmt.Sprintf("Posting Notice: #%d", notice.Number))
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
						cache.GetLogger().WithField("module", "vlive").WithField("worker", id).Info(fmt.Sprintf("Posting Celeb: #%s", celeb.ID))
						entry.PostedCelebs = append(entry.PostedCelebs, celeb)
						changes = true
						go r.postCelebToChannel(entry, celeb, updatedVliveChannel)
					}
				}
				if changes == true {
					err = helpers.MDbUpdateWithoutLogging(models.VliveTable, entry.ID, entry)
					helpers.Relax(err)
				}
			}
		}
	}
	results <- len(jobs)
}

func (r *VLive) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermVLive) {
		return
	}

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
					targetChannel, err = helpers.GetChannelFromMention(msg, args[2])
					if err != nil {
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
						return
					}
				} else {
					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
					return
				}
				targetGuild, err = helpers.GetGuild(targetChannel.GuildID)
				helpers.Relax(err)

				mentionRole := new(discordgo.Role)
				if len(args) >= 4 {
					mentionRoleName := args[3]
					serverRoles, err := session.GuildRoles(targetGuild.ID)
					if err != nil {
						if errD, ok := err.(*discordgo.RESTError); ok {
							if errD.Message.Code == 50013 {
								_, err = helpers.SendMessage(msg.ChannelID, "Please give me the `Manage Roles` permission.")
								helpers.Relax(err)
								return
							} else {
								helpers.Relax(err)
							}
						} else {
							helpers.Relax(err)
						}
					}
					for _, serverRole := range serverRoles {
						if serverRole.Mentionable == true && (serverRole.Name == mentionRoleName || serverRole.ID == mentionRoleName) {
							mentionRole = serverRole
						}
					}
					if mentionRole.ID == "" {
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
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
					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.vlive.channel-not-found"))
					return
				}
				// create new entry in db
				newID, err := helpers.MDbInsert(models.VliveTable, models.VliveEntry{
					GuildID:        targetChannel.GuildID,
					ChannelID:      targetChannel.ID,
					VLiveChannel:   vliveChannel,
					PostedVOD:      vliveChannel.VOD,
					PostedUpcoming: vliveChannel.Upcoming,
					PostedLive:     vliveChannel.Live,
					PostedNotices:  vliveChannel.Notices,
					PostedCelebs:   vliveChannel.Celebs,
					MentionRoleID:  mentionRole.ID,
				})
				helpers.Relax(err)

				_, err = helpers.EventlogLog(time.Now(), targetChannel.GuildID, helpers.MdbIdToHuman(newID),
					models.EventlogTargetTypeRobyulVliveFeed, msg.Author.ID,
					models.EventlogTypeRobyulVliveFeedAdd, "",
					nil,
					[]models.ElasticEventlogOption{
						{
							Key:   "vlive_feed_channelid",
							Value: targetChannel.ID,
							Type:  models.EventlogTargetTypeChannel,
						},
						{
							Key:   "vlive_feed_vlivechannel_name",
							Value: vliveChannel.Name,
						},
						{
							Key:   "vlive_feed_vlivechannel_code",
							Value: vliveChannel.Code,
						},
						{
							Key:   "vlive_feed_mentionroleid",
							Value: mentionRole.ID,
							Type:  models.EventlogTargetTypeRole,
						},
					}, false)
				helpers.RelaxLog(err)

				successMessage := helpers.GetTextF("plugins.vlive.channel-added-success", vliveChannel.Name, targetChannel.ID)
				if mentionRole.ID != "" {
					successMessage += helpers.GetTextF("plugins.vlive.channel-added-success-additional-role", mentionRole.Name)
				}
				helpers.SendMessage(msg.ChannelID, successMessage)
				cache.GetLogger().WithField("module", "vlive").Info(fmt.Sprintf("Added V Live Channel %s (%s) to Channel %s (#%s) on Guild %s (#%s) Mention @%s (#%s)",
					vliveChannel.Name, vliveChannel.Code, targetChannel.Name, targetChannel.ID, targetGuild.Name, targetGuild.ID,
					mentionRole.Name, mentionRole.ID))
			})
		case "delete", "del", "remove": // [p]vlive delete <id>
			helpers.RequireMod(msg, func() {
				if len(args) >= 2 {
					session.ChannelTyping(msg.ChannelID)

					channel, err := helpers.GetChannel(msg.ChannelID)
					helpers.Relax(err)

					entryId := args[1]

					var entryBucket models.VliveEntry
					err = helpers.MdbOne(
						helpers.MdbCollection(models.VliveTable).Find(bson.M{"guildid": channel.GuildID, "_id": helpers.HumanToMdbId(entryId)}),
						&entryBucket,
					)
					if helpers.IsMdbNotFound(err) {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.vlive.channel-delete-not-found-error"))
						return
					}
					helpers.Relax(err)

					err = helpers.MDbDelete(models.VliveTable, entryBucket.ID)
					helpers.Relax(err)

					_, err = helpers.EventlogLog(time.Now(), channel.GuildID, entryId,
						models.EventlogTargetTypeRobyulVliveFeed, msg.Author.ID,
						models.EventlogTypeRobyulVliveFeedRemove, "",
						nil,
						[]models.ElasticEventlogOption{
							{
								Key:   "vlive_feed_channelid",
								Value: helpers.MdbIdToHuman(entryBucket.ID),
								Type:  models.EventlogTargetTypeChannel,
							},
							{
								Key:   "vlive_feed_vlivechannel_name",
								Value: entryBucket.VLiveChannel.Name,
							},
							{
								Key:   "vlive_feed_vlivechannel_code",
								Value: entryBucket.VLiveChannel.Code,
							},
							{
								Key:   "vlive_feed_mentionroleid",
								Value: entryBucket.MentionRoleID,
								Type:  models.EventlogTargetTypeRole,
							},
						}, false)
					helpers.RelaxLog(err)

					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.vlive.channel-delete-success", entryBucket.VLiveChannel.Name))
					cache.GetLogger().WithField("module", "vlive").Info(fmt.Sprintf("Deleted V Live Channel %s (%s)", entryBucket.VLiveChannel.Name, entryBucket.VLiveChannel.Code))
				} else {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					return
				}
			})
		case "list": // [p]vlive list
			currentChannel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)
			var entryBucket []models.VliveEntry
			err = helpers.MDbIter(helpers.MdbCollection(models.VliveTable).Find(bson.M{"guildid": currentChannel.GuildID})).All(&entryBucket)
			helpers.Relax(err)

			if entryBucket == nil || len(entryBucket) <= 0 {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.vlive.channel-list-no-channels-error"))
				return
			}

			resultMessage := ""
			for _, entry := range entryBucket {
				mentionText := ""
				if entry.MentionRoleID != "" {
					role, err := session.State.Role(currentChannel.GuildID, entry.MentionRoleID)
					helpers.Relax(err)
					mentionText += fmt.Sprintf(" mentioning `@%s`", role.Name)
				}
				resultMessage += fmt.Sprintf("`%s`: V Live Channel `%s` posting to <#%s>%s\n", helpers.MdbIdToHuman(entry.ID), entry.VLiveChannel.Name, entry.ChannelID, mentionText)
			}
			resultMessage += fmt.Sprintf("Found **%d** V Live Channels in total.", len(entryBucket))
			for _, resultPage := range helpers.Pagify(resultMessage, "\n") {
				_, err := helpers.SendMessage(msg.ChannelID, resultPage)
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
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.vlive.channel-not-found"))
				return
			}
			channelEmbed := &discordgo.MessageEmbed{
				Title:     helpers.GetTextF("plugins.vlive.channel-embed-title", vliveChannel.Name),
				URL:       vliveChannel.Url,
				Thumbnail: &discordgo.MessageEmbedThumbnail{URL: vliveChannel.ProfileImgUrl},
				Footer: &discordgo.MessageEmbedFooter{
					Text:    helpers.GetText("plugins.vlive.embed-footer"),
					IconURL: helpers.GetText("plugins.vlive.embed-footer-imageurl"),
				},
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
					Value:  fmt.Sprintf("**%s**\n**Plays:** %s\n**Likes:** %s\n%s", vliveChannel.VOD[0].Title, humanize.Comma(vliveChannel.VOD[0].Plays), humanize.Comma(vliveChannel.VOD[0].Likes), vliveChannel.VOD[0].Url),
					Inline: false,
				})
				channelEmbed.Image = &discordgo.MessageEmbedImage{URL: vliveChannel.VOD[0].Thumbnail}
			}
			if len(vliveChannel.Upcoming) > 0 {
				channelEmbed.Fields = append(channelEmbed.Fields, &discordgo.MessageEmbedField{
					Name:   helpers.GetTextF("plugins.vlive.channel-embed-name-upcoming", vliveChannel.Upcoming[0].Date),
					Value:  fmt.Sprintf("**%s**", vliveChannel.Upcoming[0].Title),
					Inline: false,
				})
			}
			_, err = helpers.SendComplex(msg.ChannelID,
				&discordgo.MessageSend{
					Content: fmt.Sprintf("<%s>", vliveChannel.Url),
					Embed:   channelEmbed,
				})
			helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
			return
		}
	} else {
		helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
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

func (r *VLive) getChannelSeqFromChannelID(channelID string) (channelSeq int, err error) {
	cacheCodec := cache.GetRedisCacheCodec()
	key := "robyul2-discord:vlive:channelseq-by-channelid:v2:" + channelID

	if err = cacheCodec.Get(key, &channelSeq); err == nil {
		return channelSeq, nil
	}

	endpointDecodeChannelCode := fmt.Sprintf(VliveEndpointDecodeChannelCode, VliveAppId, channelID)
	jsonGabs := helpers.GetJSON(endpointDecodeChannelCode)

	metrics.VLiveRequests.Add(1)

	resN, ok := jsonGabs.Path("result.channelSeq").Data().(float64)
	if ok == false {
		return -1, errors.New("unable to get channel sequence")
	}

	err = cacheCodec.Set(&redisCache.Item{
		Key:        key,
		Object:     int(resN),
		Expiration: time.Hour * 24,
	})
	helpers.RelaxLog(err)

	return int(resN), nil
}

func (r *VLive) getChannelFromChannelID(channelID string) (channel models.VliveChannelInfo, err error) {
	cacheCodec := cache.GetRedisCacheCodec()
	key := "robyul2-discord:vlive:channel-by-channelid:v2:" + channelID

	if err = cacheCodec.Get(key, &channel); err == nil {
		return channel, nil
	}

	var vliveChannel models.VliveChannelInfo

	channelSeq, err := r.getChannelSeqFromChannelID(channelID)
	if err != nil {
		return vliveChannel, err
	}

	endpointChannel := fmt.Sprintf(VliveEndpointChannel, channelSeq, VliveAppId)
	resB := helpers.NetGet(endpointChannel)

	metrics.VLiveRequests.Add(1)

	err = json.Unmarshal(resB, &vliveChannel)
	if err != nil {
		return vliveChannel, err
	}

	vliveChannel.Url = fmt.Sprintf(VliveFriendlyChannel, channelID)

	err = cacheCodec.Set(&redisCache.Item{
		Key:        key,
		Object:     vliveChannel,
		Expiration: time.Minute * 30,
	})
	helpers.RelaxLog(err)

	return vliveChannel, nil
}

func (r *VLive) getVLiveChannelByVliveChannelId(channelId string) (models.VliveChannelInfo, error) {
	var vliveChannel models.VliveChannelInfo

	if channelId == "" {
		return vliveChannel, errors.New("invalid channel ID")
	}

	defer func() {
		err := recover()

		if err != nil {
			cache.GetLogger().WithField("module", "vlive").Warnf("updating vlive channel %s failed: %s", channelId, err)
		}
	}()

	vliveChannel, err := r.getChannelFromChannelID(channelId)
	if err != nil {
		return vliveChannel, err
	}

	// Get VODs and LIVEs
	var vliveVideo models.VliveVideoInfo
	endpointChannelVideoList := fmt.Sprintf(VliveEndpointChannelVideoList, VliveAppId, vliveChannel.Seq, 10)
	jsonGabs := helpers.GetJSON(endpointChannelVideoList)
	metrics.VLiveRequests.Add(1)

	resN, ok := jsonGabs.Path("result.totalVideoCount").Data().(float64)
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
	endpointUpcomingVideoList := fmt.Sprintf(VliveEndpointUpcomingVideoList, VliveAppId, vliveChannel.Seq, 10)
	jsonGabs = helpers.GetJSON(endpointUpcomingVideoList)
	metrics.VLiveRequests.Add(1)
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
	var vliveNotice models.VliveNoticeInfo
	endpointNotices := fmt.Sprintf(VliveEndpointNotices, vliveChannel.Seq)
	jsonGabs = helpers.GetJSON(endpointNotices)
	metrics.VLiveRequests.Add(1)
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
		var vliveCeleb models.VliveCelebInfo
		endpointCeleb := fmt.Sprintf(VliveEndpointCeleb, vliveChannel.CelebBoard.BoardID, VliveAppId)
		jsonGabs = helpers.GetJSON(endpointCeleb)
		metrics.VLiveRequests.Add(1)
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

func (r *VLive) postVodToChannel(entry models.VliveEntry, vod models.VliveVideoInfo, vliveChannel models.VliveChannelInfo) {
	channelEmbed := &discordgo.MessageEmbed{
		Title:     helpers.GetTextF("plugins.vlive.channel-embed-title-vod", vliveChannel.Name),
		URL:       vod.Url,
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: vliveChannel.ProfileImgUrl},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.vlive.embed-footer"),
			IconURL: helpers.GetText("plugins.vlive.embed-footer-imageurl"),
		},
		Description: fmt.Sprintf("**%s**", vod.Title),
		Image:       &discordgo.MessageEmbedImage{URL: vod.Thumbnail},
		Color:       helpers.GetDiscordColorFromHex(vliveChannel.Color),
	}
	mentionText := ""
	if entry.MentionRoleID != "" {
		mentionText = fmt.Sprintf("<@&%s>\n", entry.MentionRoleID)
	}
	_, err := helpers.SendComplex(entry.ChannelID, &discordgo.MessageSend{
		Content: mentionText + fmt.Sprintf("<%s>", vod.Url),
		Embed:   channelEmbed,
	})
	if err != nil {
		cache.GetLogger().WithField("module", "vlive").Warnf("posting vod: #%d to channel: #%s failed: %s", vod.Seq, entry.ChannelID, err)
	}
}

func (r *VLive) postUpcomingToChannel(entry models.VliveEntry, vod models.VliveVideoInfo, vliveChannel models.VliveChannelInfo) {
	channelEmbed := &discordgo.MessageEmbed{
		Title:     helpers.GetTextF("plugins.vlive.channel-embed-title-upcoming", vliveChannel.Name, vod.Date),
		URL:       vliveChannel.Url,
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: vliveChannel.ProfileImgUrl},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.vlive.embed-footer"),
			IconURL: helpers.GetText("plugins.vlive.embed-footer-imageurl"),
		},
		Description: fmt.Sprintf("**%s**", vod.Title),
		Image:       &discordgo.MessageEmbedImage{URL: vod.Thumbnail},
		Color:       helpers.GetDiscordColorFromHex(vliveChannel.Color),
	}
	mentionText := ""
	if entry.MentionRoleID != "" {
		mentionText = fmt.Sprintf("<@&%s>\n", entry.MentionRoleID)
	}
	postText := fmt.Sprintf("<%s>", vliveChannel.Url)
	_, err := helpers.SendComplex(entry.ChannelID, &discordgo.MessageSend{
		Content: mentionText + postText,
		Embed:   channelEmbed,
	})
	if err != nil {
		cache.GetLogger().WithField("module", "vlive").Warnf("posting upcoming: #%d to channel: #%s failed: %s", vod.Seq, entry.ChannelID, err)
	}
}

func (r *VLive) postLiveToChannel(entry models.VliveEntry, vod models.VliveVideoInfo, vliveChannel models.VliveChannelInfo) {
	channelEmbed := &discordgo.MessageEmbed{
		Title:     helpers.GetTextF("plugins.vlive.channel-embed-title-live", vliveChannel.Name),
		URL:       vod.Url,
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: vliveChannel.ProfileImgUrl},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.vlive.embed-footer"),
			IconURL: helpers.GetText("plugins.vlive.embed-footer-imageurl"),
		},
		Description: fmt.Sprintf("**%s**", vod.Title),
		Image:       &discordgo.MessageEmbedImage{URL: vod.Thumbnail},
		Color:       helpers.GetDiscordColorFromHex(vliveChannel.Color),
	}
	mentionText := ""
	if entry.MentionRoleID != "" {
		mentionText = fmt.Sprintf("<@&%s>\n", entry.MentionRoleID)
	}
	_, err := helpers.SendComplex(entry.ChannelID, &discordgo.MessageSend{
		Content: mentionText + fmt.Sprintf("<%s>", vod.Url),
		Embed:   channelEmbed,
	})
	if err != nil {
		cache.GetLogger().WithField("module", "vlive").Warnf("posting live: #%d to channel: #%s failed: %s", vod.Seq, entry.ChannelID, err)
	}
}

func (r *VLive) postNoticeToChannel(entry models.VliveEntry, notice models.VliveNoticeInfo, vliveChannel models.VliveChannelInfo) {
	channelEmbed := &discordgo.MessageEmbed{
		Title:     helpers.GetTextF("plugins.vlive.channel-embed-title-notice", vliveChannel.Name),
		URL:       notice.Url,
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: vliveChannel.ProfileImgUrl},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.vlive.embed-footer"),
			IconURL: helpers.GetText("plugins.vlive.embed-footer-imageurl"),
		},
		Description: fmt.Sprintf("**%s**", notice.Title),
		Image:       &discordgo.MessageEmbedImage{URL: notice.ImageUrl},
		Color:       helpers.GetDiscordColorFromHex(vliveChannel.Color),
	}
	mentionText := ""
	if entry.MentionRoleID != "" {
		mentionText = fmt.Sprintf("<@&%s>\n", entry.MentionRoleID)
	}
	_, err := helpers.SendComplex(entry.ChannelID, &discordgo.MessageSend{
		Content: mentionText + fmt.Sprintf("<%s>", notice.Url),
		Embed:   channelEmbed,
	})
	if err != nil {
		cache.GetLogger().WithField("module", "vlive").Warnf("posting notice: #%d to channel: #%s failed: %s", notice.Number, entry.ChannelID, err)
	}
}

func (r *VLive) postCelebToChannel(entry models.VliveEntry, celeb models.VliveCelebInfo, vliveChannel models.VliveChannelInfo) {
	channelEmbed := &discordgo.MessageEmbed{
		Title:     helpers.GetTextF("plugins.vlive.channel-embed-title-celeb", vliveChannel.Name),
		URL:       celeb.Url,
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: vliveChannel.ProfileImgUrl},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.vlive.embed-footer"),
			IconURL: helpers.GetText("plugins.vlive.embed-footer-imageurl"),
		},
		Description: fmt.Sprintf("%s ...", celeb.Summary),
		Color:       helpers.GetDiscordColorFromHex(vliveChannel.Color),
	}
	mentionText := ""
	if entry.MentionRoleID != "" {
		mentionText = fmt.Sprintf("<@&%s>\n", entry.MentionRoleID)
	}
	_, err := helpers.SendComplex(entry.ChannelID, &discordgo.MessageSend{
		Content: mentionText + fmt.Sprintf("<%s>", celeb.Url),
		Embed:   channelEmbed,
	})
	if err != nil {
		cache.GetLogger().WithField("module", "vlive").Warnf("posting celeb: #%s to channel: #%s failed: %s", celeb.ID, entry.ChannelID, err)
	}
}
