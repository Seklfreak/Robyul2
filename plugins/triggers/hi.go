package triggers

import "github.com/Seklfreak/Robyul2/helpers"

/**
 * Full credit to Der-Eddy and his original python implementation for Shinobu-Chan.
 * @link https://github.com/Der-Eddy/discord_bot
 */
type Hi struct{}

func (h *Hi) Triggers() []string {
	return []string{
		"wave",
		"hi",
		"hello",
		"ohai",
		"ohayou",
	}
}

func (h *Hi) Response(trigger string, content string) string {
	return ":wave: " + helpers.GetText("triggers.hi.link")
}
