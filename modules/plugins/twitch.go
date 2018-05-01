package plugins

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"net/url"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/getsentry/raven-go"
	"github.com/globalsign/mgo/bson"
)

type Twitch struct{}

const (
	twitchStatsEndpoint = "https://api.twitch.tv/kraken/streams/%s"
	twitchHexColor      = "#6441a5"
)

type TwitchStatus struct {
	Stream struct {
		ID          int64     `json:"_id"`
		Game        string    `json:"game"`
		Viewers     int       `json:"viewers"`
		VideoHeight int       `json:"video_height"`
		AverageFps  float64   `json:"average_fps"`
		Delay       int       `json:"delay"`
		CreatedAt   time.Time `json:"created_at"`
		IsPlaylist  bool      `json:"is_playlist"`
		Preview     struct {
			Small    string `json:"small"`
			Medium   string `json:"medium"`
			Large    string `json:"large"`
			Template string `json:"template"`
		} `json:"preview"`
		Channel struct {
			Mature                       bool        `json:"mature"`
			Partner                      bool        `json:"partner"`
			Status                       string      `json:"status"`
			BroadcasterLanguage          string      `json:"broadcaster_language"`
			DisplayName                  string      `json:"display_name"`
			Game                         string      `json:"game"`
			Language                     string      `json:"language"`
			ID                           int         `json:"_id"`
			Name                         string      `json:"name"`
			CreatedAt                    time.Time   `json:"created_at"`
			UpdatedAt                    time.Time   `json:"updated_at"`
			Delay                        interface{} `json:"delay"`
			Logo                         string      `json:"logo"`
			Banner                       interface{} `json:"banner"`
			VideoBanner                  string      `json:"video_banner"`
			Background                   interface{} `json:"background"`
			ProfileBanner                string      `json:"profile_banner"`
			ProfileBannerBackgroundColor interface{} `json:"profile_banner_background_color"`
			URL                          string      `json:"url"`
			Views                        int         `json:"views"`
			Followers                    int         `json:"followers"`
			Links                        struct {
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
	cache.GetLogger().WithField("module", "twitch").Info("Started twitch loop (60s)")
}
func (m *Twitch) checkTwitchFeedsLoop() {
	defer helpers.Recover()
	defer func() {
		go func() {
			cache.GetLogger().WithField("module", "twitch").Info("The checkTwitchFeedsLoop died. Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			m.checkTwitchFeedsLoop()
		}()
	}()

	var entries []models.TwitchEntry
	var bundledEntries map[string][]models.TwitchEntry

	for {
		err := helpers.MDbIterWithoutLogging(helpers.MdbCollection(models.TwitchTable).Find(nil)).All(&entries)
		helpers.Relax(err)

		bundledEntries = make(map[string][]models.TwitchEntry, 0)

		for _, entry := range entries {
			channel, err := helpers.GetChannelWithoutApi(entry.ChannelID)
			if err != nil || channel == nil || channel.ID == "" {
				//cache.GetLogger().WithField("module", "twitch").Warn(fmt.Sprintf("skipped twitch @%s for Channel #%s on Guild #%s: channel not found!",
				//	entry.TwitchChannelName, entry.ChannelID, entry.ServerID))
				continue
			}

			if _, ok := bundledEntries[entry.TwitchChannelName]; ok {
				bundledEntries[entry.TwitchChannelName] = append(bundledEntries[entry.TwitchChannelName], entry)
			} else {
				bundledEntries[entry.TwitchChannelName] = []models.TwitchEntry{entry}
			}
		}

		cache.GetLogger().WithField("module", "twitch").Infof("checking %d channels for %d feeds", len(bundledEntries), len(entries))
		start := time.Now()

		// TODO: Check multiple entries at once
		for twitchChannelName, entries := range bundledEntries {
			//cache.GetLogger().WithField("module", "twitch").Info(fmt.Sprintf("checking Twitch Channel %s", twitchChannelName))
			twitchStatus := m.getTwitchStatus(twitchChannelName)

			for _, entry := range entries {
				changes := false
				if twitchStatus.Links.Channel != "" {
					if entry.IsLive == false {
						if twitchStatus.Stream.ID != 0 {
							go func(gEntry models.TwitchEntry, gTwitchStatus TwitchStatus) {
								defer helpers.Recover()
								m.postTwitchLiveToChannel(gEntry, gTwitchStatus)
							}(entry, twitchStatus)
							entry.IsLive = true
							changes = true
						}
					} else {
						if twitchStatus.Stream.ID == 0 {
							entry.IsLive = false
							changes = true
						}
					}
				}

				if changes == true {
					err = helpers.MDbUpdateWithoutLogging(models.TwitchTable, entry.ID, entry)
					helpers.Relax(err)
				}
			}
		}

		elapsed := time.Since(start)
		cache.GetLogger().WithField("module", "twitch").Infof("checked %d channels for %d feeds, took %s", len(bundledEntries), len(entries), elapsed)
		metrics.TwitchRefreshTime.Set(elapsed.Seconds())

		time.Sleep(30 * time.Second)
	}
}

func (m *Twitch) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermTwitch) {
		return
	}

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
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
						return
					}
					targetTwitchChannelName = args[1]
				} else {
					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
					return
				}
				targetGuild, err = helpers.GetGuild(targetChannel.GuildID)
				helpers.Relax(err)
				mentionRole := new(discordgo.Role)
				if len(args) >= 4 {
					mentionRoleName := strings.ToLower(args[3])
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
				}
				// create new entry in db
				newID, err := helpers.MDbInsert(
					models.TwitchTable,
					models.TwitchEntry{
						GuildID:           targetChannel.GuildID,
						ChannelID:         targetChannel.ID,
						TwitchChannelName: targetTwitchChannelName,
						IsLive:            false,
						MentionRoleID:     mentionRole.ID,
					},
				)
				helpers.Relax(err)

				_, err = helpers.EventlogLog(time.Now(), targetChannel.GuildID, helpers.MdbIdToHuman(newID),
					models.EventlogTargetTypeRobyulTwitchFeed, msg.Author.ID,
					models.EventlogTypeRobyulTwitchFeedAdd, "",
					nil,
					[]models.ElasticEventlogOption{
						{
							Key:   "twitch_feed_channelname",
							Value: targetTwitchChannelName,
						},
						{
							Key:   "twitch_feed_mentionroleid",
							Value: mentionRole.ID,
							Type:  models.EventlogTargetTypeRole,
						},
					}, false)
				helpers.RelaxLog(err)

				_, err = helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.twitch.channel-added-success", targetTwitchChannelName, targetChannel.ID))
				helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
				cache.GetLogger().WithField("module", "twitch").Info(fmt.Sprintf("Added Twitch Channel %s to Channel %s (#%s) on Guild %s (#%s)", targetTwitchChannelName, targetChannel.Name, targetChannel.ID, targetGuild.Name, targetGuild.ID))
			})
		case "delete", "del", "remove": // [p]twitch delete <id>
			helpers.RequireMod(msg, func() {
				if len(args) >= 2 {
					session.ChannelTyping(msg.ChannelID)

					channel, err := helpers.GetChannel(msg.ChannelID)
					helpers.Relax(err)

					entryId := args[1]

					var entryBucket models.TwitchEntry
					err = helpers.MdbOne(
						helpers.MdbCollection(models.TwitchTable).Find(bson.M{"guildid": channel.GuildID, "_id": helpers.HumanToMdbId(entryId)}),
						&entryBucket,
					)
					if helpers.IsMdbNotFound(err) {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.twitch.channel-delete-not-found-error"))
						return
					}
					helpers.Relax(err)

					err = helpers.MDbDelete(models.TwitchTable, entryBucket.ID)
					helpers.Relax(err)

					_, err = helpers.EventlogLog(time.Now(), entryBucket.GuildID, helpers.MdbIdToHuman(entryBucket.ID),
						models.EventlogTargetTypeRobyulTwitchFeed, msg.Author.ID,
						models.EventlogTypeRobyulTwitchFeedRemove, "",
						nil,
						[]models.ElasticEventlogOption{
							{
								Key:   "twitch_feed_channelname",
								Value: entryBucket.TwitchChannelName,
							},
							{
								Key:   "twitch_feed_mentionroleid",
								Value: entryBucket.MentionRoleID,
								Type:  models.EventlogTargetTypeRole,
							},
						}, false)
					helpers.RelaxLog(err)

					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.twitch.channel-delete-success", entryBucket.TwitchChannelName))
					cache.GetLogger().WithField("module", "twitch").Info(fmt.Sprintf("Deleted Twitch Channel %s", entryBucket.TwitchChannelName))

				} else {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					return
				}
			})
		case "list": // [p]twitch list
			currentChannel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)
			var entryBucket []models.TwitchEntry
			err = helpers.MDbIter(helpers.MdbCollection(models.TwitchTable).Find(bson.M{"guildid": currentChannel.GuildID})).All(&entryBucket)
			helpers.Relax(err)

			if entryBucket == nil || len(entryBucket) <= 0 {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.twitch.channel-list-no-channels-error"))
				return
			} else if err != nil {
				helpers.Relax(err)
			}

			resultMessage := ""
			for _, entry := range entryBucket {
				var mentionText string
				if entry.MentionRoleID != "" {
					role, err := session.State.Role(currentChannel.GuildID, entry.MentionRoleID)
					helpers.Relax(err)
					mentionText += fmt.Sprintf(" mentioning `@%s`", role.Name)
				}
				resultMessage += fmt.Sprintf("`%s`: Twitch Channel `%s` posting to <#%s>%s\n", helpers.MdbIdToHuman(entry.ID), entry.TwitchChannelName, entry.ChannelID, mentionText)
			}
			resultMessage += fmt.Sprintf("Found **%d** Twitch Channels in total.", len(entryBucket))
			_, err = helpers.SendMessage(msg.ChannelID, resultMessage)
			helpers.RelaxMessage(err, msg.ChannelID, msg.ID)
		default:
			if args[0] == "" {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				return
			}
			session.ChannelTyping(msg.ChannelID)
			twitchStatus := m.getTwitchStatus(args[0])
			if twitchStatus.Stream.ID == 0 {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.twitch.no-channel-information"))
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
				if twitchStatus.Stream.Preview.Medium != "" {
					twitchChannelEmbed.Image = &discordgo.MessageEmbedImage{URL: twitchStatus.Stream.Preview.Medium + "?" + strconv.FormatInt(time.Now().Unix(), 10)}
				}
				if twitchStatus.Stream.Channel.Status != "" {
					twitchChannelEmbed.Description += fmt.Sprintf("**%s**\n", twitchStatus.Stream.Channel.Status)
				}
				if twitchStatus.Stream.Game != "" {
					twitchChannelEmbed.Description += fmt.Sprintf("playing **%s**\n", twitchStatus.Stream.Game)
				}
				if twitchChannelEmbed.Description != "" {
					twitchChannelEmbed.Description = strings.Trim(twitchChannelEmbed.Description, "\n")
				}
				_, err := helpers.SendEmbed(msg.ChannelID, twitchChannelEmbed)
				helpers.Relax(err)
				return
			}
		}
	}
}

