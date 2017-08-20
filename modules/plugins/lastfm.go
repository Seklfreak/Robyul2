package plugins

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/bradfitz/slice"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	rethink "github.com/gorethink/gorethink"
	"github.com/shkh/lastfm-go/lastfm"
)

type LastFm struct{}

const (
	lastfmHexColor     string = "#d51007"
	lastfmFriendlyUser string = "https://www.last.fm/user/%s"
)

var (
	lastfmClient             *lastfm.Api
	lastfmCachedStats        []LastFMAccountCachedStats
	lastfmCombinedGuildStats []LastFMCombinedGuildStats
)

type DB_LastFmAccount struct {
	UserID         string `gorethink:"userid,omitempty"`
	LastFmUsername string `gorethink:"lastfmusername"`
}

type LastFMAccount_Safe_Entries struct {
	entries []DB_LastFmAccount
	mux     sync.Mutex
}

type LastFMSongInfo struct {
	Name       string
	Url        string
	ArtistName string
	ArtistURL  string
	ImageURL   string
	Plays      int
}

type LastFMAccountCachedStats struct {
	UserID      string
	Overall     []LastFMSongInfo
	SevenDay    []LastFMSongInfo
	OneMonth    []LastFMSongInfo
	ThreeMonth  []LastFMSongInfo
	SixMonth    []LastFMSongInfo
	TwelveMonth []LastFMSongInfo
}

type LastFMCombinedGuildStats struct {
	GuildID       string
	NumberOfUsers int
	Overall       []LastFMSongInfo
	SevenDay      []LastFMSongInfo
	OneMonth      []LastFMSongInfo
	ThreeMonth    []LastFMSongInfo
	SixMonth      []LastFMSongInfo
	TwelveMonth   []LastFMSongInfo
}

func (m *LastFm) Commands() []string {
	return []string{
		"lastfm",
		"lf",
	}
}

func (m *LastFm) Init(session *discordgo.Session) {
	lastfmClient = lastfm.New(helpers.GetConfig().Path("lastfm.api_key").Data().(string), helpers.GetConfig().Path("lastfm.api_secret").Data().(string))
	lastfmCachedStats = make([]LastFMAccountCachedStats, 0)
	lastfmCombinedGuildStats = make([]LastFMCombinedGuildStats, 0)

	go m.generateDiscordStats()
}

