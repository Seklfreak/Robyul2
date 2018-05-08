package helpers

import "regexp"

var (
	// UserRegexStrict matches Discord User Mentions
	// Source: https://github.com/b1naryth1ef/disco/blob/master/disco/bot/command.py#L15
	UserRegexStrict = regexp.MustCompile(`<@!?(\d+)>`)

	// RoleRegexStrict matches Discord Role Mentions
	// Source: https://github.com/b1naryth1ef/disco/blob/master/disco/bot/command.py#L16
	RoleRegexStrict = regexp.MustCompile(`<#(\d+)>`)

	// ChannelRegexStrict matches Discord Channel Mentions
	// Source: https://github.com/b1naryth1ef/disco/blob/master/disco/bot/command.py#L17
	ChannelRegexStrict = regexp.MustCompile(`<#(\d+)>`)

	// MentionRegexStrict matches Discord Emoji
	// Source: Discord API Server => ?tag discordregex
	MentionRegexStrict = regexp.MustCompile(`<a?:(\w+):(\d+)>`)

	// URLRege matches a URL on Discord
	URLRegex = regexp.MustCompile(`((?:https?|steam):\/\/[^\s<]+[^<.,:;"'\]\s])`)
)
