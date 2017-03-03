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
)

type Charts struct{}

const (
	melonEndpointCharts        string = "http://apis.skplanetx.com/melon/charts/%s?version=1&page=1&count=10"
	melonFriendlyRealtimeStats string = "http://www.melon.com/chart/index.htm"
	melonFriendlyDailyStats    string = "http://www.melon.com/chart/day/index.htm"
)

func (m *Charts) Commands() []string {
	return []string{
		"melon",
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

func (m *Charts) Init(session *discordgo.Session) {

}

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

					// TODO: compare to rank before
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

					// TODO: compare to rank before
					chartsEmbed.Fields = append(chartsEmbed.Fields, &discordgo.MessageEmbedField{
						Name:  fmt.Sprintf("**#%s** %s", strconv.Itoa(song.CurrentRank), rankChange),
						Value: fmt.Sprintf("**%s** by **%s** (on %s)", song.SongName, artistText, song.AlbumName),
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
