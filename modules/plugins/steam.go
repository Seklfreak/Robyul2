package plugins

import (
	"strings"

	"encoding/json"

	"errors"

	"time"

	"fmt"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/shardmanager"
	"github.com/bwmarrin/discordgo"
	humanize "github.com/dustin/go-humanize"
	"github.com/sirupsen/logrus"
)

type steamAction func(args []string, in *discordgo.Message, out **discordgo.MessageSend) (next steamAction)

type Steam struct {
	apiKey string
}

const steamColour = "#000001"

func (m *Steam) Commands() []string {
	return []string{
		"steam",
	}
}

func (m *Steam) Init(session *shardmanager.Manager) {
	m.apiKey = helpers.GetConfig().Path("steam.api_key").Data().(string)
}

func (m *Steam) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermSteam) {
		return
	}

	session.ChannelTyping(msg.ChannelID)

	var result *discordgo.MessageSend
	args := strings.Fields(content)

	action := m.actionStart
	for action != nil {
		action = action(args, msg, &result)
	}
}

func (m *Steam) actionStart(args []string, in *discordgo.Message, out **discordgo.MessageSend) steamAction {
	cache.GetSession().SessionForGuildS(in.GuildID).ChannelTyping(in.ChannelID)

	return m.actionInfo
}

// [p]steam <user id or username>
func (m *Steam) actionInfo(args []string, in *discordgo.Message, out **discordgo.MessageSend) steamAction {
	if len(args) < 1 {
		*out = m.newMsg("bot.arguments.too-few")
		return m.actionFinish
	}

	steamID, err := m.getSteamID(args[0])
	helpers.Relax(err)

	steamUser, err := m.getSteamProfileSummary(steamID)
	if err != nil {
		if strings.Contains(err.Error(), "steam profile not found") {
			*out = m.newMsg("plugins.steam.user-not-found")
			return m.actionFinish
		}
	}
	helpers.Relax(err)

	createdAt := time.Unix(steamUser.Timecreated, 0)
	lastLogOff := time.Unix(steamUser.Lastlogoff, 0)

	statusText := strings.Title(m.steamStatusToText(steamUser.Personastate))
	if steamUser.Gameextrainfo != "" {
		statusText += "\n(Playing " + steamUser.Gameextrainfo + ")"
	}

	steamUserEmbed := &discordgo.MessageEmbed{
		URL:   steamUser.Profileurl,
		Title: steamUser.Personaname + " on Steam",
		Color: helpers.GetDiscordColorFromHex(steamColour),
		Footer: &discordgo.MessageEmbedFooter{
			Text:    "Steam #" + steamUser.Steamid + " | " + helpers.GetText("plugins.steam.embed-footer"),
			IconURL: helpers.GetText("plugins.steam.embed-footer-imageurl"),
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: steamUser.Avatarfull,
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Joined Steam On",
				Value:  fmt.Sprintf("%s (%s)", createdAt.Format(time.ANSIC), humanize.Time(createdAt)),
				Inline: false,
			},
			{
				Name:   "Last Online",
				Value:  fmt.Sprintf("%s (%s)", lastLogOff.Format(time.ANSIC), humanize.Time(lastLogOff)),
				Inline: false,
			},
			{
				Name:   "Status",
				Value:  statusText,
				Inline: false,
			},
		},
	}

	*out = &discordgo.MessageSend{
		Embed: steamUserEmbed,
	}
	return m.actionFinish
}

func (m *Steam) steamStatusToText(statusN int) (status string) {
	switch statusN {
	case 0:
		return "offline"
	case 1:
		return "online"
	case 2:
		return "busy"
	case 3:
		return "away"
	case 4:
		return "snooze"
	case 5:
		return "looking to trade"
	case 6:
		return "looking to play"
	}
	return "N/A"
}

func (m *Steam) getSteamID(username string) (steamID string, err error) {
	encodedUsername, err := helpers.UrlEncode(username)
	if err != nil {
		return "", err
	}
	endpoint := "http://api.steampowered.com/ISteamUser/ResolveVanityURL/v0001/?key=" + m.apiKey + "&vanityurl=" + encodedUsername

	data, err := helpers.NetGetUAWithError(endpoint, helpers.DEFAULT_UA)
	if err != nil {
		return "", err
	}

	var resolveResponse SteamResolveResponse

	err = json.Unmarshal(data, &resolveResponse)
	if err != nil {
		return "", err
	}

	if resolveResponse.Response.Success == 1 {
		return resolveResponse.Response.Steamid, nil
	}
	return username, nil
}

type SteamResolveResponse struct {
	Response struct {
		Steamid string `json:"steamid"`
		Success int    `json:"success"`
		Message string `json:"message"`
	} `json:"response"`
}

type SteamPlayerSummaryResponse struct {
	Response struct {
		Players []SteamPlayerResponse `json:"players"`
	} `json:"response"`
}

type SteamPlayerResponse struct {
	Steamid                  string `json:"steamid"`
	Communityvisibilitystate int    `json:"communityvisibilitystate"`
	Profilestate             int    `json:"profilestate"`
	Personaname              string `json:"personaname"`
	Lastlogoff               int64  `json:"lastlogoff"`
	Commentpermission        int    `json:"commentpermission"`
	Profileurl               string `json:"profileurl"`
	Avatar                   string `json:"avatar"`
	Avatarmedium             string `json:"avatarmedium"`
	Avatarfull               string `json:"avatarfull"`
	Personastate             int    `json:"personastate"`
	Realname                 string `json:"realname"`
	Primaryclanid            string `json:"primaryclanid"`
	Timecreated              int64  `json:"timecreated"`
	Personastateflags        int    `json:"personastateflags"`
	Loccountrycode           string `json:"loccountrycode"`
	Locstatecode             string `json:"locstatecode"`
	Loccityid                int    `json:"loccityid"`
	Gameextrainfo            string `json:"gameextrainfo"`
	Gameid                   string `json:"gameid"`
}

func (m *Steam) getSteamProfileSummary(steamID string) (steamPlayer SteamPlayerResponse, err error) {
	endpoint := "http://api.steampowered.com/ISteamUser/GetPlayerSummaries/v0002/?key=" + m.apiKey + "&steamids=" + steamID

	data, err := helpers.NetGetUAWithError(endpoint, helpers.DEFAULT_UA)
	if err != nil {
		return steamPlayer, err
	}

	var playerSummaryResponse SteamPlayerSummaryResponse

	err = json.Unmarshal(data, &playerSummaryResponse)
	if err != nil {
		return steamPlayer, err
	}

	if playerSummaryResponse.Response.Players != nil && len(playerSummaryResponse.Response.Players) >= 1 {
		return playerSummaryResponse.Response.Players[0], nil
	}

	return steamPlayer, errors.New("steam profile not found")
}

func (m *Steam) actionFinish(args []string, in *discordgo.Message, out **discordgo.MessageSend) steamAction {
	_, err := helpers.SendComplex(in.ChannelID, *out)
	helpers.Relax(err)

	return nil
}

func (m *Steam) newMsg(content string) *discordgo.MessageSend {
	return &discordgo.MessageSend{Content: helpers.GetText(content)}
}

func (m *Steam) Relax(err error) {
	if err != nil {
		panic(err)
	}
}

func (m *Steam) logger() *logrus.Entry {
	return cache.GetLogger().WithField("module", "steam")
}
