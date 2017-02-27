package triggers

import "git.lukas.moe/sn0w/Karen/helpers"

/**
 * Full credit to Der-Eddy and his original python implementation for Shinobu-Chan.
 * @link https://github.com/Der-Eddy/discord_bot
 */
type Nep struct{}

func (n *Nep) Triggers() []string {
    return []string{
        "nep",
        "nepgear",
        "neptune",
    }
}

func (n *Nep) Response(trigger string, content string) string {
    return helpers.GetText("triggers.nep.text") + "\n" + helpers.GetText("triggers.nep.link")
}
