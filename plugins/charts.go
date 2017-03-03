package plugins

import (
	"github.com/bwmarrin/discordgo"
	"strings"
	"net/http"
	"fmt"
	"github.com/Seklfreak/Robyul2/helpers"
	"bytes"
	"io"
	"encoding/json"
	"strconv"
	"github.com/PuerkitoBio/goquery"
)

type Charts struct{}

const (
	melonEndpointCharts         string = "http://apis.skplanetx.com/melon/charts/%s?version=1&page=1&count=10"
	melonFriendlyRealtimeStats  string = "http://www.melon.com/chart/index.htm"
	melonFriendlyDailyStats     string = "http://www.melon.com/chart/day/index.htm"
	ichartPageRealtimeCharts    string = "http://www.instiz.net/iframe_ichart_score.htm?real=1"
	ichartPageWeeklyCharts      string = "http://www.instiz.net/iframe_ichart_score.htm?week=1"
	ichartFriendlyRealtimeStats string = "http://www.instiz.net/bbs/list.php?id=spage&no=8"
	ichartFriendlyWeeklyStats   string = "http://www.instiz.net/bbs/list.php?id=spage&no=8"
)

func (m *Charts) Commands() []string {
	return []string{
		"melon",
		"ichart",
	}
}

type MelonRealTimeStats struct {
	Melon struct {
		MenuID     int `json:"menuId"`
		Count      int `json:"count"`
		Page       int `json:"page"`
		TotalPages int `json:"totalPages"`
		RankDay    string `json:"rankDay"`
		RankHour   string `json:"rankHour"`
		Songs struct {
			Song []struct {
				SongID   int `json:"songId"`
				SongName string `json:"songName"`
				Artists struct {
					Artist []struct {
						ArtistID   int `json:"artistId"`
						ArtistName string `json:"artistName"`
					} `json:"artist"`
				} `json:"artists"`
				AlbumID     int `json:"albumId"`
				AlbumName   string `json:"albumName"`
				CurrentRank int `json:"currentRank"`
				PastRank    int `json:"pastRank"`
				PlayTime    int `json:"playTime"`
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
		Songs struct {
			Song []struct {
				SongID   int `json:"songId"`
				SongName string `json:"songName"`
				Artists struct {
					Artist []struct {
						ArtistID   int `json:"artistId"`
						ArtistName string `json:"artistName"`
					} `json:"artist"`
				} `json:"artists"`
				AlbumID     int `json:"albumId"`
				AlbumName   string `json:"albumName"`
				CurrentRank int `json:"currentRank"`
				PastRank    int `json:"pastRank"`
				RankDay     int `json:"rankDay"`
				PlayTime    int `json:"playTime"`
				IsTitleSong string `json:"isTitleSong"`
				IsHitSong   string `json:"isHitSong"`
				IsAdult     string `json:"isAdult"`
				IsFree      string `json:"isFree"`
			} `json:"song"`
		} `json:"songs"`
	} `json:"melon"`
}

type GenericSongScore struct {
	Title       string
	Artist      string
	Album       string
	CurrentRank int
	PastRank    int
	IsNew       bool
}

func (m *Charts) Init(session *discordgo.Session) {
}

// todo: gaon

func (m *Charts) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	args := strings.Split(content, " ")

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
						artistText += artist.ArtistName
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
						rankChange = fmt.Sprintf(":arrow_down:  %d", rankChangeN * -1)
					}

					chartsEmbed.Fields = append(chartsEmbed.Fields, &discordgo.MessageEmbedField{
						Name:  fmt.Sprintf("**#%s** %s", strconv.Itoa(song.CurrentRank), rankChange),
						Value: fmt.Sprintf("**%s** by **%s** (on %s)", song.SongName, artistText, song.AlbumName),
					})
				}
				_, err := session.ChannelMessageSendEmbed(msg.ChannelID, chartsEmbed)
				helpers.Relax(err)
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
						artistText += artist.ArtistName
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
						rankChange = fmt.Sprintf(":arrow_down:  %d", rankChangeN * -1)
					}

					chartsEmbed.Fields = append(chartsEmbed.Fields, &discordgo.MessageEmbedField{
						Name:  fmt.Sprintf("**#%s** %s", strconv.Itoa(song.CurrentRank), rankChange),
						Value: fmt.Sprintf("**%s** by **%s** (on %s)", song.SongName, artistText, song.AlbumName),
					})
				}
				_, err := session.ChannelMessageSendEmbed(msg.ChannelID, chartsEmbed)
				helpers.Relax(err)
			}
		}
	case "ichart":
		if len(args) >= 1 {
			switch args[0] {
			case "realtime":
				session.ChannelTyping(msg.ChannelID)
				time, songRanks := m.GetIChartRealtimeStats()
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
						rankChange = fmt.Sprintf(":arrow_down:  %d", rankChangeN * -1)
					}
					if song.IsNew == true {
						rankChange += ":new:"
					}

					chartsEmbed.Fields = append(chartsEmbed.Fields, &discordgo.MessageEmbedField{
						Name:  fmt.Sprintf("**#%s** %s", strconv.Itoa(song.CurrentRank), rankChange),
						Value: fmt.Sprintf("**%s** by **%s** (on %s)", song.Title, song.Artist, song.Album),
					})
				}
				_, err := session.ChannelMessageSendEmbed(msg.ChannelID, chartsEmbed)
				helpers.Relax(err)
			case "week", "weekly":
				session.ChannelTyping(msg.ChannelID)
				time, songRanks := m.GetIChartWeekStats()
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
						rankChange = fmt.Sprintf(":arrow_down:  %d", rankChangeN * -1)
					}
					if song.IsNew == true {
						rankChange += ":new:"
					}

					chartsEmbed.Fields = append(chartsEmbed.Fields, &discordgo.MessageEmbedField{
						Name:  fmt.Sprintf("**#%s** %s", strconv.Itoa(song.CurrentRank), rankChange),
						Value: fmt.Sprintf("**%s** by **%s** (on %s)", song.Title, song.Artist, song.Album),
					})
				}
				_, err := session.ChannelMessageSendEmbed(msg.ChannelID, chartsEmbed)
				helpers.Relax(err)
			}
		}
	}
}

