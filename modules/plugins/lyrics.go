package plugins

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

type Lyrics struct{}

func (l *Lyrics) Commands() []string {
	return []string{
		"lyrics",
	}
}

const (
	GeniusApiBaseUrl = "https://api.genius.com"
	GeniusBaseUrl    = "https://genius.com"
)

type GeniusSearchResult struct {
	Meta struct {
		Status int `json:"status"`
	} `json:"meta"`
	Response struct {
		Hits []struct {
			Highlights []interface{} `json:"highlights"`
			Index      string        `json:"index"`
			Type       string        `json:"type"`
			Result     struct {
				AnnotationCount          int    `json:"annotation_count"`
				APIPath                  string `json:"api_path"`
				FullTitle                string `json:"full_title"`
				HeaderImageThumbnailURL  string `json:"header_image_thumbnail_url"`
				HeaderImageURL           string `json:"header_image_url"`
				ID                       int    `json:"id"`
				LyricsOwnerID            int    `json:"lyrics_owner_id"`
				LyricsState              string `json:"lyrics_state"`
				Path                     string `json:"path"`
				PyongsCount              int    `json:"pyongs_count"`
				SongArtImageThumbnailURL string `json:"song_art_image_thumbnail_url"`
				Stats                    struct {
					Hot                   bool `json:"hot"`
					UnreviewedAnnotations int  `json:"unreviewed_annotations"`
					Concurrents           int  `json:"concurrents"`
					Pageviews             int  `json:"pageviews"`
				} `json:"stats"`
				Title             string `json:"title"`
				TitleWithFeatured string `json:"title_with_featured"`
				URL               string `json:"url"`
				PrimaryArtist     struct {
					APIPath        string `json:"api_path"`
					HeaderImageURL string `json:"header_image_url"`
					ID             int    `json:"id"`
					ImageURL       string `json:"image_url"`
					IsMemeVerified bool   `json:"is_meme_verified"`
					IsVerified     bool   `json:"is_verified"`
					Name           string `json:"name"`
					URL            string `json:"url"`
				} `json:"primary_artist"`
			} `json:"result"`
		} `json:"hits"`
	} `json:"response"`
}

func (l *Lyrics) Init(session *discordgo.Session) {

}

func (l *Lyrics) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	session.ChannelTyping(msg.ChannelID)

	listSongs := false
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "list") {
		listSongs = true
		content = strings.TrimSpace(strings.TrimLeft(content, "list"))
	}

	encodedQuery, err := helpers.UrlEncode(content)
	helpers.Relax(err)

	var searchResult GeniusSearchResult
	err = l.GeniusRequest(fmt.Sprintf("/search?q=%s", encodedQuery), &searchResult)
	helpers.Relax(err)

	if searchResult.Meta.Status != 200 {
		_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.lyrics.genius-api-error"))
		helpers.Relax(err)
		return
	}

	if len(searchResult.Response.Hits) <= 0 {
		_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.lyrics.genius-no-results"))
		helpers.Relax(err)
		return
	}

	hitI := -1
	for i, hit := range searchResult.Response.Hits {
		if hit.Type == "song" {
			hitI = i
			break
		}
	}

	if hitI == -1 {
		_, err := helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.lyrics.genius-no-results"))
		helpers.Relax(err)
		return
	}

	if listSongs == true {
		songList := ""
		for i, hit := range searchResult.Response.Hits {
			if hit.Type == "song" {
				songList += fmt.Sprintf("[%s](%s)\n", hit.Result.FullTitle, hit.Result.URL)
			}
			if i >= 4 {
				break
			}
		}
		songListEmbed := &discordgo.MessageEmbed{
			Title:       helpers.GetTextF("plugins.lyrics.song-list-embed-title", content),
			Footer:      &discordgo.MessageEmbedFooter{Text: helpers.GetText("plugins.lyrics.powered-by")},
			Description: songList,
		}

		_, err := helpers.SendEmbed(msg.ChannelID, songListEmbed)
		helpers.Relax(err)
		return
	}

	hit := searchResult.Response.Hits[hitI]
	lyrics, err := l.GeniusScrapeLyrics(hit.Result.APIPath)
	helpers.Relax(err)

	result := fmt.Sprintf("__Lyrics for: **%s**__\n%s\n_%s_",
		hit.Result.FullTitle, lyrics, helpers.GetText("plugins.lyrics.powered-by"))

	for i, page := range helpers.Pagify(result, "\n") {
		if i >= 2 {
			_, err := helpers.SendMessage(msg.ChannelID, fmt.Sprintf("More on <%s>", hit.Result.URL))
			helpers.Relax(err)
			break
		}
		_, err := helpers.SendMessage(msg.ChannelID, page)
		helpers.Relax(err)
	}
}

func (l *Lyrics) GeniusRequest(location string, object interface{}) error {
	requestUrl := GeniusApiBaseUrl + location
	client := &http.Client{
		Timeout: time.Duration(10 * time.Second),
	}
	req, err := http.NewRequest("GET", requestUrl, nil)
	if err != nil {
		return err
	}
	req.Header.Add("User-Agent", helpers.DEFAULT_UA)
	req.Header.Add("Authorization", helpers.GetConfig().Path("genius.token").Data().(string))
	resp, err := client.Do(req)
	defer resp.Body.Close()
	if err != nil {
		return err
	}

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(bytes, object)
}

func (l *Lyrics) GeniusScrapeLyrics(songApiPath string) (string, error) {
	requestUrl := GeniusBaseUrl + songApiPath

	doc, err := goquery.NewDocument(requestUrl)
	if err != nil {
		return "", err
	}

	lyrics := ""
	doc.Find("div.lyrics").Children().Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if strings.Count(text, "\n") <= 1 {
			lyrics += "\n**" + text + "**\n"
		} else {
			lyrics += text + "\n"
		}
	})
	return lyrics, nil
}
