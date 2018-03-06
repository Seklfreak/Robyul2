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

	"github.com/PuerkitoBio/goquery"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

type Charts struct{}

const (
	melonEndpointCharts         = "http://apis.skplanetx.com/melon/charts/%s?version=1&page=1&count=10"
	melonEndpointSongSearch     = "http://apis.skplanetx.com/melon/songs?version=1&page=1&count=2&searchKeyword=%s"
	melonEndpointArtistSearch   = "http://apis.skplanetx.com/melon/artists?version=1&page=1&count=2&searchKeyword=%s"
	melonEndpointAlbumSearch    = "http://apis.skplanetx.com/melon/albums?version=1&page=1&count=2&searchKeyword=%s"
	melonFriendlyRealtimeStats  = "http://www.melon.com/chart/index.htm"
	melonFriendlyDailyStats     = "http://www.melon.com/chart/day/index.htm"
	melonFriendlySongDetails    = "http://www.melon.com/song/detail.htm?songId=%s"
	melonFriendlyArtistDetails  = "http://www.melon.com/artist/detail.htm?artistId=%d"
	melonFriendlyAlbumDetails   = "http://www.melon.com/album/detail.htm?albumId=%d"
	ichartPageRealtimeCharts    = "http://www.instiz.net/iframe_ichart_score.htm?real=1"
	ichartPageWeeklyCharts      = "http://www.instiz.net/iframe_ichart_score.htm?week=1"
	ichartFriendlyRealtimeStats = "http://www.instiz.net/bbs/list.php?id=spage&no=8"
	ichartFriendlyWeeklyStats   = "http://www.instiz.net/bbs/list.php?id=spage&no=8"
	gaonPageWeeklyCharts        = "http://gaonchart.co.kr/main/section/chart/album.gaon?nationGbn=T&serviceGbn=&termGbn=week"
	gaonPageMonthlyCharts       = "http://gaonchart.co.kr/main/section/chart/album.gaon?nationGbn=T&serviceGbn=&termGbn=month"
	gaonPageYearlyCharts        = "http://gaonchart.co.kr/main/section/chart/album.gaon?nationGbn=T&serviceGbn=&termGbn=year"
	gaonFriendlyWeeklyCharts    = "http://gaonchart.co.kr/main/section/chart/album.gaon?nationGbn=T&serviceGbn=&termGbn=week"
	gaonFriendlyMonthlyCharts   = "http://gaonchart.co.kr/main/section/chart/album.gaon?nationGbn=T&serviceGbn=&termGbn=month"
	gaonFriendlyYearlyCharts    = "http://gaonchart.co.kr/main/section/chart/album.gaon?nationGbn=T&serviceGbn=&termGbn=year"
)

func (m *Charts) Commands() []string {
	return []string{
		"melon",
		"ichart",
		"gaon",
	}
}

type MelonRealTimeStats struct {
	Melon struct {
		MenuID     int    `json:"menuId"`
		Count      int    `json:"count"`
		Page       int    `json:"page"`
		TotalPages int    `json:"totalPages"`
		RankDay    string `json:"rankDay"`
		RankHour   string `json:"rankHour"`
		Songs      struct {
			Song []struct {
				SongID   int    `json:"songId"`
				SongName string `json:"songName"`
				Artists  struct {
					Artist []struct {
						ArtistID   int    `json:"artistId"`
						ArtistName string `json:"artistName"`
					} `json:"artist"`
				} `json:"artists"`
				AlbumID     int    `json:"albumId"`
				AlbumName   string `json:"albumName"`
				CurrentRank int    `json:"currentRank"`
				PastRank    int    `json:"pastRank"`
				PlayTime    int    `json:"playTime"`
				IssueDate   string `json:"issueDate"`
				IsTitleSong string `json:"isTitleSong"`
				IsHitSong   string `json:"isHitSong"`
				IsAdult     string `json:"isAdult"`
				IsFree      string `json:"isFree"`
			} `json:"song"`
		} `json:"songs"`
	} `json:"melon"`
}

type MelonDailyStats struct {
	Melon struct {
		MenuID     int `json:"menuId"`
		Count      int `json:"count"`
		Page       int `json:"page"`
		TotalPages int `json:"totalPages"`
		Songs      struct {
			Song []struct {
				SongID   int    `json:"songId"`
				SongName string `json:"songName"`
				Artists  struct {
					Artist []struct {
						ArtistID   int    `json:"artistId"`
						ArtistName string `json:"artistName"`
					} `json:"artist"`
				} `json:"artists"`
				AlbumID     int    `json:"albumId"`
				AlbumName   string `json:"albumName"`
				CurrentRank int    `json:"currentRank"`
				PastRank    int    `json:"pastRank"`
				RankDay     int    `json:"rankDay"`
				PlayTime    int    `json:"playTime"`
				IsTitleSong string `json:"isTitleSong"`
				IsHitSong   string `json:"isHitSong"`
				IsAdult     string `json:"isAdult"`
				IsFree      string `json:"isFree"`
			} `json:"song"`
		} `json:"songs"`
	} `json:"melon"`
}

type MelonSearchSongResults struct {
	Melon struct {
		MenuID     int `json:"menuId"`
		Count      int `json:"count"`
		Page       int `json:"page"`
		TotalPages int `json:"totalPages"`
		TotalCount int `json:"totalCount"`
		Songs      struct {
			Song []struct {
				SongID    string `json:"songId"`
				SongName  string `json:"songName"`
				AlbumID   int    `json:"albumId"`
				AlbumName string `json:"albumName"`
				Artists   struct {
					Artist []struct {
						ArtistID   int    `json:"artistId"`
						ArtistName string `json:"artistName"`
					} `json:"artist"`
				} `json:"artists"`
				PlayTime    int    `json:"playTime"`
				IssueDate   string `json:"issueDate"`
				IsTitleSong string `json:"isTitleSong"`
				IsHitSong   string `json:"isHitSong"`
				IsAdult     string `json:"isAdult"`
				IsFree      string `json:"isFree"`
			} `json:"song"`
		} `json:"songs"`
	} `json:"melon"`
}