func (m *LastFm) generateDiscordStats() {
	var safeEntries LastFMAccount_Safe_Entries
	log := cache.GetLogger()

	defer helpers.Recover()
	defer func() {
		go func() {
			log.WithField("module", "lastfm").Error("The generateDiscordStats died. Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			m.generateDiscordStats()
		}()
	}()

	for {
		cursor, err := rethink.Table("lastfm").Run(helpers.GetDB())
		helpers.Relax(err)

		err = cursor.All(&safeEntries.entries)
		helpers.Relax(err)

		// Get Stats from LastFM
		newLastfmCachedStats := make([]LastFMAccountCachedStats, 0)
		for _, safeAccount := range safeEntries.entries {
			safeEntries.mux.Lock()
			newLastFmCachedStat := new(LastFMAccountCachedStats)
			newLastFmCachedStat.UserID = safeAccount.UserID
			periods := []string{"overall", "7day", "1month", "3month", "6month", "12month"}

			for _, period := range periods {
				lastfmTopTracks, err := lastfmClient.User.GetTopTracks(lastfm.P{
					"limit":  50,
					"user":   safeAccount.LastFmUsername,
					"period": period,
				})
				metrics.LastFmRequests.Add(1)
				if err != nil {
					log.WithField("module", "lastfm").Error(fmt.Sprintf("getting %s stats for last.fm user %s failed: %s", period, safeAccount.LastFmUsername, err.Error()))
					continue
				}

				if lastfmTopTracks.Total > 0 {
					for _, track := range lastfmTopTracks.Tracks {
						imageUrl := ""
						if len(track.Images) > 0 {
							imageUrl = track.Images[0].Url
						}
						playCount, err := strconv.Atoi(track.PlayCount)
						if err != nil {
							playCount = 1
						}
						songInfo := LastFMSongInfo{
							Name:       track.Name,
							Url:        track.Url,
							ArtistName: track.Artist.Name,
							ArtistURL:  track.Artist.Url,
							ImageURL:   imageUrl,
							Plays:      playCount,
						}
						switch period {
						case "overall":
							newLastFmCachedStat.Overall = append(newLastFmCachedStat.Overall, songInfo)
							break
						case "7day":
							newLastFmCachedStat.SevenDay = append(newLastFmCachedStat.SevenDay, songInfo)
							break
						case "1month":
							newLastFmCachedStat.OneMonth = append(newLastFmCachedStat.OneMonth, songInfo)
							break
						case "3month":
							newLastFmCachedStat.ThreeMonth = append(newLastFmCachedStat.ThreeMonth, songInfo)
							break
						case "6month":
							newLastFmCachedStat.SixMonth = append(newLastFmCachedStat.SixMonth, songInfo)
							break
						case "12month":
							newLastFmCachedStat.TwelveMonth = append(newLastFmCachedStat.TwelveMonth, songInfo)
							break
						}
					}
				}
			}
			newLastfmCachedStats = append(newLastfmCachedStats, *newLastFmCachedStat)
			safeEntries.mux.Unlock()
		}
		lastfmCachedStats = newLastfmCachedStats

		// Combine Stats
		newCombinedGuildStats := make([]LastFMCombinedGuildStats, 0)
		for _, guild := range cache.GetSession().State.Guilds {
			newCombinedGuildStat := new(LastFMCombinedGuildStats)
			newCombinedGuildStat.GuildID = guild.ID
			newCombinedGuildStat.NumberOfUsers = 0

			lastAfterMemberId := ""
			for {
				members, err := cache.GetSession().GuildMembers(guild.ID, lastAfterMemberId, 1000)
				if err != nil {
					continue
				}
				if len(members) <= 0 {
					break
				}
				lastAfterMemberId = members[len(members)-1].User.ID
				for _, member := range members {
					for _, cachedStat := range lastfmCachedStats {
						if cachedStat.UserID == member.User.ID {
							// User is on Guild
							newCombinedGuildStat.NumberOfUsers += 1
							// Append tracks
							for _, track := range cachedStat.Overall {
								added := false
								for i, trackInDb := range newCombinedGuildStat.Overall {
									if strings.ToLower(trackInDb.Name) == strings.ToLower(track.Name) &&
										strings.ToLower(trackInDb.ArtistName) == strings.ToLower(track.ArtistName) {
										newCombinedGuildStat.Overall[i].Plays += track.Plays
										added = true
									}
								}
								if added == false {
									newCombinedGuildStat.Overall = append(newCombinedGuildStat.Overall, track)
								}
							}
							for _, track := range cachedStat.SevenDay {
								added := false
								for i, trackInDb := range newCombinedGuildStat.SevenDay {
									if strings.ToLower(trackInDb.Name) == strings.ToLower(track.Name) &&
										strings.ToLower(trackInDb.ArtistName) == strings.ToLower(track.ArtistName) {
										newCombinedGuildStat.SevenDay[i].Plays += track.Plays
										added = true
									}
								}
								if added == false {
									newCombinedGuildStat.SevenDay = append(newCombinedGuildStat.SevenDay, track)
								}
							}
							for _, track := range cachedStat.OneMonth {
								added := false
								for i, trackInDb := range newCombinedGuildStat.OneMonth {
									if strings.ToLower(trackInDb.Name) == strings.ToLower(track.Name) &&
										strings.ToLower(trackInDb.ArtistName) == strings.ToLower(track.ArtistName) {
										newCombinedGuildStat.OneMonth[i].Plays += track.Plays
										added = true
									}
								}
								if added == false {
									newCombinedGuildStat.OneMonth = append(newCombinedGuildStat.OneMonth, track)
								}
							}
							for _, track := range cachedStat.ThreeMonth {
								added := false
								for i, trackInDb := range newCombinedGuildStat.ThreeMonth {
									if strings.ToLower(trackInDb.Name) == strings.ToLower(track.Name) &&
										strings.ToLower(trackInDb.ArtistName) == strings.ToLower(track.ArtistName) {
										newCombinedGuildStat.ThreeMonth[i].Plays += track.Plays
										added = true
									}
								}
								if added == false {
									newCombinedGuildStat.ThreeMonth = append(newCombinedGuildStat.ThreeMonth, track)
								}
							}
							for _, track := range cachedStat.SixMonth {
								added := false
								for i, trackInDb := range newCombinedGuildStat.SixMonth {
									if strings.ToLower(trackInDb.Name) == strings.ToLower(track.Name) &&
										strings.ToLower(trackInDb.ArtistName) == strings.ToLower(track.ArtistName) {
										newCombinedGuildStat.SixMonth[i].Plays += track.Plays
										added = true
									}
								}
								if added == false {
									newCombinedGuildStat.SixMonth = append(newCombinedGuildStat.SixMonth, track)
								}
							}
							for _, track := range cachedStat.TwelveMonth {
								added := false
								for i, trackInDb := range newCombinedGuildStat.TwelveMonth {
									if strings.ToLower(trackInDb.Name) == strings.ToLower(track.Name) &&
										strings.ToLower(trackInDb.ArtistName) == strings.ToLower(track.ArtistName) {
										newCombinedGuildStat.TwelveMonth[i].Plays += track.Plays
										added = true
									}
								}
								if added == false {
									newCombinedGuildStat.TwelveMonth = append(newCombinedGuildStat.TwelveMonth, track)
								}
							}
						}
					}
				}
			}
			newCombinedGuildStats = append(newCombinedGuildStats, *newCombinedGuildStat)
		}
		for n := range newCombinedGuildStats {
			slice.Sort(newCombinedGuildStats[n].Overall[:], func(i, j int) bool {
				return newCombinedGuildStats[n].Overall[i].Plays > newCombinedGuildStats[n].Overall[j].Plays
			})
			slice.Sort(newCombinedGuildStats[n].SevenDay[:], func(i, j int) bool {
				return newCombinedGuildStats[n].SevenDay[i].Plays > newCombinedGuildStats[n].SevenDay[j].Plays
			})
			slice.Sort(newCombinedGuildStats[n].OneMonth[:], func(i, j int) bool {
				return newCombinedGuildStats[n].OneMonth[i].Plays > newCombinedGuildStats[n].OneMonth[j].Plays
			})
			slice.Sort(newCombinedGuildStats[n].ThreeMonth[:], func(i, j int) bool {
				return newCombinedGuildStats[n].ThreeMonth[i].Plays > newCombinedGuildStats[n].ThreeMonth[j].Plays
			})
			slice.Sort(newCombinedGuildStats[n].SixMonth[:], func(i, j int) bool {
				return newCombinedGuildStats[n].SixMonth[i].Plays > newCombinedGuildStats[n].SixMonth[j].Plays
			})
			slice.Sort(newCombinedGuildStats[n].TwelveMonth[:], func(i, j int) bool {
				return newCombinedGuildStats[n].TwelveMonth[i].Plays > newCombinedGuildStats[n].TwelveMonth[j].Plays
			})
		}

		lastfmCombinedGuildStats = newCombinedGuildStats

		time.Sleep(6 * time.Hour)
	}
}

func (m *LastFm) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	args := strings.Fields(content)
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
				helpers.RelaxEmbed(err, msg.ChannelID)
			} else {
				session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.lastfm.no-recent-tracks"))
				return
			}
		case "topalbums", "topalbum":
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
				helpers.RelaxEmbed(err, msg.ChannelID)
			} else {
				session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.lastfm.no-recent-tracks"))
				return
			}
		case "topartists", "topartist":
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
				helpers.RelaxEmbed(err, msg.ChannelID)
			} else {
				session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.lastfm.no-recent-tracks"))
				return
			}
		case "toptracks", "topsongs", "toptrack", "topsong":
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
				helpers.RelaxEmbed(err, msg.ChannelID)
			} else {
				session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.lastfm.no-recent-tracks"))
				return
			}
		case "discord-top", "server-top":
			if len(args) < 1 {
				session.ChannelMessageSend(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
				return
			}
			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)
			guild, err := helpers.GetGuild(channel.GuildID)
			helpers.Relax(err)

			var combinedStats LastFMCombinedGuildStats
			for _, combinedStatsN := range lastfmCombinedGuildStats {
				if combinedStatsN.GuildID == guild.ID {
					combinedStats = combinedStatsN
				}
			}

			if combinedStats.GuildID == "" {
				session.ChannelMessageSend(msg.ChannelID, helpers.GetText("plugins.lastfm.no-stats-available"))
				return
			}

			topTracks := make([]LastFMSongInfo, 0)
			timeString := "all time"
			topTracks = combinedStats.Overall
			if len(args) >= 2 {
				switch args[1] {
				case "overall":
					timeString = "all time"
					topTracks = combinedStats.Overall
					break
				case "7days", "week":
					timeString = "the last seven days"
					topTracks = combinedStats.SevenDay
					break
				case "1month", "month":
					timeString = "the last month"
					topTracks = combinedStats.OneMonth
					break
				case "3month":
					timeString = "the last three months"
					topTracks = combinedStats.ThreeMonth
					break
				case "6month":
					timeString = "the last six months"
					topTracks = combinedStats.SixMonth
					break
				case "12month", "year":
					timeString = "the last twelve months"
					topTracks = combinedStats.TwelveMonth
					break
				}
			}

			topTracksEmbed := &discordgo.MessageEmbed{
				Title:       helpers.GetTextF("plugins.lastfm.toptracks-embed-title", fmt.Sprintf("%s Server", guild.Name)),
				Description: fmt.Sprintf("of **%s**", timeString),
				Footer: &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("%s | %d last.fm users on this server",
					helpers.GetText("plugins.lastfm.embed-footer"), combinedStats.NumberOfUsers)},
				Fields: []*discordgo.MessageEmbedField{},
				Color:  helpers.GetDiscordColorFromHex(lastfmHexColor),
			}
			for i, topTrack := range topTracks {
				topTracksEmbed.Fields = append(topTracksEmbed.Fields, &discordgo.MessageEmbedField{
					Name:   fmt.Sprintf("**#%s** (%s plays)", strconv.Itoa(i+1), strconv.Itoa(topTrack.Plays)),
					Value:  fmt.Sprintf("**%s** by **%s**", topTrack.Name, topTrack.ArtistName),
					Inline: false})
				if i == 9 {
					break
				}
			}
			_, err = session.ChannelMessageSendEmbed(msg.ChannelID, topTracksEmbed)
			helpers.RelaxEmbed(err, msg.ChannelID)
			break
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
			helpers.RelaxEmbed(err, msg.ChannelID)
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
	_, err := rethink.Table("lastfm").Filter(
		rethink.Row.Field("userid").Eq(entry.UserID),
	).Update(entry).RunWrite(helpers.GetDB())
	helpers.Relax(err)
}
