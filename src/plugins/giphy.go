package plugins

import (
    "github.com/bwmarrin/discordgo"
    "strings"
    "fmt"
    "net/http"
    "../utils"
    "errors"
    "strconv"
    "io"
    "os"
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
    query := strings.Replace(
        strings.Replace(msg.Content, command, "", 1),
        " ",
        "+",
        -1,
    )

    url := fmt.Sprintf("%s?q=%s&api_key=%s&rating=%s&limit=%s", ENDPOINT, query, API_KEY, RATING, LIMIT)
    response, err := http.Get(url)

    fmt.Println(url)
    fmt.Println(response)

    if err != nil {
        utils.SendError(session, msg.ChannelID, err)
    } else {
        if response.StatusCode != 200 {
            utils.SendError(session, msg.ChannelID, errors.New("Expected status 200; Got " + strconv.Itoa(response.StatusCode)))
        } else {
            defer response.Body.Close()
            _, err := io.Copy(os.Stdout, response.Body)
            if err != nil {
                utils.SendError(session, msg.ChannelID, err)
            }
        }
    }
}

func (g Giphy) New() Plugin {
    return &Giphy{}
}