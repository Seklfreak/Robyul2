package triggers

import "git.lukas.moe/sn0w/Karen/helpers"

/**
 * Full credit to Der-Eddy and his original python implementation for Shinobu-Chan.
 * @link https://github.com/Der-Eddy/discord_bot
 */
type CSS struct{}

func (c *CSS) Triggers() []string {
    return []string{
        "css",
        "cs:s",
    }
}

func (c *CSS) Response(trigger string, content string) string {
    return helpers.GetText("triggers.css")
}