type MelonSearchArtistsResults struct {
	Melon struct {
		MenuID     int `json:"menuId"`
		Count      int `json:"count"`
		Page       int `json:"page"`
		TotalPages int `json:"totalPages"`
		TotalCount int `json:"totalCount"`
		Artists    struct {
			Artist []struct {
				ArtistID        int    `json:"artistId"`
				ArtistName      string `json:"artistName"`
				Sex             string `json:"sex"`
				NationalityName string `json:"nationalityName"`
				ActTypeName     string `json:"actTypeName"`
				GenreNames      string `json:"genreNames"`
			} `json:"artist"`
		} `json:"artists"`
	} `json:"melon"`
}

type MelonSearchAlbumsResults struct {
	Melon struct {
		MenuID     int `json:"menuId"`
		Count      int `json:"count"`
		Page       int `json:"page"`
		TotalPages int `json:"totalPages"`
		TotalCount int `json:"totalCount"`
		Albums     struct {
			Album []struct {
				AlbumID   int    `json:"albumId"`
				AlbumName string `json:"albumName"`
				Artists   struct {
					Artist []struct {
						ArtistID   int    `json:"artistId"`
						ArtistName string `json:"artistName"`
					} `json:"artist"`
				} `json:"artists"`
				AverageScore string `json:"averageScore"`
				IssueDate    string `json:"issueDate"`
			} `json:"album"`
		} `json:"albums"`
	} `json:"melon"`
}

type GenericSongScore struct {
	Title         string
	Artist        string
	Album         string
	CurrentRank   int
	PastRank      int
	IsNew         bool
	MusicVideoUrl string
}
type GenericAlbumScore struct {
	Artist      string
	Album       string
	CurrentRank int
	PastRank    int
	IsNew       bool
}

func (m *Charts) Init(session *discordgo.Session) {
}

