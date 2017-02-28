package plugins

import (
	"fmt"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	rethink "github.com/gorethink/gorethink"
	"github.com/shkh/lastfm-go/lastfm"
	"strconv"
	"strings"
	"time"
)

type LastFm struct{}

const (
	lastfmHexColor     string = "#d51007"
	lastfmFriendlyUser string = "https://www.last.fm/user/%s"
)

var (
	lastfmClient *lastfm.Api
)

type DB_LastFmAccount struct {
	UserID         string `gorethink:"userid,omitempty"`
	LastFmUsername string `gorethink:"lastfmusername"`
}

func (m *LastFm) Commands() []string {
	return []string{
		"lastfm",
		"lf",
	}
}

func (m *LastFm) Init(session *discordgo.Session) {
	lastfmClient = lastfm.New(helpers.GetConfig().Path("lastfm.api_key").Data().(string), helpers.GetConfig().Path("lastfm.api_secret").Data().(string))
}

func (m *LastFm) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	args := strings.Split(content, " ")
	lastfmUsername := m.getLastFmUsername(msg.Author.ID)
	subCom := ""
	if len(args) >= 1 {
		subCom = args[0]
	}
	if subCom != "" || lastfmUsername != "" {
		switch subCom {
		case "set":
			if len(args) >= 2 {
				lastfmUsername = args[1]

				lastFmAccount := m.getLastFmAccountOrCreate(msg.Author.ID)
				lastFmAccount.LastFmUsername = lastfmUsername
				m.setLastFmAccount(lastFmAccount)

				session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.lastfm.set-username-success", lastfmUsername))
			} else {
				session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
				return
			}
		case "np", "nowplaying":
			if len(args) >= 2 {
				lastfmUsername = args[1]
				targetUser, err := helpers.GetUserFromMention(lastfmUsername)
				if err == nil {
					lastfmUsername = m.getLastFmUsername(targetUser.ID)
				}
			}
			if lastfmUsername == "" {
				session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.lastfm.too-few"))
				return
			}
			session.ChannelTyping(msg.ChannelID)
			lastfmRecentTracks, err := lastfmClient.User.GetRecentTracks(lastfm.P{
				"limit": 1,
				"user":  lastfmUsername,
			})
			metrics.LastFmRequests.Add(1)
			if err != nil {
				if e, ok := err.(*lastfm.LastfmError); ok {
					session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Error: `%s`", e.Message))
					return
				}
			}
			if lastfmRecentTracks.Total > 0 {
				lastTrack := lastfmRecentTracks.Tracks[0]
				lastTrackEmbedTitle := helpers.GetTextF("plugins.lastfm.lasttrack-embed-title-last", lastfmUsername)
				if lastTrack.NowPlaying == "true" {
					lastTrackEmbedTitle = helpers.GetTextF("plugins.lastfm.lasttrack-embed-title-np", lastfmUsername)
				}
				lastTrackEmbed := &discordgo.MessageEmbed{
					Title:       lastTrackEmbedTitle,
					URL:         lastTrack.Url,
					Description: fmt.Sprintf("**%s** by **%s**", lastTrack.Name, lastTrack.Artist.Name),
					Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.lastfm.embed-footer")},
					Fields:      []*discordgo.MessageEmbedField{},
					Color:       helpers.GetDiscordColorFromHex(lastfmHexColor),
				}
				if lastTrack.Album.Name != "" {
					lastTrackEmbed.Fields = append(lastTrackEmbed.Fields, &discordgo.MessageEmbedField{Name: "Album", Value: lastTrack.Album.Name, Inline: true})
				}
				if len(lastTrack.Images) > 0 {
					for _, image := range lastTrack.Images {
						if image.Size == "large" {
							lastTrackEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: image.Url}
						}
					}
				}
				_, err = session.ChannelMessageSendEmbed(msg.ChannelID, lastTrackEmbed)
				helpers.Relax(err)
			} else {
				session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.lastfm.no-recent-tracks"))
				return
			}
		case "topalbums":
			if len(args) >= 2 {
				lastfmUsername = args[1]
				targetUser, err := helpers.GetUserFromMention(lastfmUsername)
				if err == nil {
					lastfmUsername = m.getLastFmUsername(targetUser.ID)
				}
			}
			if lastfmUsername == "" {
				session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.lastfm.too-few"))
				return
			}
			session.ChannelTyping(msg.ChannelID)
			lastfmTopAlbums, err := lastfmClient.User.GetTopAlbums(lastfm.P{
				"limit":  10,
				"period": "overall",
				"user":   lastfmUsername,
			})
			metrics.LastFmRequests.Add(1)
			if err != nil {
				if e, ok := err.(*lastfm.LastfmError); ok {
					session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Error: `%s`", e.Message))
					return
				}
			}
			if lastfmTopAlbums.Total > 0 {
				topAlbumsEmbed := &discordgo.MessageEmbed{
					Title:       helpers.GetTextF("plugins.lastfm.topalbums-embed-title", lastfmUsername),
					URL:         fmt.Sprintf(lastfmFriendlyUser, lastfmTopAlbums.User),
					Description: "of **all Time**",
					Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.lastfm.embed-footer")},
					Fields:      []*discordgo.MessageEmbedField{},
					Color:       helpers.GetDiscordColorFromHex(lastfmHexColor),
				}
				for _, topAlbum := range lastfmTopAlbums.Albums {
					topAlbumsEmbed.Fields = append(topAlbumsEmbed.Fields, &discordgo.MessageEmbedField{
						Name:   fmt.Sprintf("**#%s** (%s plays)", topAlbum.Rank, topAlbum.PlayCount),
						Value:  fmt.Sprintf("**%s** by **%s**", topAlbum.Name, topAlbum.Artist.Name),
						Inline: false})
				}
				_, err = session.ChannelMessageSendEmbed(msg.ChannelID, topAlbumsEmbed)
				helpers.Relax(err)
			} else {
				session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.lastfm.no-recent-tracks"))
				return
			}
		case "topartists":
			if len(args) >= 2 {
				lastfmUsername = args[1]
				targetUser, err := helpers.GetUserFromMention(lastfmUsername)
				if err == nil {
					lastfmUsername = m.getLastFmUsername(targetUser.ID)
				}
			}
			if lastfmUsername == "" {
				session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.lastfm.too-few"))
				return
			}
			session.ChannelTyping(msg.ChannelID)
			lastfmTopArtists, err := lastfmClient.User.GetTopArtists(lastfm.P{
				"limit":  10,
				"period": "overall",
				"user":   lastfmUsername,
			})
			metrics.LastFmRequests.Add(1)
			if err != nil {
				if e, ok := err.(*lastfm.LastfmError); ok {
					session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Error: `%s`", e.Message))
					return
				}
			}
			if lastfmTopArtists.Total > 0 {
				topArtistsEmbed := &discordgo.MessageEmbed{
					Title:       helpers.GetTextF("plugins.lastfm.topartists-embed-title", lastfmUsername),
					URL:         fmt.Sprintf(lastfmFriendlyUser, lastfmTopArtists.User),
					Description: "of **all Time**",
					Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.lastfm.embed-footer")},
					Fields:      []*discordgo.MessageEmbedField{},
					Color:       helpers.GetDiscordColorFromHex(lastfmHexColor),
				}
				for _, topArtist := range lastfmTopArtists.Artists {
					topArtistsEmbed.Fields = append(topArtistsEmbed.Fields, &discordgo.MessageEmbedField{
						Name:   fmt.Sprintf("**#%s** (%s plays)", topArtist.Rank, topArtist.PlayCount),
						Value:  fmt.Sprintf("**%s**", topArtist.Name),
						Inline: false})
				}
				_, err = session.ChannelMessageSendEmbed(msg.ChannelID, topArtistsEmbed)
				helpers.Relax(err)
			} else {
				session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.lastfm.no-recent-tracks"))
				return
			}
		case "toptracks":
			if len(args) >= 2 {
				lastfmUsername = args[1]
				targetUser, err := helpers.GetUserFromMention(lastfmUsername)
				if err == nil {
					lastfmUsername = m.getLastFmUsername(targetUser.ID)
				}
			}
			if lastfmUsername == "" {
				session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.lastfm.too-few"))
				return
			}
			session.ChannelTyping(msg.ChannelID)
			lastfmTopTracks, err := lastfmClient.User.GetTopTracks(lastfm.P{
				"limit":  10,
				"period": "overall",
				"user":   lastfmUsername,
			})
			metrics.LastFmRequests.Add(1)
			if err != nil {
				if e, ok := err.(*lastfm.LastfmError); ok {
					session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Error: `%s`", e.Message))
					return
				}
			}
			if lastfmTopTracks.Total > 0 {
				topTracksEmbed := &discordgo.MessageEmbed{
					Title:       helpers.GetTextF("plugins.lastfm.toptracks-embed-title", lastfmUsername),
					URL:         fmt.Sprintf(lastfmFriendlyUser, lastfmTopTracks.User),
					Description: "of **all Time**",
					Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.lastfm.embed-footer")},
					Fields:      []*discordgo.MessageEmbedField{},
					Color:       helpers.GetDiscordColorFromHex(lastfmHexColor),
				}
				for _, topTrack := range lastfmTopTracks.Tracks {
					topTracksEmbed.Fields = append(topTracksEmbed.Fields, &discordgo.MessageEmbedField{
						Name:   fmt.Sprintf("**#%s** (%s plays)", topTrack.Rank, topTrack.PlayCount),
						Value:  fmt.Sprintf("**%s** by **%s**", topTrack.Name, topTrack.Artist.Name),
						Inline: false})
				}
				_, err = session.ChannelMessageSendEmbed(msg.ChannelID, topTracksEmbed)
				helpers.Relax(err)
			} else {
				session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.lastfm.no-recent-tracks"))
				return
			}
		default:
			if subCom != "" {
				lastfmUsername = subCom
				targetUser, err := helpers.GetUserFromMention(lastfmUsername)
				if err == nil {
					lastfmUsername = m.getLastFmUsername(targetUser.ID)
				}
			}
			session.ChannelTyping(msg.ChannelID)
			lastfmUser, err := lastfmClient.User.GetInfo(lastfm.P{"user": lastfmUsername})
			if err != nil {
				if e, ok := err.(*lastfm.LastfmError); ok {
					session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Error: `%s`", e.Message))
					return
				}
			}
			metrics.LastFmRequests.Add(1)
			scrobblesCount, err := strconv.Atoi(lastfmUser.PlayCount)
			helpers.Relax(err)
			embedTitle := helpers.GetTextF("plugins.lastfm.profile-embed-title", lastfmUser.Name)
			if lastfmUser.RealName != "" {
				embedTitle = helpers.GetTextF("plugins.lastfm.profile-embed-title-realname", lastfmUser.RealName, lastfmUser.Name)
			}
			accountEmbed := &discordgo.MessageEmbed{
				Title:  embedTitle,
				URL:    lastfmUser.Url,
				Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.lastfm.embed-footer")},
				Fields: []*discordgo.MessageEmbedField{
					{Name: "Scrobbles", Value: humanize.Comma(int64(scrobblesCount)), Inline: true}},
				Color: helpers.GetDiscordColorFromHex(lastfmHexColor),
			}
			if len(lastfmUser.Images) > 0 {
				for _, image := range lastfmUser.Images {
					if image.Size == "large" {
						accountEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: image.Url}
					}
				}
			}
			if lastfmUser.Country != "" {
				accountEmbed.Fields = append(accountEmbed.Fields, &discordgo.MessageEmbedField{Name: "Country", Value: lastfmUser.Country, Inline: true})
			}
			if lastfmUser.Age != "" && lastfmUser.Age != "0" {
				accountEmbed.Fields = append(accountEmbed.Fields, &discordgo.MessageEmbedField{Name: "Age", Value: lastfmUser.Age + " years", Inline: true})
			}
			if lastfmUser.Gender != "" && lastfmUser.Gender != "n" {
				if lastfmUser.Gender == "f" {
					accountEmbed.Fields = append(accountEmbed.Fields, &discordgo.MessageEmbedField{Name: "Gender", Value: "Female", Inline: true})
				} else if lastfmUser.Gender == "m" {
					accountEmbed.Fields = append(accountEmbed.Fields, &discordgo.MessageEmbedField{Name: "Gender", Value: "Male", Inline: true})
				} else {
					accountEmbed.Fields = append(accountEmbed.Fields, &discordgo.MessageEmbedField{Name: "Gender", Value: lastfmUser.Gender, Inline: true})
				}
			}
			if lastfmUser.Registered.Unixtime != "" {
				timeI, _ := strconv.ParseInt(lastfmUser.Registered.Unixtime, 10, 64)
				if err == nil {
					timeRegistered := time.Unix(timeI, 0)
					accountEmbed.Fields = append(accountEmbed.Fields, &discordgo.MessageEmbedField{Name: "Account Creation", Value: humanize.Time(timeRegistered), Inline: true})
				} else {
					helpers.SendError(msg, err)
				}
			}
			_, err = session.ChannelMessageSendEmbed(msg.ChannelID, accountEmbed)
			helpers.Relax(err)
		}
	} else {
		session.ChannelMessageSend(msg.ChannelID, helpers.GetTextF("plugins.lastfm.too-few"))
		return
	}

}

