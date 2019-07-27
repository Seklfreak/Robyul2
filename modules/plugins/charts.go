package plugins

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

type Charts struct{}

const (
	melonEndpointRealtimeCharts = "http://www.melon.com/chart/index.htm"
	melonEndpointDailyCharts    = "http://www.melon.com/chart/day/index.htm"
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
				time, realtimeStats := m.GetMelonRealtimeStats()
				chartsEmbed := &discordgo.MessageEmbed{
					Title:  helpers.GetTextF("plugins.charts.realtime-melon-embed-title", time),
					URL:    melonFriendlyRealtimeStats,
					Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.charts.melon-embed-footer")},
					Fields: []*discordgo.MessageEmbedField{},
					Color:  helpers.GetDiscordColorFromHex(helpers.GetText("plugins.charts.melon-embed-hex-color")),
				}
				for _, song := range realtimeStats {
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
				return
			case "daily":
				session.ChannelTyping(msg.ChannelID)
				time, dailyStats := m.GetMelonDailyStats()
				chartsEmbed := &discordgo.MessageEmbed{
					Title:  helpers.GetTextF("plugins.charts.daily-melon-embed-title", time),
					URL:    melonFriendlyDailyStats,
					Footer: &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.charts.melon-embed-footer")},
					Fields: []*discordgo.MessageEmbedField{},
					Color:  helpers.GetDiscordColorFromHex(helpers.GetText("plugins.charts.melon-embed-hex-color")),
				}
				for _, song := range dailyStats {
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

func (m *Charts) GetMelonRealtimeStats() (time string, ranks []GenericSongScore) {
	doc, err := goquery.NewDocument(melonEndpointRealtimeCharts)
	helpers.Relax(err)

	ranks = make([]GenericSongScore, 0)

	time = doc.Find(".calendar_prid > .yyyymmdd > .year").Text() + " " + doc.Find(".calendar_prid > .hhmm > .hour").Text()

	for i := 0; i < 10; i++ {
		currentRank := GenericSongScore{}
		node := goquery.NewDocumentFromNode(doc.Find(".lst50").Get(i))

		currentRank.Title = node.Find(".rank01 > span > a").Text()
		currentRank.Artist = node.Find(".rank02 > a").Text()
		currentRank.Album = node.Find(".rank03 > a").Text()
		currentRankN, _ := strconv.Atoi(node.Find(".rank").Text())
		currentRank.CurrentRank = currentRankN
		currentRank.PastRank = currentRank.CurrentRank

		rankingChangeDoc := goquery.NewDocumentFromNode(node.Find("td").Get(2))
		if len(rankingChangeDoc.Find(".up").Nodes) > 0 {
			rankUpChangeN, _ := strconv.Atoi(rankingChangeDoc.Find(".up").Text())
			currentRank.PastRank += rankUpChangeN
		}
		if len(rankingChangeDoc.Find(".down").Nodes) > 0 {
			rankDownChangeN, _ := strconv.Atoi(rankingChangeDoc.Find(".down").Text())
			currentRank.PastRank -= rankDownChangeN
		}

		ranks = append(ranks, currentRank)
	}

	return time, ranks
}

func (m *Charts) GetMelonDailyStats() (time string, ranks []GenericSongScore) {
	doc, err := goquery.NewDocument(melonEndpointDailyCharts)
	helpers.Relax(err)

	ranks = make([]GenericSongScore, 0)

	time = doc.Find(".calendar_prid > .yyyymmdd > .year").Text()

	for i := 0; i < 10; i++ {
		currentRank := GenericSongScore{}
		node := goquery.NewDocumentFromNode(doc.Find(".lst50").Get(i))

		currentRank.Title = node.Find(".rank01 > span > a").Text()
		currentRank.Artist = node.Find(".rank02 > a").Text()
		currentRank.Album = node.Find(".rank03 > a").Text()
		currentRankN, _ := strconv.Atoi(node.Find(".rank").Text())
		currentRank.CurrentRank = currentRankN
		currentRank.PastRank = currentRank.CurrentRank

		rankingChangeDoc := goquery.NewDocumentFromNode(node.Find("td").Get(2))
		if len(rankingChangeDoc.Find(".up").Nodes) > 0 {
			rankUpChangeN, _ := strconv.Atoi(rankingChangeDoc.Find(".up").Text())
			currentRank.PastRank += rankUpChangeN
		}
		if len(rankingChangeDoc.Find(".down").Nodes) > 0 {
			rankDownChangeN, _ := strconv.Atoi(rankingChangeDoc.Find(".down").Text())
			currentRank.PastRank -= rankDownChangeN
		}

		ranks = append(ranks, currentRank)
	}

	return time, ranks
}

func (m *Charts) DoMelonRequest(url string) []byte {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}

	request.Header.Set("User-Agent", helpers.DEFAULT_UA)
	request.Header.Set("Accept-Language", "en_US")
	request.Header.Set("appKey", helpers.GetConfig().Path("melon.app_key").Data().(string))

	response, err := helpers.DefaultClient.Do(request)
	helpers.Relax(err)

	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}

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