func (m *Charts) GetMelonRealtimeStats() MelonRealTimeStats {
	var realtimeStats MelonRealTimeStats
	result := m.DoMelonRequest("realtime")

	json.Unmarshal(result, &realtimeStats)
	return realtimeStats
}

func (m *Charts) GetMelonDailyStats() MelonDailyStats {
	var dailyStats MelonDailyStats
	result := m.DoMelonRequest("todaytopsongs")

	json.Unmarshal(result, &dailyStats)
	return dailyStats
}

func (m *Charts) DoMelonRequest(endpoint string) []byte {
	client := &http.Client{}

	request, err := http.NewRequest("GET", fmt.Sprintf(melonEndpointCharts, endpoint), nil)
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

func (m *Charts) GetIChartRealtimeStats() (string, []GenericSongScore) {
	doc, err := goquery.NewDocument(ichartPageRealtimeCharts)
	helpers.Relax(err)

	time := strings.Trim(strings.Replace(doc.Find("#content > div.ichart_score_title > div.ichart_score_title_right.minitext3").Text(), "기준", "", -1), " ")
	var ranks []GenericSongScore
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
			ranks[i].Title = doc.Find(fmt.Sprintf("#content > div.spage_intistore_body > div.spage_score_bottom > div:nth-child(%d) > div.ichart_score2_song > div.ichart_score2_song1", i*4)).Text()
			ranks[i].Artist = doc.Find(fmt.Sprintf("#content > div.spage_intistore_body > div.spage_score_bottom > div:nth-child(%d) > div.ichart_score2_artist > div.ichart_score2_artist1", i*4)).Text()
			ranks[i].Album = doc.Find(fmt.Sprintf("#content > div.spage_intistore_body > div.spage_score_bottom > div:nth-child(%d) > div.ichart_score2_song > div.ichart_score2_song2 > span > a", i*4)).Text()
			ranks[i].PastRank = ranks[i].CurrentRank
			if len(doc.Find(fmt.Sprintf("#content > div.spage_intistore_body > div.spage_score_bottom > div:nth-child(%d) > div.ichart_score2_change.rank > .arrow1", i*4)).Nodes) > 0 {
				pastRankUncalculated, err := strconv.Atoi(doc.Find(fmt.Sprintf("#content > div.spage_intistore_body > div.spage_score_bottom > div:nth-child(%d) > div.ichart_score2_change.rank > .arrow1", i*4)).Parent().Text())
				helpers.Relax(err)
				ranks[i].PastRank = ranks[i].CurrentRank + pastRankUncalculated
			}
			if len(doc.Find(fmt.Sprintf("#content > div.spage_intistore_body > div.spage_score_bottom > div:nth-child(%d) > div.ichart_score2_change.rank > .arrow2", i*4)).Nodes) > 0 {
				pastRankUncalculated, err := strconv.Atoi(doc.Find(fmt.Sprintf("#content > div.spage_intistore_body > div.spage_score_bottom > div:nth-child(%d) > div.ichart_score2_change.rank > .arrow2", i*4)).Parent().Text())
				helpers.Relax(err)
				ranks[i].PastRank = ranks[i].CurrentRank - pastRankUncalculated
			}
			if len(doc.Find(fmt.Sprintf("#content > div.spage_intistore_body > div.spage_score_bottom > div:nth-child(%d) > div.ichart_score2_change.rank > .arrow4", i*4)).Nodes) > 0 {
				ranks[i].IsNew = true
			}
		}
	}

	return time, ranks
}