func (m *Charts) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermCharts) {
		return
	}

	args := strings.Fields(content)

	switch command {
	case "melon":
		if len(args) >= 1 {
			switch args[0] {
			case "realtime":
				session.ChannelTyping(msg.ChannelID)
				realtimeStats := m.GetMelonRealtimeStats()
				chartsEmbed := &discordgo.MessageEmbed{
					Title:  helpers.GetTextF("plugins.charts.realtime-melon-embed-title", realtimeStats.Melon.RankDay, realtimeStats.Melon.RankHour),
					URL:    melonFriendlyRealtimeStats,
					Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.charts.melon-embed-footer")},
					Fields: []*discordgo.MessageEmbedField{},
					Color:  helpers.GetDiscordColorFromHex(helpers.GetText("plugins.charts.melon-embed-hex-color")),
				}
				for _, song := range realtimeStats.Melon.Songs.Song {
					artistText := ""
					for i, artist := range song.Artists.Artist {
						artistText += fmt.Sprintf("[%s](%s)",
							artist.ArtistName, fmt.Sprintf(melonFriendlyArtistDetails, artist.ArtistID))
						if i+1 < len(song.Artists.Artist) {
							artistText += ", "
						} else if (len(song.Artists.Artist) - (i + 1)) > 0 {
							artistText += " and "
						}
					}
					rankChange := ""
					rankChangeN := song.PastRank - song.CurrentRank
					if rankChangeN > 0 {
						rankChange = fmt.Sprintf(":arrow_up: %d", rankChangeN)
					} else if rankChangeN < 0 {
						rankChange = fmt.Sprintf(":arrow_down:  %d", rankChangeN*-1)
					}

					chartsEmbed.Fields = append(chartsEmbed.Fields, &discordgo.MessageEmbedField{
						Name: fmt.Sprintf("**#%s** %s", strconv.Itoa(song.CurrentRank), rankChange),
						Value: fmt.Sprintf("[**%s**](%s) by **%s** (on [%s](%s))",
							song.SongName, fmt.Sprintf(melonFriendlySongDetails, strconv.Itoa(song.SongID)),
							artistText,
							song.AlbumName, fmt.Sprintf(melonFriendlyAlbumDetails, song.AlbumID)),
					})
				}
				_, err := helpers.SendEmbed(msg.ChannelID, chartsEmbed)
				helpers.Relax(err)
				return
			case "daily":
				session.ChannelTyping(msg.ChannelID)
				dailyStats := m.GetMelonDailyStats()
				chartsEmbed := &discordgo.MessageEmbed{
					Title:  helpers.GetText("plugins.charts.daily-melon-embed-title"),
					URL:    melonFriendlyDailyStats,
					Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.charts.melon-embed-footer")},
					Fields: []*discordgo.MessageEmbedField{},
					Color:  helpers.GetDiscordColorFromHex(helpers.GetText("plugins.charts.melon-embed-hex-color")),
				}
				for _, song := range dailyStats.Melon.Songs.Song {
					artistText := ""
					for i, artist := range song.Artists.Artist {
						artistText += fmt.Sprintf("[%s](%s)",
							artist.ArtistName, fmt.Sprintf(melonFriendlyArtistDetails, artist.ArtistID))
						if i+1 < len(song.Artists.Artist) {
							artistText += ", "
						} else if (len(song.Artists.Artist) - (i + 1)) > 0 {
							artistText += " and "
						}
					}
					rankChange := ""
					rankChangeN := song.PastRank - song.CurrentRank
					if rankChangeN > 0 {
						rankChange = fmt.Sprintf(":arrow_up: %d", rankChangeN)
					} else if rankChangeN < 0 {
						rankChange = fmt.Sprintf(":arrow_down:  %d", rankChangeN*-1)
					}

					chartsEmbed.Fields = append(chartsEmbed.Fields, &discordgo.MessageEmbedField{
						Name: fmt.Sprintf("**#%s** %s", strconv.Itoa(song.CurrentRank), rankChange),
						Value: fmt.Sprintf("[**%s**](%s) by **%s** (on [%s](%s))",
							song.SongName, fmt.Sprintf(melonFriendlySongDetails, strconv.Itoa(song.SongID)),
							artistText,
							song.AlbumName, fmt.Sprintf(melonFriendlyAlbumDetails, song.AlbumID)),
					})
				}
				_, err := helpers.SendEmbed(msg.ChannelID, chartsEmbed)
				helpers.Relax(err)
				return
			case "song":
				session.ChannelTyping(msg.ChannelID)
				searchString, err := helpers.UrlEncode(strings.Join(args[1:], " "))
				helpers.Relax(err)

				var searchResult MelonSearchSongResults
				result := m.DoMelonRequest(fmt.Sprintf(melonEndpointSongSearch, searchString))

				json.Unmarshal(result, &searchResult)

				if searchResult.Melon.Count <= 0 {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.charts.search-no-result"))
					helpers.Relax(err)
					return
				}

				melonSong := searchResult.Melon.Songs.Song[0]

				artistText := ""
				for i, artist := range melonSong.Artists.Artist {
					artistText += fmt.Sprintf("[%s](%s)",
						artist.ArtistName, fmt.Sprintf(melonFriendlyArtistDetails, artist.ArtistID))
					if i+1 < len(melonSong.Artists.Artist) {
						artistText += ", "
					} else if (len(melonSong.Artists.Artist) - (i + 1)) > 0 {
						artistText += " and "
					}
				}

				durationText := strconv.Itoa(melonSong.PlayTime) + "s"
				if melonSong.PlayTime >= 60 {
					minutes := melonSong.PlayTime / 60
					seconds := melonSong.PlayTime % 60
					durationText = fmt.Sprintf("%dm%ds", minutes, seconds)
				}

				songEmbed := &discordgo.MessageEmbed{
					Title: helpers.GetText("plugins.charts.search-melon-embed-title"),
					Description: fmt.Sprintf("[**%s**](%s) by **%s**",
						melonSong.SongName, fmt.Sprintf(melonFriendlySongDetails, melonSong.SongID),
						artistText),
					Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.charts.melon-embed-footer")},
					Fields: []*discordgo.MessageEmbedField{
						{Name: "Album", Value: fmt.Sprintf("[%s](%s)", melonSong.AlbumName, fmt.Sprintf(melonFriendlyAlbumDetails, melonSong.AlbumID)), Inline: true},
						{Name: "Duration", Value: durationText, Inline: true},
					},
					Color: helpers.GetDiscordColorFromHex(helpers.GetText("plugins.charts.melon-embed-hex-color")),
				}

				_, err = helpers.SendComplex(msg.ChannelID,
					&discordgo.MessageSend{
						Content: "<" + fmt.Sprintf(melonFriendlySongDetails, melonSong.SongID) + ">",
						Embed:   songEmbed,
					})
				helpers.Relax(err)
				return
			case "artist":
				session.ChannelTyping(msg.ChannelID)
				searchString, err := helpers.UrlEncode(strings.Join(args[1:], " "))
				helpers.Relax(err)

				var searchResult MelonSearchArtistsResults
				result := m.DoMelonRequest(fmt.Sprintf(melonEndpointArtistSearch, searchString))

				json.Unmarshal(result, &searchResult)

				if searchResult.Melon.Count <= 0 {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.charts.search-no-result"))
					helpers.Relax(err)
					return
				}

				melonArtist := searchResult.Melon.Artists.Artist[0]

				genderText := "Not specified"
				if melonArtist.Sex != "" {
					genderText = melonArtist.Sex
					if melonArtist.Sex == "M" {
						genderText = "Male"
					} else if melonArtist.Sex == "F" {
						genderText = "Female"
					}
				}
				genreText := "Not specified"
				if melonArtist.GenreNames != "" {
					genreText = melonArtist.GenreNames
				}

				artistEmbed := &discordgo.MessageEmbed{
					Title: helpers.GetText("plugins.charts.search-melon-embed-title"),
					Description: fmt.Sprintf("[**%s**](%s)",
						melonArtist.ArtistName, fmt.Sprintf(melonFriendlyArtistDetails, melonArtist.ArtistID)),
					Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.charts.melon-embed-footer")},
					Fields: []*discordgo.MessageEmbedField{
						{Name: "Gender", Value: genderText, Inline: true},
						{Name: "Genres", Value: genreText, Inline: true},
					},
					Color: helpers.GetDiscordColorFromHex(helpers.GetText("plugins.charts.melon-embed-hex-color")),
				}

				_, err = helpers.SendComplex(msg.ChannelID, &discordgo.MessageSend{
					Content: "<" + fmt.Sprintf(melonFriendlyArtistDetails, melonArtist.ArtistID) + ">",
					Embed:   artistEmbed,
				})
				helpers.Relax(err)
				return
			case "album":
				session.ChannelTyping(msg.ChannelID)
				searchString, err := helpers.UrlEncode(strings.Join(args[1:], " "))
				helpers.Relax(err)

				var searchResult MelonSearchAlbumsResults
				result := m.DoMelonRequest(fmt.Sprintf(melonEndpointAlbumSearch, searchString))

				json.Unmarshal(result, &searchResult)

				if searchResult.Melon.Count <= 0 {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.charts.search-no-result"))
					helpers.Relax(err)
					return
				}

				melonAlbum := searchResult.Melon.Albums.Album[0]

				artistText := ""
				for i, artist := range melonAlbum.Artists.Artist {
					artistText += fmt.Sprintf("[%s](%s)",
						artist.ArtistName, fmt.Sprintf(melonFriendlyArtistDetails, artist.ArtistID))
					if i+1 < len(melonAlbum.Artists.Artist) {
						artistText += ", "
					} else if (len(melonAlbum.Artists.Artist) - (i + 1)) > 0 {
						artistText += " and "
					}
				}

				artistEmbed := &discordgo.MessageEmbed{
					Title: helpers.GetText("plugins.charts.search-melon-embed-title"),
					Description: fmt.Sprintf("[**%s**](%s)",
						melonAlbum.AlbumName, fmt.Sprintf(melonFriendlyAlbumDetails, melonAlbum.AlbumID)),
					Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.charts.melon-embed-footer")},
					Fields: []*discordgo.MessageEmbedField{
						{Name: "Artists", Value: artistText, Inline: true},
						{Name: "Date", Value: melonAlbum.IssueDate, Inline: true},
					},
					Color: helpers.GetDiscordColorFromHex(helpers.GetText("plugins.charts.melon-embed-hex-color")),
				}

				_, err = helpers.SendComplex(msg.ChannelID, &discordgo.MessageSend{
					Content: "<" + fmt.Sprintf(melonFriendlyAlbumDetails, melonAlbum.AlbumID) + ">",
					Embed:   artistEmbed,
				})
				helpers.Relax(err)
				return
			}
		}
	case "ichart":
		if len(args) >= 1 {
			switch args[0] {
			case "realtime":
				session.ChannelTyping(msg.ChannelID)
				time, songRanks, maintenance, overloaded := m.GetIChartRealtimeStats()

				if maintenance == true {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.charts.ichart-maintenance"))
					helpers.Relax(err)
					return
				}
				if overloaded == true {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.charts.ichart-overloaded"))
					helpers.Relax(err)
					return
				}

				chartsEmbed := &discordgo.MessageEmbed{
					Title:  helpers.GetTextF("plugins.charts.realtime-ichart-embed-title", time),
					URL:    ichartFriendlyRealtimeStats,
					Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.charts.ichart-embed-footer")},
					Fields: []*discordgo.MessageEmbedField{},
					Color:  helpers.GetDiscordColorFromHex(helpers.GetText("plugins.charts.ichart-embed-hex-color")),
				}
				for _, song := range songRanks {
					rankChange := ""
					rankChangeN := song.PastRank - song.CurrentRank
					if rankChangeN > 0 {
						rankChange = fmt.Sprintf(":arrow_up: %d", rankChangeN)
					} else if rankChangeN < 0 {
						rankChange = fmt.Sprintf(":arrow_down:  %d", rankChangeN*-1)
					}
					if song.IsNew == true {
						rankChange += ":new:"
					}

					chartsFieldValue := fmt.Sprintf("**%s** by **%s** (on %s)", song.Title, song.Artist, song.Album)
					if song.MusicVideoUrl != "" {
						chartsFieldValue = fmt.Sprintf("[%s](%s)", chartsFieldValue, song.MusicVideoUrl)
					}

					chartsEmbed.Fields = append(chartsEmbed.Fields, &discordgo.MessageEmbedField{
						Name:  fmt.Sprintf("**#%s** %s", strconv.Itoa(song.CurrentRank), rankChange),
						Value: chartsFieldValue,
					})
				}
				_, err := helpers.SendEmbed(msg.ChannelID, chartsEmbed)
				helpers.Relax(err)
			case "week", "weekly":
				session.ChannelTyping(msg.ChannelID)
				time, songRanks, maintenance, overloaded := m.GetIChartWeekStats()

				if maintenance == true {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.charts.ichart-maintenance"))
					helpers.Relax(err)
					return
				}
				if overloaded == true {
					_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.charts.ichart-overloaded"))
					helpers.Relax(err)
					return
				}

				chartsEmbed := &discordgo.MessageEmbed{
					Title:  helpers.GetTextF("plugins.charts.week-ichart-embed-title", time),
					URL:    ichartFriendlyWeeklyStats,
					Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.charts.ichart-embed-footer")},
					Fields: []*discordgo.MessageEmbedField{},
					Color:  helpers.GetDiscordColorFromHex(helpers.GetText("plugins.charts.ichart-embed-hex-color")),
				}
				for _, song := range songRanks {
					rankChange := ""
					rankChangeN := song.PastRank - song.CurrentRank
					if rankChangeN > 0 {
						rankChange = fmt.Sprintf(":arrow_up: %d", rankChangeN)
					} else if rankChangeN < 0 {
						rankChange = fmt.Sprintf(":arrow_down:  %d", rankChangeN*-1)
					}
					if song.IsNew == true {
						rankChange += ":new:"
					}
					chartsFieldValue := fmt.Sprintf("**%s** by **%s** (on %s)", song.Title, song.Artist, song.Album)
					if song.MusicVideoUrl != "" {
						chartsFieldValue = fmt.Sprintf("[%s](%s)", chartsFieldValue, song.MusicVideoUrl)
					}

					chartsEmbed.Fields = append(chartsEmbed.Fields, &discordgo.MessageEmbedField{
						Name:  fmt.Sprintf("**#%s** %s", strconv.Itoa(song.CurrentRank), rankChange),
						Value: chartsFieldValue,
					})
				}
				_, err := helpers.SendEmbed(msg.ChannelID, chartsEmbed)
				helpers.Relax(err)
			}
		}
	case "gaon":
		if len(args) >= 1 {
			switch args[0] {
			case "week", "weekly":
				session.ChannelTyping(msg.ChannelID)
				time, albumRanks := m.GetGaonWeekStats()
				chartsEmbed := &discordgo.MessageEmbed{
					Title:  helpers.GetTextF("plugins.charts.week-gaon-embed-title", time),
					URL:    gaonFriendlyWeeklyCharts,
					Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.charts.gaon-embed-footer")},
					Fields: []*discordgo.MessageEmbedField{},
					Color:  helpers.GetDiscordColorFromHex(helpers.GetText("plugins.charts.gaon-embed-hex-color")),
				}
				for _, album := range albumRanks {
					rankChange := ""
					rankChangeN := album.PastRank - album.CurrentRank
					if rankChangeN > 0 {
						rankChange = fmt.Sprintf(":arrow_up: %d", rankChangeN)
					} else if rankChangeN < 0 {
						rankChange = fmt.Sprintf(":arrow_down:  %d", rankChangeN*-1)
					}
					if album.IsNew == true {
						rankChange += ":new:"
					}

					chartsEmbed.Fields = append(chartsEmbed.Fields, &discordgo.MessageEmbedField{
						Name:  fmt.Sprintf("**#%s** %s", strconv.Itoa(album.CurrentRank), rankChange),
						Value: fmt.Sprintf("**%s** by **%s**", album.Album, album.Artist),
					})
				}
				_, err := helpers.SendEmbed(msg.ChannelID, chartsEmbed)
				helpers.Relax(err)
			case "month", "monthly":
				session.ChannelTyping(msg.ChannelID)
				time, albumRanks := m.GetGaonMonthStats()
				chartsEmbed := &discordgo.MessageEmbed{
					Title:  helpers.GetTextF("plugins.charts.month-gaon-embed-title", time),
					URL:    gaonFriendlyMonthlyCharts,
					Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.charts.gaon-embed-footer")},
					Fields: []*discordgo.MessageEmbedField{},
					Color:  helpers.GetDiscordColorFromHex(helpers.GetText("plugins.charts.gaon-embed-hex-color")),
				}
				for _, album := range albumRanks {
					rankChange := ""
					rankChangeN := album.PastRank - album.CurrentRank
					if rankChangeN > 0 {
						rankChange = fmt.Sprintf(":arrow_up: %d", rankChangeN)
					} else if rankChangeN < 0 {
						rankChange = fmt.Sprintf(":arrow_down:  %d", rankChangeN*-1)
					}
					if album.IsNew == true {
						rankChange += ":new:"
					}

					chartsEmbed.Fields = append(chartsEmbed.Fields, &discordgo.MessageEmbedField{
						Name:  fmt.Sprintf("**#%s** %s", strconv.Itoa(album.CurrentRank), rankChange),
						Value: fmt.Sprintf("**%s** by **%s**", album.Album, album.Artist),
					})
				}
				_, err := helpers.SendEmbed(msg.ChannelID, chartsEmbed)
				helpers.Relax(err)
			case "year", "yearly":
				session.ChannelTyping(msg.ChannelID)
				time, albumRanks := m.GetGaonYearStats()
				chartsEmbed := &discordgo.MessageEmbed{
					Title:  helpers.GetTextF("plugins.charts.year-gaon-embed-title", time),
					URL:    gaonFriendlyYearlyCharts,
					Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.charts.gaon-embed-footer")},
					Fields: []*discordgo.MessageEmbedField{},
					Color:  helpers.GetDiscordColorFromHex(helpers.GetText("plugins.charts.gaon-embed-hex-color")),
				}
				for _, album := range albumRanks {
					rankChange := ""
					rankChangeN := album.PastRank - album.CurrentRank
					if rankChangeN > 0 {
						rankChange = fmt.Sprintf(":arrow_up: %d", rankChangeN)
					} else if rankChangeN < 0 {
						rankChange = fmt.Sprintf(":arrow_down:  %d", rankChangeN*-1)
					}
					if album.IsNew == true {
						rankChange += ":new:"
					}

					chartsEmbed.Fields = append(chartsEmbed.Fields, &discordgo.MessageEmbedField{
						Name:  fmt.Sprintf("**#%s** %s", strconv.Itoa(album.CurrentRank), rankChange),
						Value: fmt.Sprintf("**%s** by **%s**", album.Album, album.Artist),
					})
				}
				_, err := helpers.SendEmbed(msg.ChannelID, chartsEmbed)
				helpers.Relax(err)
			}
		}
	}
}