func (m *LastFm) getLastFmUsername(uid string) string {
	var lastfmAccountBucket DB_LastFmAccount
	listCursor, err := rethink.Table("lastfm").Filter(
		rethink.Row.Field("userid").Eq(uid),
	).Run(helpers.GetDB())
	defer listCursor.Close()
	err = listCursor.One(&lastfmAccountBucket)

	// If user has no DB entries create an empty document
	if err == rethink.ErrEmptyResult {
		return ""
	} else if err != nil {
		panic(err)
	}

	return lastfmAccountBucket.LastFmUsername
}

func (m *LastFm) getLastFmAccountOrCreate(uid string) DB_LastFmAccount {
	var lastfmAccountBucket DB_LastFmAccount
	listCursor, err := rethink.Table("lastfm").Filter(
		rethink.Row.Field("userid").Eq(uid),
	).Run(helpers.GetDB())
	defer listCursor.Close()
	err = listCursor.One(&lastfmAccountBucket)

	// If user has no DB entries create an empty document
	if err == rethink.ErrEmptyResult {
		_, e := rethink.Table("lastfm").Insert(DB_LastFmAccount{
			UserID:         uid,
			LastFmUsername: "",
		}).RunWrite(helpers.GetDB())

		// If the creation was successful read the document
		if e != nil {
			panic(e)
		} else {
			return m.getLastFmAccountOrCreate(uid)
		}
	} else if err != nil {
		panic(err)
	}

	return lastfmAccountBucket
}

func (m *LastFm) setLastFmAccount(entry DB_LastFmAccount) {
	_, err := rethink.Table("lastfm").Update(entry).Run(helpers.GetDB())
	helpers.Relax(err)
}