func (m *Charts) GetIChartWeekStats() (string, []GenericSongScore) {
	doc, err := goquery.NewDocument(ichartPageWeeklyCharts)
	helpers.Relax(err)

	time := strings.Trim(strings.Replace(doc.Find("#content > div.ichart_score_title > div.ichart_score_title_right.minitext3").Text(), "기준", "", -1), " ")
	var ranks []GenericSongScore
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
			ranks[i].Title = doc.Find(fmt.Sprintf("#content > div.spage_intistore_body > div.spage_score_bottom > div:nth-child(%d) > div.ichart_score2_song > div.ichart_score2_song1", i*4)).Text()
			ranks[i].Artist = doc.Find(fmt.Sprintf("#content > div.spage_intistore_body > div.spage_score_bottom > div:nth-child(%d) > div.ichart_score2_artist > div.ichart_score2_artist1", i*4)).Text()
			ranks[i].Album = doc.Find(fmt.Sprintf("#content > div.spage_intistore_body > div.spage_score_bottom > div:nth-child(%d) > div.ichart_score2_song > div.ichart_score2_song2 > span > a", i*4)).Text()
			ranks[i].PastRank = ranks[i].CurrentRank
			if len(doc.Find(fmt.Sprintf("#content > div.spage_intistore_body > div.spage_score_bottom > div:nth-child(%d) > div.ichart_score2_change.rank > .arrow1", i*4)).Nodes) > 0 {
				pastRankUncalculated, err := strconv.Atoi(doc.Find(fmt.Sprintf("#content > div.spage_intistore_body > div.spage_score_bottom > div:nth-child(%d) > div.ichart_score2_change.rank > .arrow1", i*4)).Parent().Text())
				helpers.Relax(err)
				ranks[i].PastRank = ranks[i].CurrentRank + pastRankUncalculated
			}
			if len(doc.Find(fmt.Sprintf("#content > div.spage_intistore_body > div.spage_score_bottom > div:nth-child(%d) > div.ichart_score2_change.rank > .arrow2", i*4)).Nodes) > 0 {
				pastRankUncalculated, err := strconv.Atoi(doc.Find(fmt.Sprintf("#content > div.spage_intistore_body > div.spage_score_bottom > div:nth-child(%d) > div.ichart_score2_change.rank > .arrow2", i*4)).Parent().Text())
				helpers.Relax(err)
				ranks[i].PastRank = ranks[i].CurrentRank - pastRankUncalculated
			}
			if len(doc.Find(fmt.Sprintf("#content > div.spage_intistore_body > div.spage_score_bottom > div:nth-child(%d) > div.ichart_score2_change.rank > .arrow4", i*4)).Nodes) > 0 {
				ranks[i].IsNew = true
			}
		}
	}

	return time, ranks
}