func (m *Charts) GetMelonRealtimeStats() MelonRealTimeStats {
	var realtimeStats MelonRealTimeStats
	result := m.DoMelonRequest(fmt.Sprintf(melonEndpointCharts, "realtime"))

	json.Unmarshal(result, &realtimeStats)
	return realtimeStats
}

func (m *Charts) GetMelonDailyStats() MelonDailyStats {
	var dailyStats MelonDailyStats
	result := m.DoMelonRequest(fmt.Sprintf(melonEndpointCharts, "todaytopsongs"))

	json.Unmarshal(result, &dailyStats)
	return dailyStats
}

func (m *Charts) DoMelonRequest(url string) []byte {
	client := &http.Client{
		Timeout: time.Duration(10 * time.Second),
	}

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}

	request.Header.Set("User-Agent", helpers.DEFAULT_UA)
	request.Header.Set("Accept-Language", "en_US")
	request.Header.Set("appKey", helpers.GetConfig().Path("melon.app_key").Data().(string))

	response, err := client.Do(request)
	helpers.Relax(err)

	defer response.Body.Close()

	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, response.Body)
	helpers.Relax(err)

	return buf.Bytes()
}

func (m *Charts) GetIChartRealtimeStats() (string, []GenericSongScore, bool, bool) {
	doc, err := goquery.NewDocument(ichartPageRealtimeCharts)
	helpers.Relax(err)

	var ranks []GenericSongScore

	if m.IChartInMaintenance(doc) == true {
		return "", ranks, true, false
	}

	if m.IChartOverloaded(doc) == true {
		return "", ranks, false, true
	}

	time := strings.TrimSpace(strings.Replace(doc.Find("#content > div.ichart_score_title > div.ichart_score_title_right.minitext3").Text(), "기준", "", -1))
	for i := 0; i < 10; i++ {
		ranks = append(ranks,
			GenericSongScore{
				Title:       "",
				Artist:      "",
				Album:       "",
				CurrentRank: i + 1,
				PastRank:    0,
			})
		if i == 0 {
			ranks[i].Title = doc.Find("#score_1st > div.ichart_score_song > div.ichart_score_song1 > b").Text()
			ranks[i].Artist = doc.Find("#score_1st > div.ichart_score_artist > div.ichart_score_artist1 > b").Text()
			ranks[i].Album = doc.Find("#score_1st > div.ichart_score_song > div.ichart_score_song2 > span > a").Text()
			helpers.Relax(err)
			ranks[i].PastRank = ranks[i].CurrentRank
			if musicVideoUrl, ok := doc.Find("#yttop").Attr("href"); ok {
				ranks[i].MusicVideoUrl = m.IChartHrefExtractMVUrl(musicVideoUrl)
			}
			if len(doc.Find("#score_1st > div.ichart_score_change.rank > .arrow1").Nodes) > 0 {
				pastRankUncalculated, err := strconv.Atoi(doc.Find("#score_1st > div.ichart_score_change.rank > .arrow1").Parent().Text())
				helpers.Relax(err)
				ranks[i].PastRank = ranks[i].CurrentRank + pastRankUncalculated
			}
			if len(doc.Find("#score_1st > div.ichart_score_change.rank > .arrow2").Nodes) > 0 {
				pastRankUncalculated, err := strconv.Atoi(doc.Find("#score_1st > div.ichart_score_change.rank > .arrow2").Parent().Text())
				helpers.Relax(err)
				ranks[i].PastRank = ranks[i].CurrentRank - pastRankUncalculated
			}
		} else {
			itemDoc := goquery.NewDocumentFromNode(doc.Find("#content > div.spage_intistore_body > div.spage_score_item").Get(i - 1))
			submenuDoc := goquery.NewDocumentFromNode(doc.Find("#content > div.spage_intistore_body > div.ichart_submenu").Get(i - 1))
			ranks[i].Title = itemDoc.Find("div.ichart_score2_song > div.ichart_score2_song1").Text()
			ranks[i].Artist = itemDoc.Find("div.ichart_score2_artist > div.ichart_score2_artist1").Text()
			ranks[i].Album = itemDoc.Find("div.ichart_score2_song > div.ichart_score2_song2 > span > a").Text()
			ranks[i].PastRank = ranks[i].CurrentRank
			if musicVideoUrl, ok := submenuDoc.Find("ul > li.ichart_mv > a").Attr("href"); ok {
				ranks[i].MusicVideoUrl = m.IChartHrefExtractMVUrl(musicVideoUrl)
			}
			if len(itemDoc.Find("div.ichart_score2_change.rank > .arrow1").Nodes) > 0 {
				pastRankUncalculated, err := strconv.Atoi(itemDoc.Find("div.ichart_score2_change.rank").Text())
				helpers.Relax(err)
				ranks[i].PastRank = ranks[i].CurrentRank + pastRankUncalculated
			}
			if len(itemDoc.Find("div.ichart_score2_change.rank > .arrow2").Nodes) > 0 {
				pastRankUncalculated, err := strconv.Atoi(itemDoc.Find("div.ichart_score2_change.rank").Text())
				helpers.Relax(err)
				ranks[i].PastRank = ranks[i].CurrentRank - pastRankUncalculated
			}
			if len(itemDoc.Find("div.ichart_score2_change.rank > .arrow4").Nodes) > 0 {
				ranks[i].IsNew = true
			}
		}
	}

	return time, ranks, false, false
}

