package plugins

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
)

type Osu struct{}

func (o *Osu) Commands() []string {
	return []string{
		"osu",
		"osu!mania",
		"osu!k",
		"osu!ctb",
		"osu!taiko",
	}
}

func (o *Osu) Init(session *discordgo.Session) {

}

func (o *Osu) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	session.ChannelTyping(msg.ChannelID)

	user := strings.TrimSpace(content)

	var mode string
	switch command {
	case "osu":
		mode = "0"
		break

	case "osu!taiko":
		mode = "1"
		break

	case "osu!ctb":
		mode = "2"
		break

	case "osu!mania", "osu!k":
		mode = "3"
		break
	}

	jsonc, err := helpers.GetJSON(
		fmt.Sprintf(
			"https://osu.ppy.sh/api/get_user?k=%s&u=%s&type=u&m=%s",
			helpers.GetConfig().Path("osu").Data().(string),
			user,
			mode,
		),
	).Children()
	helpers.Relax(err)

	if len(jsonc) == 0 {
		helpers.SendMessage(msg.ChannelID, "User not found <a:ablobfrown:394026913292615701>")
		return
	}

	json := jsonc[0]
	html := string(helpers.NetGet("https://osu.ppy.sh/u/" + user))
	avatar := regexp.MustCompile(
		`"//a\.ppy\.sh/` + json.Path("user_id").Data().(string) + `_\d+\.\w{2,5}"`,
	).FindString(html)

	if avatar == "" {
		avatar = "http://i.imgur.com/Ea1qmJX.png"
	} else {
		avatar = "https:" + avatar
	}

	avatar = strings.Replace(avatar, `"`, "", -1)

	if (!json.ExistsP("level")) || json.Path("level").Data() == nil {
		helpers.SendMessage(msg.ChannelID, "Seems like "+user+" didn't play this mode yet <:blobthinking:317028940885524490>")
		return
	}

	helpers.SendEmbed(msg.ChannelID, &discordgo.MessageEmbed{
		Color:       0xEF77AF,
		Description: "Showing stats for " + user,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: avatar,
		},
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Name", Value: json.Path("username").Data().(string), Inline: true},
			{Name: "Country", Value: json.Path("country").Data().(string), Inline: true},
			{Name: "Level", Value: json.Path("level").Data().(string), Inline: true},
			{Name: "Playcount", Value: json.Path("playcount").Data().(string), Inline: true},
			{Name: "Accuracy", Value: json.Path("accuracy").Data().(string) + "%", Inline: true},
			{Name: "Rank (Country)", Value: json.Path("pp_country_rank").Data().(string) + "th", Inline: true},
			{Name: "Rank (Global)", Value: json.Path("pp_rank").Data().(string) + "th", Inline: true},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "ppy powered :3",
		},
	})
}
