package plugins

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"bytes"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/metrics"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/Seklfreak/Robyul2/services/youtube"
	"github.com/Seklfreak/lastfm-go/lastfm"
	"github.com/bradfitz/slice"
	"github.com/bwmarrin/discordgo"
	humanize "github.com/dustin/go-humanize"
	"github.com/globalsign/mgo/bson"
)

type LastFm struct{}

const (
	lastfmHexColor           = "#d51007"
	lastfmFriendlyUser       = "https://www.last.fm/user/%s"
	lastfmYouTubeFriendlyUrl = "https://youtu.be/%s"
)

var (
	lastfmCachedStats        []LastFMAccountCachedStats
	lastfmCombinedGuildStats []LastFMCombinedGuildStats
)

type LastFMAccount_Safe_Entries struct {
	entries []models.LastFmEntry
	mux     sync.Mutex
}

type LastFMSongInfo struct {
	Name       string
	Url        string
	ArtistName string
	ArtistURL  string
	ImageURL   string
	Plays      int
	Users      int
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
		err := helpers.MDbIter(helpers.MdbCollection(models.LastFmTable).Find(nil)).All(&safeEntries.entries)
		helpers.Relax(err)

		// Get Stats from LastFM
		newLastfmCachedStats := make([]LastFMAccountCachedStats, 0)
		for _, safeAccount := range safeEntries.entries {
			safeEntries.mux.Lock()
			newLastFmCachedStat := new(LastFMAccountCachedStats)
			newLastFmCachedStat.UserID = safeAccount.UserID
			periods := []string{"overall", "7day", "1month", "3month", "6month", "12month"}

			for _, period := range periods {
				lastfmTopTracks, err := helpers.GetLastFmClient().User.GetTopTracks(lastfm.P{
					"limit":  50,
					"user":   safeAccount.LastFmUsername,
					"period": period,
				})
				metrics.LastFmRequests.Add(1)
				if err != nil {
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
							Users:      1,
						}
						switch period {
						case "overall":
							newLastFmCachedStat.Overall = append(newLastFmCachedStat.Overall, songInfo)
							break
						case "7day", "7days", "week":
							newLastFmCachedStat.SevenDay = append(newLastFmCachedStat.SevenDay, songInfo)
							break
						case "1month", "1months", "month":
							newLastFmCachedStat.OneMonth = append(newLastFmCachedStat.OneMonth, songInfo)
							break
						case "3month", "3months":
							newLastFmCachedStat.ThreeMonth = append(newLastFmCachedStat.ThreeMonth, songInfo)
							break
						case "6month", "6months":
							newLastFmCachedStat.SixMonth = append(newLastFmCachedStat.SixMonth, songInfo)
							break
						case "12month", "12months", "year":
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

			members := make([]*discordgo.Member, 0)
			for _, botGuild := range cache.GetSession().State.Guilds {
				if botGuild.ID == guild.ID {
					for _, member := range guild.Members {
						members = append(members, member)
					}
				}
			}

			if len(members) <= 0 {
				continue
			}
			for _, member := range members {
				for _, cachedStat := range lastfmCachedStats {
					if cachedStat.UserID == member.User.ID {
						// User is on Guild
						newCombinedGuildStat.NumberOfUsers += 1
						// Append tracks
						for _, track := range cachedStat.Overall {
							added := false
							for i, trackInDb := range newCombinedGuildStat.Overall {
								if strings.EqualFold(trackInDb.Name, track.Name) &&
									strings.EqualFold(trackInDb.ArtistName, track.ArtistName) {
									newCombinedGuildStat.Overall[i].Plays += track.Plays
									newCombinedGuildStat.Overall[i].Users += track.Users
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
								if strings.EqualFold(trackInDb.Name, track.Name) &&
									strings.EqualFold(trackInDb.ArtistName, track.ArtistName) {
									newCombinedGuildStat.SevenDay[i].Plays += track.Plays
									newCombinedGuildStat.SevenDay[i].Users += track.Users
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
								if strings.EqualFold(trackInDb.Name, track.Name) &&
									strings.EqualFold(trackInDb.ArtistName, track.ArtistName) {
									newCombinedGuildStat.OneMonth[i].Plays += track.Plays
									newCombinedGuildStat.OneMonth[i].Users += track.Users
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
								if strings.EqualFold(trackInDb.Name, track.Name) &&
									strings.EqualFold(trackInDb.ArtistName, track.ArtistName) {
									newCombinedGuildStat.ThreeMonth[i].Plays += track.Plays
									newCombinedGuildStat.ThreeMonth[i].Users += track.Users
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
								if strings.EqualFold(trackInDb.Name, track.Name) &&
									strings.EqualFold(trackInDb.ArtistName, track.ArtistName) {
									newCombinedGuildStat.SixMonth[i].Plays += track.Plays
									newCombinedGuildStat.SixMonth[i].Users += track.Users
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
								if strings.EqualFold(trackInDb.Name, track.Name) &&
									strings.EqualFold(trackInDb.ArtistName, track.ArtistName) {
									newCombinedGuildStat.TwelveMonth[i].Plays += track.Plays
									newCombinedGuildStat.TwelveMonth[i].Users += track.Users
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

		newCombinedGuildStats = nil
		safeEntries.entries = nil

		time.Sleep(6 * time.Hour)
	}
}

func (m *LastFm) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermLastFm) {
		return
	}

	args := strings.Fields(content)
	lastfmUsername := helpers.GetLastFmUsername(msg.Author.ID)
	subCom := ""
	if len(args) >= 1 {
		subCom = args[0]
	}
	if subCom != "" || lastfmUsername != "" {
		switch subCom {
		case "set", "register":
			if len(args) >= 2 {
				lastfmUsername = args[1]

				err := helpers.MDbUpsert(
					models.LastFmTable,
					bson.M{"userid": msg.Author.ID},
					models.LastFmEntry{
						UserID:         msg.Author.ID,
						LastFmUsername: lastfmUsername,
					},
				)
				helpers.Relax(err)

				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.lastfm.set-username-success", lastfmUsername))
			} else {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
				return
			}
		case "np", "nowplaying":
			var err error
			targetUser := msg.Author
			if len(args) >= 2 {
				lastfmUsername = args[1]
				targetUser, err = helpers.GetUserFromMention(lastfmUsername)
				if err == nil {
					lastfmUsername = helpers.GetLastFmUsername(targetUser.ID)
				}
			}

			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)

			if lastfmUsername == "" {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.lastfm.too-few", helpers.GetPrefixForServer(channel.GuildID)))
				return
			}
			session.ChannelTyping(msg.ChannelID)
			lastfmRecentTracks, err := helpers.GetLastFmClient().User.GetRecentTracksExtended(lastfm.P{
				"limit": 3,
				"user":  lastfmUsername,
			})
			metrics.LastFmRequests.Add(1)
			if err != nil {
				if e, ok := err.(*lastfm.LastfmError); ok {
					helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Error: `%s`", e.Message))
					return
				}
			}
			playcountText := ""
			lastfmAvatar := targetUser.AvatarURL("64")
			lastfmUserInfo, err := helpers.GetLastFmClient().User.GetInfo(lastfm.P{
				"user": lastfmUsername,
			})
			metrics.LastFmRequests.Add(1)
			helpers.RelaxLog(err)
			if err == nil {
				playcountText = " | Total plays: " + lastfmUserInfo.PlayCount
				playcountNumber, err := strconv.Atoi(lastfmUserInfo.PlayCount)
				if err == nil {
					playcountText = " | Total plays: " + humanize.Comma(int64(playcountNumber))
				}
				if len(lastfmUserInfo.Images) > 0 {
					for _, lastfmImage := range lastfmUserInfo.Images {
						if lastfmImage.Size == "large" {
							lastfmAvatar = lastfmImage.Url
						}
					}
				}
			}
			if lastfmRecentTracks.Total > 0 {
				lastTrack := lastfmRecentTracks.Tracks[0]
				lastTrackEmbedTitle := helpers.GetTextF("plugins.lastfm.lasttrack-embed-title-last", lastfmUsername)
				if lastTrack.NowPlaying == "true" {
					lastTrackEmbedTitle = helpers.GetTextF("plugins.lastfm.lasttrack-embed-title-np", lastfmUsername)
				}
				var heartText string
				if lastTrack.Loved == "1" {
					heartText = " :heart:"
				}
				lastTrackEmbed := &discordgo.MessageEmbed{
					Description: fmt.Sprintf(
						"[**%s** by **%s**](%s)%s",
						lastTrack.Name, lastTrack.Artist.Name,
						helpers.EscapeLinkForMarkdown(lastTrack.Url),
						heartText),
					Footer: &discordgo.MessageEmbedFooter{
						Text:    helpers.GetText("plugins.lastfm.embed-footer"),
						IconURL: helpers.GetText("plugins.lastfm.embed-footer-imageurl"),
					},
					Author: &discordgo.MessageEmbedAuthor{
						URL:     fmt.Sprintf(lastfmFriendlyUser, lastfmUsername),
						Name:    lastTrackEmbedTitle,
						IconURL: lastfmAvatar,
					},
					Fields: []*discordgo.MessageEmbedField{},
					Color:  helpers.GetDiscordColorFromHex(lastfmHexColor),
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
				if lastTrack.NowPlaying == "true" && lastfmRecentTracks.Total > 1 {
					beforeTrack := lastfmRecentTracks.Tracks[1]
					if beforeTrack.Url == lastTrack.Url && lastfmRecentTracks.Total > 2 {
						beforeTrack = lastfmRecentTracks.Tracks[2]
					}
					var heartText string
					if beforeTrack.Loved == "1" {
						heartText = " :heart:"
					}
					lastTrackEmbed.Fields = append(lastTrackEmbed.Fields, &discordgo.MessageEmbedField{
						Name: "Listened to before",
						Value: fmt.Sprintf("[**%s** by **%s**](%s)%s",
							beforeTrack.Name, beforeTrack.Artist.Name, helpers.EscapeLinkForMarkdown(beforeTrack.Url),
							heartText),
						Inline: false,
					})
				}
				if youtube.HasYouTubeService() {
					searchResult, err := youtube.GetYouTubeService().SearchQuerySingle(
						[]string{lastTrack.Artist.Name, lastTrack.Name}, "video")
					helpers.RelaxLog(err)
					if err == nil && searchResult != nil && searchResult.Snippet != nil {
						lastTrackEmbed.Description += "\navailable on [YouTube](" + fmt.Sprintf(lastfmYouTubeFriendlyUrl, searchResult.Id.VideoId) + ")"
						lastTrackEmbed.Footer.Text += " and YouTube"
					}
				}
				lastTrackEmbed.Footer.Text += playcountText
				_, err = helpers.SendEmbed(msg.ChannelID, lastTrackEmbed)
				helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
			} else {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.lastfm.no-recent-tracks"))
				return
			}
		case "yt", "youtube":
			if !youtube.HasYouTubeService() {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("lastfm.no-youtube"))
				return
			}
			if len(args) >= 2 {
				lastfmUsername = args[1]
				targetUser, err := helpers.GetUserFromMention(lastfmUsername)
				if err == nil {
					lastfmUsername = helpers.GetLastFmUsername(targetUser.ID)
				}
			}

			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)

			if lastfmUsername == "" {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.lastfm.too-few", helpers.GetPrefixForServer(channel.GuildID)))
				return
			}
			session.ChannelTyping(msg.ChannelID)
			lastfmRecentTracks, err := helpers.GetLastFmClient().User.GetRecentTracks(lastfm.P{
				"limit": 2,
				"user":  lastfmUsername,
			})
			metrics.LastFmRequests.Add(1)
			if err != nil {
				if e, ok := err.(*lastfm.LastfmError); ok {
					helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Error: `%s`", e.Message))
					return
				}
			}
			if lastfmRecentTracks.Total > 0 {
				lastTrack := lastfmRecentTracks.Tracks[0]
				searchResult, err := youtube.GetYouTubeService().SearchQuerySingle(
					[]string{lastTrack.Artist.Name, lastTrack.Name}, "video")
				helpers.RelaxLog(err)
				if err != nil || searchResult == nil || searchResult.Snippet == nil {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("lastfm.no-youtube"))
					return
				}
				messageContent := "**" + searchResult.Snippet.Title + "** on " + searchResult.Snippet.ChannelTitle + "\n"
				messageContent += fmt.Sprintf(lastfmYouTubeFriendlyUrl, searchResult.Id.VideoId)
				_, err = helpers.SendMessage(msg.ChannelID, messageContent)
				helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
			} else {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.lastfm.no-recent-tracks"))
				return
			}
		case "topalbums", "topalbum", "tal":
			timeLookup := "overall"
			timeString := "all time"
			var collage bool
			if len(args) >= 2 {
				if args[len(args)-1] == "collage" {
					collage = true
					args = append(args[:len(args)-1], args[len(args):]...) // remove last element from slice
				}
				switch args[len(args)-1] {
				case "7days", "week", "7day":
					timeString = "the last seven days"
					timeLookup = "7day"
					args = append(args[:len(args)-1], args[len(args):]...) // remove last element from slice
					break
				case "1month", "month", "1months":
					timeString = "the last month"
					timeLookup = "1month"
					args = append(args[:len(args)-1], args[len(args):]...) // remove last element from slice
					break
				case "3month", "3months":
					timeString = "the last three months"
					timeLookup = "3month"
					args = append(args[:len(args)-1], args[len(args):]...) // remove last element from slice
					break
				case "6month", "6months":
					timeString = "the last six months"
					timeLookup = "6month"
					args = append(args[:len(args)-1], args[len(args):]...) // remove last element from slice
					break
				case "12month", "year", "12months":
					timeString = "the last twelve months"
					timeLookup = "12month"
					args = append(args[:len(args)-1], args[len(args):]...) // remove last element from slice
					break
				}
			}

			if len(args) >= 2 {
				lastfmUsername = args[1]
				targetUser, err := helpers.GetUserFromMention(lastfmUsername)
				if err == nil {
					lastfmUsername = helpers.GetLastFmUsername(targetUser.ID)
				}
			}

			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)

			if lastfmUsername == "" {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.lastfm.too-few", helpers.GetPrefixForServer(channel.GuildID)))
				return
			}
			session.ChannelTyping(msg.ChannelID)
			lastfmTopAlbums, err := helpers.GetLastFmClient().User.GetTopAlbums(lastfm.P{
				"limit":  10,
				"period": timeLookup,
				"user":   lastfmUsername,
			})
			metrics.LastFmRequests.Add(1)
			if err != nil {
				if e, ok := err.(*lastfm.LastfmError); ok {
					helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Error: `%s`", e.Message))
					return
				}
			}
			lastfmUser, err := helpers.GetLastFmClient().User.GetInfo(lastfm.P{
				"user": lastfmUsername,
			})
			metrics.LastFmRequests.Add(1)
			helpers.Relax(err)
			if lastfmTopAlbums.Total > 0 {
				if collage {
					description := ""
					var n int
					imageUrls := make([]string, 0)
					for _, topAlbum := range lastfmTopAlbums.Albums {
						if len(topAlbum.Images) > 0 {
							for _, topAlbumImage := range topAlbum.Images {
								if topAlbumImage.Size == "extralarge" {
									n++
									imageUrls = append(imageUrls, topAlbumImage.Url)
									description += strconv.Itoa(n) + ". "
									description += "[" + topAlbum.Name + " by " + topAlbum.Artist.Name + "](" + helpers.EscapeLinkForMarkdown(topAlbum.Url) + ") "
								}
							}
							if len(imageUrls) >= 9 {
								break
							}
						}
					}

					collageBytes := lastfmCollage(imageUrls, nil)

					topAlbumsEmbed := &discordgo.MessageEmbed{
						Description: description,
						Footer: &discordgo.MessageEmbedFooter{
							Text:    helpers.GetText("plugins.lastfm.embed-footer"),
							IconURL: helpers.GetText("plugins.lastfm.embed-footer-imageurl"),
						},
						Color: helpers.GetDiscordColorFromHex(lastfmHexColor),
						Author: &discordgo.MessageEmbedAuthor{
							Name: helpers.GetTextF("plugins.lastfm.topalbums-embed-title", lastfmUsername) + " of " + timeString,
							URL:  fmt.Sprintf(lastfmFriendlyUser, lastfmTopAlbums.User),
						},
						Image: &discordgo.MessageEmbedImage{
							URL: "attachment://Robyul-LastFM-Collage.png",
						},
					}
					if len(lastfmUser.Images) > 0 {
						for _, image := range lastfmUser.Images {
							if image.Size == "large" {
								topAlbumsEmbed.Author.IconURL = image.Url
							}
						}
					}
					helpers.SendComplex(msg.ChannelID, &discordgo.MessageSend{
						Embed: topAlbumsEmbed,
						Files: []*discordgo.File{{
							Name:   "Robyul-LastFM-Collage.png",
							Reader: bytes.NewReader(collageBytes),
						}},
					})
					return
				}

				topAlbumsEmbed := &discordgo.MessageEmbed{
					Description: "of **" + timeString + "**",
					Footer: &discordgo.MessageEmbedFooter{
						Text:    helpers.GetText("plugins.lastfm.embed-footer"),
						IconURL: helpers.GetText("plugins.lastfm.embed-footer-imageurl"),
					},
					Fields: []*discordgo.MessageEmbedField{},
					Color:  helpers.GetDiscordColorFromHex(lastfmHexColor),
					Author: &discordgo.MessageEmbedAuthor{
						Name: helpers.GetTextF("plugins.lastfm.topalbums-embed-title", lastfmUsername),
						URL:  fmt.Sprintf(lastfmFriendlyUser, lastfmTopAlbums.User),
					},
				}
				for _, topAlbum := range lastfmTopAlbums.Albums {
					topAlbumsEmbed.Fields = append(topAlbumsEmbed.Fields, &discordgo.MessageEmbedField{
						Name: fmt.Sprintf("**#%s** (%s plays)", topAlbum.Rank, topAlbum.PlayCount),
						Value: fmt.Sprintf("[**%s** by **%s**](%s)",
							topAlbum.Name, topAlbum.Artist.Name, helpers.EscapeLinkForMarkdown(topAlbum.Url)),
						Inline: false})
				}
				if len(lastfmUser.Images) > 0 {
					for _, image := range lastfmUser.Images {
						if image.Size == "large" {
							topAlbumsEmbed.Author.IconURL = image.Url
						}
					}
				}
				_, err = helpers.SendEmbed(msg.ChannelID, topAlbumsEmbed)
				helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
			} else {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.lastfm.no-recent-tracks"))
				return
			}
		case "topartists", "topartist", "top", "ta":
			timeLookup := "overall"
			timeString := "all time"
			var collage bool
			if len(args) >= 2 {
				if args[len(args)-1] == "collage" {
					collage = true
					args = append(args[:len(args)-1], args[len(args):]...) // remove last element from slice
				}
				switch args[len(args)-1] {
				case "7days", "week", "7day":
					timeString = "the last seven days"
					timeLookup = "7day"
					args = append(args[:len(args)-1], args[len(args):]...) // remove last element from slice
					break
				case "1month", "month", "1months":
					timeString = "the last month"
					timeLookup = "1month"
					args = append(args[:len(args)-1], args[len(args):]...) // remove last element from slice
					break
				case "3month", "3months":
					timeString = "the last three months"
					timeLookup = "3month"
					args = append(args[:len(args)-1], args[len(args):]...) // remove last element from slice
					break
				case "6month", "6months":
					timeString = "the last six months"
					timeLookup = "6month"
					args = append(args[:len(args)-1], args[len(args):]...) // remove last element from slice
					break
				case "12month", "year", "12months":
					timeString = "the last twelve months"
					timeLookup = "12month"
					args = append(args[:len(args)-1], args[len(args):]...) // remove last element from slice
					break
				}
			}

			if len(args) >= 2 {
				lastfmUsername = args[1]
				targetUser, err := helpers.GetUserFromMention(lastfmUsername)
				if err == nil {
					lastfmUsername = helpers.GetLastFmUsername(targetUser.ID)
				}
			}

			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)

			if lastfmUsername == "" {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.lastfm.too-few", helpers.GetPrefixForServer(channel.GuildID)))
				return
			}
			session.ChannelTyping(msg.ChannelID)

			lastfmTopArtists, err := helpers.GetLastFmClient().User.GetTopArtists(lastfm.P{
				"limit":  10,
				"period": timeLookup,
				"user":   lastfmUsername,
			})
			metrics.LastFmRequests.Add(1)
			if err != nil {
				if e, ok := err.(*lastfm.LastfmError); ok {
					helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Error: `%s`", e.Message))
					return
				}
			}
			lastfmUser, err := helpers.GetLastFmClient().User.GetInfo(lastfm.P{
				"user": lastfmUsername,
			})
			metrics.LastFmRequests.Add(1)
			helpers.Relax(err)
			if lastfmTopArtists.Total > 0 {
				if collage {
					imageUrls := make([]string, 0)
					artistNames := make([]string, 0)
					for _, topArtist := range lastfmTopArtists.Artists {
						if len(topArtist.Images) > 0 {
							for _, topArtistImage := range topArtist.Images {
								if topArtistImage.Size == "extralarge" {
									imageUrls = append(imageUrls, topArtistImage.Url)
									artistNames = append(artistNames, topArtist.Name)
								}
							}
							if len(imageUrls) >= 9 {
								break
							}
						}
					}

					collageBytes := lastfmCollage(imageUrls, artistNames)

					topArtistsEmbed := &discordgo.MessageEmbed{
						Footer: &discordgo.MessageEmbedFooter{
							Text:    helpers.GetText("plugins.lastfm.embed-footer"),
							IconURL: helpers.GetText("plugins.lastfm.embed-footer-imageurl"),
						},
						Fields: []*discordgo.MessageEmbedField{},
						Color:  helpers.GetDiscordColorFromHex(lastfmHexColor),
						Author: &discordgo.MessageEmbedAuthor{
							Name: helpers.GetTextF("plugins.lastfm.topartists-embed-title", lastfmUsername) + " of " + timeString,
							URL:  fmt.Sprintf(lastfmFriendlyUser, lastfmTopArtists.User),
						},
						Image: &discordgo.MessageEmbedImage{
							URL: "attachment://Robyul-LastFM-Collage.png",
						},
					}
					if len(lastfmUser.Images) > 0 {
						for _, image := range lastfmUser.Images {
							if image.Size == "large" {
								topArtistsEmbed.Author.IconURL = image.Url
							}
						}
					}

					helpers.SendComplex(msg.ChannelID, &discordgo.MessageSend{
						Embed: topArtistsEmbed,
						Files: []*discordgo.File{{
							Name:   "Robyul-LastFM-Collage.png",
							Reader: bytes.NewReader(collageBytes),
						}},
					})
					return
				}

				topArtistsEmbed := &discordgo.MessageEmbed{
					Description: "of **" + timeString + "**",
					Footer: &discordgo.MessageEmbedFooter{
						Text:    helpers.GetText("plugins.lastfm.embed-footer"),
						IconURL: helpers.GetText("plugins.lastfm.embed-footer-imageurl"),
					},
					Fields: []*discordgo.MessageEmbedField{},
					Color:  helpers.GetDiscordColorFromHex(lastfmHexColor),
					Author: &discordgo.MessageEmbedAuthor{
						Name: helpers.GetTextF("plugins.lastfm.topartists-embed-title", lastfmUsername),
						URL:  fmt.Sprintf(lastfmFriendlyUser, lastfmTopArtists.User),
					},
				}
				for _, topArtist := range lastfmTopArtists.Artists {
					topArtistsEmbed.Fields = append(topArtistsEmbed.Fields, &discordgo.MessageEmbedField{
						Name: fmt.Sprintf("**#%s** (%s plays)", topArtist.Rank, topArtist.PlayCount),
						Value: fmt.Sprintf("[**%s**](%s)",
							topArtist.Name, helpers.EscapeLinkForMarkdown(topArtist.Url)),
						Inline: false})
				}
				if len(lastfmUser.Images) > 0 {
					for _, image := range lastfmUser.Images {
						if image.Size == "large" {
							topArtistsEmbed.Author.IconURL = image.Url
						}
					}
				}
				_, err = helpers.SendEmbed(msg.ChannelID, topArtistsEmbed)
				helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
			} else {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.lastfm.no-recent-tracks"))
				return
			}
		case "toptracks", "topsongs", "toptrack", "topsong", "tt", "ts":
			timeLookup := "overall"
			timeString := "all time"
			var collage bool
			if len(args) >= 2 {
				if args[len(args)-1] == "collage" {
					collage = true
					args = append(args[:len(args)-1], args[len(args):]...) // remove last element from slice
				}
				switch args[len(args)-1] {
				case "7days", "week", "7day":
					timeString = "the last seven days"
					timeLookup = "7day"
					args = append(args[:len(args)-1], args[len(args):]...) // remove last element from slice
					break
				case "1month", "month", "1months":
					timeString = "the last month"
					timeLookup = "1month"
					args = append(args[:len(args)-1], args[len(args):]...) // remove last element from slice
					break
				case "3month", "3months":
					timeString = "the last three months"
					timeLookup = "3month"
					args = append(args[:len(args)-1], args[len(args):]...) // remove last element from slice
					break
				case "6month", "6months":
					timeString = "the last six months"
					timeLookup = "6month"
					args = append(args[:len(args)-1], args[len(args):]...) // remove last element from slice
					break
				case "12month", "year", "12months":
					timeString = "the last twelve months"
					timeLookup = "12month"
					args = append(args[:len(args)-1], args[len(args):]...) // remove last element from slice
					break
				}
			}

			if len(args) >= 2 {
				lastfmUsername = args[1]
				targetUser, err := helpers.GetUserFromMention(lastfmUsername)
				if err == nil {
					lastfmUsername = helpers.GetLastFmUsername(targetUser.ID)
				}
			}

			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)

			if lastfmUsername == "" {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.lastfm.too-few", helpers.GetPrefixForServer(channel.GuildID)))
				return
			}
			session.ChannelTyping(msg.ChannelID)

			lastfmTopTracks, err := helpers.GetLastFmClient().User.GetTopTracks(lastfm.P{
				"limit":  10,
				"period": timeLookup,
				"user":   lastfmUsername,
			})
			metrics.LastFmRequests.Add(1)
			if err != nil {
				if e, ok := err.(*lastfm.LastfmError); ok {
					helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Error: `%s`", e.Message))
					return
				}
			}
			lastfmUser, err := helpers.GetLastFmClient().User.GetInfo(lastfm.P{
				"user": lastfmUsername,
			})
			metrics.LastFmRequests.Add(1)
			helpers.Relax(err)
			if lastfmTopTracks.Total > 0 {
				if collage {
					trackNames := make([]string, 0)
					imageUrls := make([]string, 0)
					for _, topTrack := range lastfmTopTracks.Tracks {
						if len(topTrack.Images) > 0 {
							for _, topTrackImage := range topTrack.Images {
								if topTrackImage.Size == "extralarge" {
									imageUrls = append(imageUrls, topTrackImage.Url)
									trackNames = append(trackNames, topTrack.Name+"\n"+topTrack.Artist.Name)
								}
							}
							if len(imageUrls) >= 9 {
								break
							}
						}
					}

					collageBytes := lastfmCollage(imageUrls, trackNames)

					topTracksEmbed := &discordgo.MessageEmbed{
						Footer: &discordgo.MessageEmbedFooter{
							Text:    helpers.GetText("plugins.lastfm.embed-footer"),
							IconURL: helpers.GetText("plugins.lastfm.embed-footer-imageurl"),
						},
						Color: helpers.GetDiscordColorFromHex(lastfmHexColor),
						Author: &discordgo.MessageEmbedAuthor{
							Name: helpers.GetTextF("plugins.lastfm.toptracks-embed-title", lastfmUsername) + " of " + timeString,
							URL:  fmt.Sprintf(lastfmFriendlyUser, lastfmTopTracks.User),
						},
						Image: &discordgo.MessageEmbedImage{
							URL: "attachment://Robyul-LastFM-Collage.png",
						},
					}
					if len(lastfmUser.Images) > 0 {
						for _, image := range lastfmUser.Images {
							if image.Size == "large" {
								topTracksEmbed.Author.IconURL = image.Url
							}
						}
					}

					helpers.SendComplex(msg.ChannelID, &discordgo.MessageSend{
						Embed: topTracksEmbed,
						Files: []*discordgo.File{{
							Name:   "Robyul-LastFM-Collage.png",
							Reader: bytes.NewReader(collageBytes),
						}},
					})
					return
				}

				topTracksEmbed := &discordgo.MessageEmbed{
					Description: "of **" + timeString + "**",
					Footer: &discordgo.MessageEmbedFooter{
						Text:    helpers.GetText("plugins.lastfm.embed-footer"),
						IconURL: helpers.GetText("plugins.lastfm.embed-footer-imageurl"),
					},
					Fields: []*discordgo.MessageEmbedField{},
					Color:  helpers.GetDiscordColorFromHex(lastfmHexColor),
					Author: &discordgo.MessageEmbedAuthor{
						Name: helpers.GetTextF("plugins.lastfm.toptracks-embed-title", lastfmUsername),
						URL:  fmt.Sprintf(lastfmFriendlyUser, lastfmTopTracks.User),
					},
				}
				for _, topTrack := range lastfmTopTracks.Tracks {
					topTracksEmbed.Fields = append(topTracksEmbed.Fields, &discordgo.MessageEmbedField{
						Name: fmt.Sprintf("**#%s** (%s plays)", topTrack.Rank, topTrack.PlayCount),
						Value: fmt.Sprintf("[**%s** by **%s**](%s)",
							topTrack.Name, topTrack.Artist.Name, helpers.EscapeLinkForMarkdown(topTrack.Url)),
						Inline: false})
				}
				if len(lastfmUser.Images) > 0 {
					for _, image := range lastfmUser.Images {
						if image.Size == "large" {
							topTracksEmbed.Author.IconURL = image.Url
						}
					}
				}
				_, err = helpers.SendEmbed(msg.ChannelID, topTracksEmbed)
				helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
			} else {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.lastfm.no-recent-tracks"))
				return
			}
		case "discord-top", "server-top", "servertop", "discordtop":
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
				helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.lastfm.no-stats-available"))
				return
			}

			var topTracks []LastFMSongInfo
			timeString := "all time"
			topTracks = combinedStats.Overall
			if len(args) >= 2 {
				switch args[1] {
				case "overall":
					timeString = "all time"
					topTracks = combinedStats.Overall
					break
				case "7days", "week", "7day":
					timeString = "the last seven days"
					topTracks = combinedStats.SevenDay
					break
				case "1month", "month", "1months":
					timeString = "the last month"
					topTracks = combinedStats.OneMonth
					break
				case "3month", "3months":
					timeString = "the last three months"
					topTracks = combinedStats.ThreeMonth
					break
				case "6month", "6months":
					timeString = "the last six months"
					topTracks = combinedStats.SixMonth
					break
				case "12month", "year", "12months":
					timeString = "the last twelve months"
					topTracks = combinedStats.TwelveMonth
					break
				}
			}

			topTracksEmbed := &discordgo.MessageEmbed{
				Title:       helpers.GetTextF("plugins.lastfm.toptracks-embed-title", fmt.Sprintf("%s Server", guild.Name)),
				Description: fmt.Sprintf("of **%s**", timeString),
				Footer: &discordgo.MessageEmbedFooter{
					Text: fmt.Sprintf(
						"%s | %d last.fm users on this server",
						helpers.GetText("plugins.lastfm.embed-footer"),
						combinedStats.NumberOfUsers),
					IconURL: helpers.GetText("plugins.lastfm.embed-footer-imageurl"),
				},
				Fields: []*discordgo.MessageEmbedField{},
				Color:  helpers.GetDiscordColorFromHex(lastfmHexColor),
			}
			for i, topTrack := range topTracks {
				topTracksEmbed.Fields = append(topTracksEmbed.Fields, &discordgo.MessageEmbedField{
					Name: fmt.Sprintf("**#%s** (%s plays by %s users)",
						strconv.Itoa(i+1),
						humanize.Comma(int64(topTrack.Plays)), humanize.Comma(int64(topTrack.Users))),
					Value: fmt.Sprintf("[**%s** by **%s**](%s)",
						topTrack.Name, topTrack.ArtistName, helpers.EscapeLinkForMarkdown(topTrack.Url)),
					Inline: false})
				if i == 9 {
					break
				}
			}
			if guild.Icon != "" {
				topTracksEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{
					URL: discordgo.EndpointGuildIcon(guild.ID, guild.Icon),
				}
			}
			_, err = helpers.SendEmbed(msg.ChannelID, topTracksEmbed)
			helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
			break
		case "recent", "recents", "recently":
			var err error
			targetUser := msg.Author
			if len(args) >= 2 {
				lastfmUsername = args[1]
				targetUser, err = helpers.GetUserFromMention(lastfmUsername)
				if err == nil {
					lastfmUsername = helpers.GetLastFmUsername(targetUser.ID)
				}
			}

			channel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)

			if lastfmUsername == "" {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.lastfm.too-few", helpers.GetPrefixForServer(channel.GuildID)))
				return
			}
			session.ChannelTyping(msg.ChannelID)

			lastfmRecentTracks, err := helpers.GetLastFmClient().User.GetRecentTracksExtended(lastfm.P{
				"limit": 11,
				"user":  lastfmUsername,
			})
			metrics.LastFmRequests.Add(1)
			if err != nil {
				if e, ok := err.(*lastfm.LastfmError); ok {
					helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Error: `%s`", e.Message))
					return
				}
			}

			if len(lastfmRecentTracks.Tracks) <= 0 {
				helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.lastfm.no-recent-tracks"))
				return
			}

			playcountText := ""
			lastfmAvatar := targetUser.AvatarURL("64")
			lastfmUserInfo, err := helpers.GetLastFmClient().User.GetInfo(lastfm.P{
				"user": lastfmUsername,
			})
			metrics.LastFmRequests.Add(1)
			helpers.RelaxLog(err)
			if err == nil {
				playcountText = " | Total plays: " + lastfmUserInfo.PlayCount
				playcountNumber, err := strconv.Atoi(lastfmUserInfo.PlayCount)
				if err == nil {
					playcountText = " | Total plays: " + humanize.Comma(int64(playcountNumber))
				}
				if len(lastfmUserInfo.Images) > 0 {
					for _, lastfmImage := range lastfmUserInfo.Images {
						if lastfmImage.Size == "large" {
							lastfmAvatar = lastfmImage.Url
						}
					}
				}
			}

			recentsEmbed := &discordgo.MessageEmbed{
				Footer: &discordgo.MessageEmbedFooter{
					Text:    helpers.GetText("plugins.lastfm.embed-footer") + playcountText,
					IconURL: helpers.GetText("plugins.lastfm.embed-footer-imageurl"),
				},
				Author: &discordgo.MessageEmbedAuthor{
					URL:     fmt.Sprintf(lastfmFriendlyUser, lastfmUsername),
					Name:    helpers.GetTextF("plugins.lastfm.recents-embed-title", lastfmUsername),
					IconURL: lastfmAvatar,
				},
				//Fields: []*discordgo.MessageEmbedField{},
				Color: helpers.GetDiscordColorFromHex(lastfmHexColor),
			}

			var currentTrackUrl string
			var added int

			for _, recentTrack := range lastfmRecentTracks.Tracks {
				if added <= 1 && recentTrack.Url == currentTrackUrl {
					continue
				}
				trackText := fmt.Sprintf("**[%s](%s) by [%s](%s)**",
					recentTrack.Name, helpers.EscapeLinkForMarkdown(recentTrack.Url),
					recentTrack.Artist.Name, helpers.EscapeLinkForMarkdown(recentTrack.Artist.Url))
				if recentTrack.Loved == "1" {
					trackText += " :heart:"
				}
				if recentTrack.NowPlaying == "true" {
					trackText += " - _now playing_"
					currentTrackUrl = recentTrack.Url
				} else {
					timestamp, err := strconv.Atoi(recentTrack.Date.Uts)
					if err == nil {
						trackText += " - " + humanize.Time(time.Unix(int64(timestamp), 0))
					}
				}
				recentsEmbed.Description += trackText + "\n"
				added++
				if added >= 10 {
					break
				}
			}

			_, err = helpers.SendEmbed(msg.ChannelID, recentsEmbed)
			helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
			return
		default:
			var err error
			targetUser := msg.Author
			if subCom != "" {
				lastfmUsername = subCom
				targetUser, err = helpers.GetUserFromMention(lastfmUsername)
				if err == nil {
					lastfmUsername = helpers.GetLastFmUsername(targetUser.ID)
				}
			}
			session.ChannelTyping(msg.ChannelID)
			lastfmUser, err := helpers.GetLastFmClient().User.GetInfo(lastfm.P{"user": lastfmUsername})
			if err != nil {
				if e, ok := err.(*lastfm.LastfmError); ok {
					helpers.SendMessage(msg.ChannelID, fmt.Sprintf("Error: `%s`", e.Message))
					return
				}
			}
			metrics.LastFmRequests.Add(1)
			scrobblesCount := 0
			if lastfmUser.PlayCount != "" {
				scrobblesCount, err = strconv.Atoi(lastfmUser.PlayCount)
				helpers.Relax(err)
			}
			embedTitle := helpers.GetTextF("plugins.lastfm.profile-embed-title", lastfmUser.Name)
			if lastfmUser.RealName != "" {
				embedTitle = helpers.GetTextF("plugins.lastfm.profile-embed-title-realname", lastfmUser.RealName, lastfmUser.Name)
			}
			accountEmbed := &discordgo.MessageEmbed{
				Footer: &discordgo.MessageEmbedFooter{
					Text:    helpers.GetText("plugins.lastfm.embed-footer"),
					IconURL: helpers.GetText("plugins.lastfm.embed-footer-imageurl"),
				},
				Fields: []*discordgo.MessageEmbedField{
					{Name: "Scrobbles", Value: humanize.Comma(int64(scrobblesCount)), Inline: true}},
				Color: helpers.GetDiscordColorFromHex(lastfmHexColor),
				Author: &discordgo.MessageEmbedAuthor{
					URL:  fmt.Sprintf(lastfmFriendlyUser, lastfmUsername),
					Name: embedTitle,
				},
			}
			if len(lastfmUser.Images) > 0 {
				for _, image := range lastfmUser.Images {
					if image.Size == "large" {
						accountEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: image.Url}
						accountEmbed.Author.IconURL = image.Url
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
			_, err = helpers.SendEmbed(msg.ChannelID, accountEmbed)
			helpers.RelaxEmbed(err, msg.ChannelID, msg.ID)
		}
	} else {
		helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
		return
	}

}

func lastfmCollage(imageUrls, titles []string) []byte {
	for i, _ := range imageUrls {
		if !strings.HasSuffix(imageUrls[i], ".jpg") {
			continue
		}

		imageUrls[i] = imageUrls[i][:len(imageUrls[i])-4] + ".png"
	}

	return helpers.CollageFromUrls(
		imageUrls,
		titles,
		900, 900,
		300, 300,
		helpers.DISCORD_DARK_THEME_BACKGROUND_HEX,
	)
}