func (m *Charts) IChartInMaintenance(doc *goquery.Document) bool {
	if strings.Contains(doc.Text(), "서버 점검으로 인해 현재 서비스가 일시 중단되었습니다") { // maintenance
		return true
	}
	return false
}

func (m *Charts) IChartOverloaded(doc *goquery.Document) bool {
	isOverloaded := false
	if strings.Contains(doc.Text(), "사이트 이용자가 많습니다") {
		isOverloaded = true
	}
	return isOverloaded
}

func (m *Charts) IChartHrefExtractMVUrl(musicVideoUrl string) string {
	if strings.Contains(musicVideoUrl, "javascript:show_youtube") {
		parts := strings.Split(musicVideoUrl, "'")
		if len(parts) == 3 {
			return "https://youtu.be/" + parts[1]
		}
	}
	return ""
}

func (m *Charts) GetIChartWeekStats() (string, []GenericSongScore, bool, bool) {
	doc, err := goquery.NewDocument(ichartPageWeeklyCharts)
	helpers.Relax(err)

	var ranks []GenericSongScore

	if m.IChartInMaintenance(doc) == true {
		return "", ranks, true, false
	}

	if m.IChartOverloaded(doc) == true {
		return "", ranks, false, true
	}

	time := strings.TrimSpace(strings.Replace(doc.Find("#content > div.ichart_score_title > div.ichart_score_title_right.minitext3").Text(), "기준", "", -1))
	for i := 0; i < 10; i++ {
		ranks = append(ranks,
			GenericSongScore{
				Title:       "",
				Artist:      "",
				Album:       "",
				CurrentRank: i + 1,
				PastRank:    0,
			})
		if i == 0 {
			ranks[i].Title = doc.Find("#score_1st > div.ichart_score_song > div.ichart_score_song1 > b").Text()
			ranks[i].Artist = doc.Find("#score_1st > div.ichart_score_artist > div.ichart_score_artist1 > b").Text()
			ranks[i].Album = doc.Find("#score_1st > div.ichart_score_song > div.ichart_score_song2 > span > a").Text()
			helpers.Relax(err)
			ranks[i].PastRank = ranks[i].CurrentRank
			if musicVideoUrl, ok := doc.Find("#yttop").Attr("href"); ok {
				ranks[i].MusicVideoUrl = m.IChartHrefExtractMVUrl(musicVideoUrl)
			}
			if len(doc.Find("#score_1st > div.ichart_score_change.rank > .arrow1").Nodes) > 0 {
				pastRankUncalculated, err := strconv.Atoi(doc.Find("#score_1st > div.ichart_score_change.rank > .arrow1").Parent().Text())
				helpers.Relax(err)
				ranks[i].PastRank = ranks[i].CurrentRank + pastRankUncalculated
			}
			if len(doc.Find("#score_1st > div.ichart_score_change.rank > .arrow2").Nodes) > 0 {
				pastRankUncalculated, err := strconv.Atoi(doc.Find("#score_1st > div.ichart_score_change.rank > .arrow2").Parent().Text())
				helpers.Relax(err)
				ranks[i].PastRank = ranks[i].CurrentRank - pastRankUncalculated
			}
		} else {
			itemDoc := goquery.NewDocumentFromNode(doc.Find("#content > div.spage_intistore_body > div.spage_score_item").Get(i - 1))
			submenuDoc := goquery.NewDocumentFromNode(doc.Find("#content > div.spage_intistore_body > div.ichart_submenu").Get(i - 1))
			ranks[i].Title = itemDoc.Find("div.ichart_score2_song > div.ichart_score2_song1").Text()
			ranks[i].Artist = itemDoc.Find("div.ichart_score2_artist > div.ichart_score2_artist1").Text()
			ranks[i].Album = itemDoc.Find("div.ichart_score2_song > div.ichart_score2_song2 > span > a").Text()
			ranks[i].PastRank = ranks[i].CurrentRank
			if musicVideoUrl, ok := submenuDoc.Find("ul > li.ichart_mv > a").Attr("href"); ok {
				ranks[i].MusicVideoUrl = m.IChartHrefExtractMVUrl(musicVideoUrl)
			}
			if len(itemDoc.Find("div.ichart_score2_change.rank > .arrow1").Nodes) > 0 {
				pastRankUncalculated, err := strconv.Atoi(itemDoc.Find("div.ichart_score2_change.rank").Text())
				helpers.Relax(err)
				ranks[i].PastRank = ranks[i].CurrentRank + pastRankUncalculated
			}
			if len(itemDoc.Find("div.ichart_score2_change.rank > .arrow2").Nodes) > 0 {
				pastRankUncalculated, err := strconv.Atoi(itemDoc.Find("div.ichart_score2_change.rank").Text())
				helpers.Relax(err)
				ranks[i].PastRank = ranks[i].CurrentRank - pastRankUncalculated
			}
			if len(itemDoc.Find("div.ichart_score2_change.rank > .arrow4").Nodes) > 0 {
				ranks[i].IsNew = true
			}
		}
	}

	return time, ranks, false, false
}

