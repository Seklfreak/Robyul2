package triggers

import "github.com/Seklfreak/Robyul2/helpers"

/**
 * Full credit to Der-Eddy and his original python implementation for Shinobu-Chan.
 * @link https://github.com/Der-Eddy/discord_bot
 */
type HypeTrain struct{}

func (h *HypeTrain) Triggers() []string {
	return []string{
		"hype",
		"hypu",
	}
}

func (h *HypeTrain) Response(trigger string, content string) string {
	return helpers.GetText("triggers.hypetrain.text") + "\n" + helpers.GetText("triggers.hypetrain.link")
}
