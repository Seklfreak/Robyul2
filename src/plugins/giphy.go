package plugins

import (
    "github.com/bwmarrin/discordgo"
    "fmt"
    "math/rand"
    "net/url"
    "../utils"
)

type Giphy struct{}

func (g Giphy) Name() string {
    return "Giphy"
}

func (g Giphy) Description() string {
    return "Gets a random gif"
}

func (g Giphy) Commands() map[string]string {
    return map[string]string{
        "giphy" : "<search>",
        "gif" : "Alias for giphy",
    }
}

func (g Giphy) Init(session *discordgo.Session) {

}

func (g Giphy) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
    const ENDPOINT = "http://api.giphy.com/v1/gifs/search"
    const API_KEY = "dc6zaTOxFJmzC"
    const RATING = "pg-13"
    const LIMIT = 5

    session.ChannelTyping(msg.ChannelID)

    // Send request
    json := utils.GetJSON(
        fmt.Sprintf(
            "%s?q=%s&api_key=%s&rating=%s&limit=%s",
            ENDPOINT,
            url.QueryEscape(content),
            API_KEY,
            RATING,
            LIMIT,
        ),
    )

    // Get gifs
    gifs, err := json.Path("data").Children()
    if err != nil {
        session.ChannelMessageSend(msg.ChannelID, "Error parsing Giphy's response :frowning:")
        return
    }

    // Chose a random one
    m := ""
    if len(gifs) > 0 {
        m = gifs[rand.Intn(len(gifs))].Path("bitly_url").Data().(string)
    } else {
        m = "No gifs found :frowning:"
    }

    // Send the result
    session.ChannelMessageSend(msg.ChannelID, m)
}