func (m *Charts) GetGaonWeekStats() (string, []GenericAlbumScore) {
	doc, err := goquery.NewDocument(gaonPageWeeklyCharts)
	helpers.Relax(err)

	time := strings.TrimSpace(strings.Replace(doc.Find("#wrap > div.now > div.fl").Text(), "Album Chart", "", -1))
	var ranks []GenericAlbumScore
	for i := 0; i < 10; i++ {
		ranks = append(ranks,
			GenericAlbumScore{
				Album:       "",
				CurrentRank: i + 1,
				PastRank:    0,
			})
		ranks[i].Album = doc.Find(fmt.Sprintf("#wrap > div.chart > table > tbody > tr:nth-child(%d) > td.subject > p:nth-child(1)", ranks[i].CurrentRank+1)).Text()
		ranks[i].Artist = doc.Find(fmt.Sprintf("#wrap > div.chart > table > tbody > tr:nth-child(%d) > td.subject > p.singer", ranks[i].CurrentRank+1)).Text()
		ranks[i].PastRank = ranks[i].CurrentRank
		if len(doc.Find(fmt.Sprintf("#wrap > div.chart > table > tbody > tr:nth-child(%d) > td.change > .up", ranks[i].CurrentRank+1)).Nodes) > 0 {
			pastRankUncalculated, err := strconv.Atoi(doc.Find(fmt.Sprintf("#wrap > div.chart > table > tbody > tr:nth-child(%d) > td.change > .up", ranks[i].CurrentRank+1)).Text())
			if err == nil {
				ranks[i].PastRank = ranks[i].CurrentRank + pastRankUncalculated
			}
		}
		if len(doc.Find(fmt.Sprintf("#wrap > div.chart > table > tbody > tr:nth-child(%d) > td.change > .down", ranks[i].CurrentRank+1)).Nodes) > 0 {
			pastRankUncalculated, err := strconv.Atoi(doc.Find(fmt.Sprintf("#wrap > div.chart > table > tbody > tr:nth-child(%d) > td.change > .down", ranks[i].CurrentRank+1)).Text())
			if err == nil {
				ranks[i].PastRank = ranks[i].CurrentRank - pastRankUncalculated
			}
		}
		if len(doc.Find(fmt.Sprintf("#wrap > div.chart > table > tbody > tr:nth-child(%d) > td.change > .new", ranks[i].CurrentRank+1)).Nodes) > 0 {
			ranks[i].IsNew = true
		}
	}

	return time, ranks
}