func (m *Twitch) getTwitchStatus(name string) TwitchStatus {
	var twitchStatus TwitchStatus

	client := &http.Client{
		Timeout: time.Duration(10 * time.Second),
	}

	request, err := http.NewRequest("GET", fmt.Sprintf(twitchStatsEndpoint, name), nil)
	if err != nil {
		panic(err)
	}

	request.Header.Set("User-Agent", helpers.DEFAULT_UA)
	request.Header.Set("Client-ID", helpers.GetConfig().Path("twitch.token").Data().(string))

	response, err := client.Do(request)
	if err != nil {
		if errU, ok := err.(*url.Error); ok {
			cache.GetLogger().WithField("module", "twitch").Warnf("twitch status request failed: %#v", errU.Err)
			return twitchStatus
		} else {
			raven.CaptureError(fmt.Errorf("%#v", err), map[string]string{})
		}
		panic(err)
	}

	defer response.Body.Close()

	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, response.Body)
	if err != nil {
		panic(err)
	}

	json.Unmarshal(buf.Bytes(), &twitchStatus)
	return twitchStatus
}

func (m *Twitch) postTwitchLiveToChannel(entry models.TwitchEntry, twitchStatus TwitchStatus) {
	twitchStreamName := twitchStatus.Stream.Channel.DisplayName
	if strings.ToLower(twitchStatus.Stream.Channel.Name) != strings.ToLower(twitchStatus.Stream.Channel.DisplayName) {
		twitchStreamName += fmt.Sprintf(" (%s)", twitchStatus.Stream.Channel.Name)
	}
	var mentionText string
	if entry.MentionRoleID != "" {
		mentionText = fmt.Sprintf("<@&%s>\n", entry.MentionRoleID)
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
	}
	if twitchStatus.Stream.Channel.Status != "" {
		twitchChannelEmbed.Description += fmt.Sprintf("**%s**\n", twitchStatus.Stream.Channel.Status)
	}
	if twitchStatus.Stream.Game != "" {
		twitchChannelEmbed.Description += fmt.Sprintf("playing **%s**\n", twitchStatus.Stream.Game)
	}
	if twitchChannelEmbed.Description != "" {
		twitchChannelEmbed.Description = strings.Trim(twitchChannelEmbed.Description, "\n")
	}
	_, err := helpers.SendComplex(entry.ChannelID, &discordgo.MessageSend{
		Content: mentionText + fmt.Sprintf("<%s>", twitchStatus.Stream.Channel.URL),
		Embed:   twitchChannelEmbed,
	})
	helpers.Relax(err)
}
