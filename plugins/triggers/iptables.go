package triggers

import "git.lukas.moe/sn0w/Karen/helpers"

/**
 * Full credit to Der-Eddy and his original python implementation for Shinobu-Chan.
 * @link https://github.com/Der-Eddy/discord_bot
 */
type IPTables struct {}

func (i IPTables) Triggers() []string {
    return []string{
        "ipt",
        "iptables",
    }
}

func (i IPTables) Response() string {
    return helpers.GetText("triggers.iptables")
}