func (m *Charts) GetGaonMonthStats() (string, []GenericAlbumScore) {
	doc, err := goquery.NewDocument(gaonPageMonthlyCharts)
	helpers.Relax(err)

	time := strings.TrimSpace(strings.Replace(doc.Find("#wrap > div.now > div.fl").Text(), "Album Chart", "", -1))
	var ranks []GenericAlbumScore
	for i := 0; i < 10; i++ {
		ranks = append(ranks,
			GenericAlbumScore{
				Album:       "",
				CurrentRank: i + 1,
				PastRank:    0,
			})
		ranks[i].Album = doc.Find(fmt.Sprintf("#wrap > div.chart > table > tbody > tr:nth-child(%d) > td.subject > p:nth-child(1)", ranks[i].CurrentRank+1)).Text()
		ranks[i].Artist = doc.Find(fmt.Sprintf("#wrap > div.chart > table > tbody > tr:nth-child(%d) > td.subject > p.singer", ranks[i].CurrentRank+1)).Text()
		ranks[i].PastRank = ranks[i].CurrentRank
		if len(doc.Find(fmt.Sprintf("#wrap > div.chart > table > tbody > tr:nth-child(%d) > td.change > .up", ranks[i].CurrentRank+1)).Nodes) > 0 {
			pastRankUncalculated, err := strconv.Atoi(doc.Find(fmt.Sprintf("#wrap > div.chart > table > tbody > tr:nth-child(%d) > td.change > .up", ranks[i].CurrentRank+1)).Text())
			if err == nil {
				ranks[i].PastRank = ranks[i].CurrentRank + pastRankUncalculated
			}
		}
		if len(doc.Find(fmt.Sprintf("#wrap > div.chart > table > tbody > tr:nth-child(%d) > td.change > .down", ranks[i].CurrentRank+1)).Nodes) > 0 {
			pastRankUncalculated, err := strconv.Atoi(doc.Find(fmt.Sprintf("#wrap > div.chart > table > tbody > tr:nth-child(%d) > td.change > .down", ranks[i].CurrentRank+1)).Text())
			if err == nil {
				ranks[i].PastRank = ranks[i].CurrentRank - pastRankUncalculated
			}
		}
		if len(doc.Find(fmt.Sprintf("#wrap > div.chart > table > tbody > tr:nth-child(%d) > td.change > .new", ranks[i].CurrentRank+1)).Nodes) > 0 {
			ranks[i].IsNew = true
		}
	}

	return time, ranks
}

