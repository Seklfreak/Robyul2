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
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/getsentry/raven-go"
	rethink "github.com/gorethink/gorethink"
)

type Twitch struct{}

const (
	twitchStatsEndpoint = "https://api.twitch.tv/kraken/streams/%s"
	twitchHexColor      = "#6441a5"
)

type DB_TwitchChannel struct {
	ID                string `gorethink:"id,omitempty"`
	ServerID          string `gorethink:"serverid"`
	ChannelID         string `gorethink:"channelid"`
	TwitchChannelName string `gorethink:"twitchchannelname"`
	IsLive            bool   `gorethink:"islive"`
}

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

	var entries []DB_TwitchChannel
	var bundledEntries map[string][]DB_TwitchChannel

	for {
		cursor, err := rethink.Table("twitch").Run(helpers.GetDB())
		helpers.Relax(err)

		err = cursor.All(&entries)
		helpers.Relax(err)

		bundledEntries = make(map[string][]DB_TwitchChannel, 0)

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
				bundledEntries[entry.TwitchChannelName] = []DB_TwitchChannel{entry}
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
				}

				if changes == true {
					m.setEntry(entry)
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
				// create new entry in db
				entry := m.getEntryByOrCreateEmpty("id", "")
				entry.ServerID = targetChannel.GuildID
				entry.ChannelID = targetChannel.ID
				entry.TwitchChannelName = targetTwitchChannelName
				m.setEntry(entry)

				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.twitch.channel-added-success", targetTwitchChannelName, entry.ChannelID))
				cache.GetLogger().WithField("module", "twitch").Info(fmt.Sprintf("Added Twitch Channel %s to Channel %s (#%s) on Guild %s (#%s)", targetTwitchChannelName, targetChannel.Name, entry.ChannelID, targetGuild.Name, targetGuild.ID))
			})
		case "delete", "del": // [p]twitch delete <id>
			helpers.RequireMod(msg, func() {
				if len(args) >= 2 {
					session.ChannelTyping(msg.ChannelID)
					entryId := args[1]
					entryBucket := m.getEntryBy("id", entryId)
					if entryBucket.ID != "" {
						m.deleteEntryById(entryBucket.ID)

						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.twitch.channel-delete-success", entryBucket.TwitchChannelName))
						cache.GetLogger().WithField("module", "twitch").Info(fmt.Sprintf("Deleted Twitch Channel %s", entryBucket.TwitchChannelName))
					} else {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.twitch.channel-delete-not-found-error"))
						return
					}

				} else {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
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
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.twitch.channel-list-no-channels-error"))
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
				_, err := helpers.SendMessage(msg.ChannelID, resultPage)
				helpers.Relax(err)
			}
		default:
			if args[0] == "" {
				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.invalid"))
				helpers.Relax(err)
				return
			}
			session.ChannelTyping(msg.ChannelID)
			twitchStatus := m.getTwitchStatus(args[0])
			if twitchStatus.Stream.ID == 0 {
				_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.twitch.no-channel-information"))
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

func (m *Twitch) getEntryBy(key string, id string) DB_TwitchChannel {
	var entryBucket DB_TwitchChannel
	listCursor, err := rethink.Table("twitch").Filter(
		rethink.Row.Field(key).Eq(id),
	).Run(helpers.GetDB())
	if err != nil {
		panic(err)
	}
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
	if err != nil {
		panic(err)
	}
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
	_, err := helpers.SendComplex(channelID, &discordgo.MessageSend{
		Content: fmt.Sprintf("<%s>", twitchStatus.Stream.Channel.URL),
		Embed:   twitchChannelEmbed,
	})
	helpers.Relax(err)
}
