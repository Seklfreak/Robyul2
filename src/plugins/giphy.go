package plugins

import (
    "github.com/bwmarrin/discordgo"
    "strings"
    "fmt"
    "net/http"
    "errors"
    "strconv"
    "io"
    "github.com/Jeffail/gabs"
    "math/rand"
    "bytes"
)

const ENDPOINT = "http://api.giphy.com/v1/gifs/search"
const API_KEY = "dc6zaTOxFJmzC"
const RATING = "pg-13"
const LIMIT = 5

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
        "gif" : "<search>",
    }
}

func (g Giphy) Action(command string, msg *discordgo.Message, session *discordgo.Session) {
    session.ChannelTyping(msg.ChannelID)

    // URL encoder
    query := strings.Replace(
        strings.Replace(msg.Content, command, "", 1),
        " ",
        "+",
        -1,
    )

    // Build full url
    url := fmt.Sprintf("%s?q=%s&api_key=%s&rating=%s&limit=%s", ENDPOINT, query, API_KEY, RATING, LIMIT)

    // Send request
    response, err := http.Get(url)
    if err != nil {
        panic(err)
    }

    // Only continue if code was 200
    if response.StatusCode != 200 {
        panic(errors.New("Expected status 200; Got " + strconv.Itoa(response.StatusCode)))
    } else {
        // Read body
        defer response.Body.Close()

        buf := bytes.NewBuffer(nil)
        _, err := io.Copy(buf, response.Body)
        if err != nil {
            panic(err)
        }

        // Parse json
        json, err := gabs.ParseJSON(buf.Bytes())
        if err != nil {
            panic(err)
        }

        // Get gifs
        gifs, err := json.Path("data").Children()
        if err != nil {
            panic(err)
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
}

func (g Giphy) New() Plugin {
    return &Giphy{}
}