func (m *Charts) GetGaonYearStats() (string, []GenericAlbumScore) {
	doc, err := goquery.NewDocument(gaonPageYearlyCharts)
	helpers.Relax(err)

	time := strings.TrimSpace(strings.Replace(doc.Find("#wrap > div.now > div.fl").Text(), "Album Chart", "", -1))
	var ranks []GenericAlbumScore
	for i := 0; i < 10; i++ {
		ranks = append(ranks,
			GenericAlbumScore{
				Album:       "",
				CurrentRank: i + 1,
				PastRank:    0,
			})
		ranks[i].Album = doc.Find(fmt.Sprintf("#wrap > div.chart > table > tbody > tr:nth-child(%d) > td.subject > p:nth-child(1)", ranks[i].CurrentRank+1)).Text()
		ranks[i].Artist = doc.Find(fmt.Sprintf("#wrap > div.chart > table > tbody > tr:nth-child(%d) > td.subject > p.singer", ranks[i].CurrentRank+1)).Text()
		ranks[i].PastRank = ranks[i].CurrentRank
		if len(doc.Find(fmt.Sprintf("#wrap > div.chart > table > tbody > tr:nth-child(%d) > td.change > .up", ranks[i].CurrentRank+1)).Nodes) > 0 {
			pastRankUncalculated, err := strconv.Atoi(doc.Find(fmt.Sprintf("#wrap > div.chart > table > tbody > tr:nth-child(%d) > td.change > .up", ranks[i].CurrentRank+1)).Text())
			if err == nil {
				ranks[i].PastRank = ranks[i].CurrentRank + pastRankUncalculated
			}
		}
		if len(doc.Find(fmt.Sprintf("#wrap > div.chart > table > tbody > tr:nth-child(%d) > td.change > .down", ranks[i].CurrentRank+1)).Nodes) > 0 {
			pastRankUncalculated, err := strconv.Atoi(doc.Find(fmt.Sprintf("#wrap > div.chart > table > tbody > tr:nth-child(%d) > td.change > .down", ranks[i].CurrentRank+1)).Text())
			if err == nil {
				ranks[i].PastRank = ranks[i].CurrentRank - pastRankUncalculated
			}
		}
		if len(doc.Find(fmt.Sprintf("#wrap > div.chart > table > tbody > tr:nth-child(%d) > td.change > .new", ranks[i].CurrentRank+1)).Nodes) > 0 {
			ranks[i].IsNew = true
		}
	}

	return time, ranks
}
