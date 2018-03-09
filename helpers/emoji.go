package helpers

import (
	"regexp"
	"strings"

	"fmt"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
)

var (
	emojiRegex        *regexp.Regexp = regexp.MustCompile(`[\x{00A0}-\x{1F9EF}]|<(a)?:[^<>:]+:[0-9]+>`)
	discordEmojiRegex *regexp.Regexp = regexp.MustCompile(`<(a)?:([^<>:]+):([0-9]+)>`)
	unicodeEmojiRegex *regexp.Regexp = regexp.MustCompile(`[\x{00A0}-\x{1F9EF}]`)
)

// returns true if text is an unicode emoji or a discord custom emoji, returns false for everything else
func IsEmoji(text string) (isEmoji bool) {
	if emojiRegex.MatchString(text) {
		return true
	}
	return false
}

// returns true if text is an unicode emoji, returns false for everything else
func IsUnicodeEmoji(text string) (isEmoji bool) {
	if unicodeEmojiRegex.MatchString(text) {
		return true
	}
	return false
}

// returns true if text is a discord custom emoji, returns false for everything else
func IsDiscordEmoji(text string) (isEmoji bool) {
	if discordEmojiRegex.MatchString(text) {
		return true
	}
	return false
}

// gathers the emoji ID and the animation status from a custom emoji posted on discord
// text	: the custom emoji string, example: <a:anayoungSCREAM:394044148438794240>
func ParseCustomEmoji(text string) (emojiID, emojiName string, animated bool) {
	submatches := discordEmojiRegex.FindStringSubmatch(text)
	if len(submatches) >= 4 {
		var animated bool
		if submatches[1] == "a" {
			animated = true
		}
		return submatches[3], submatches[2], animated
	}
	return "", "", false
}

func GetDiscordEmojiFromText(guildID string, text string) (emoji *discordgo.Emoji, err error) {
	text = strings.Replace(text, "<", "", -1)
	text = strings.Replace(text, ">", "", -1)
	textParts := strings.Split(text, ":")
	if len(textParts) < 2 {
		return nil, errors.New("invalid emoji text received")
	}
	fmt.Println(textParts)
	return cache.GetSession().State.Emoji(guildID, textParts[len(textParts)-1])
}

func GetDiscordEmojiFromName(guildID string, name string) (emoji *discordgo.Emoji, err error) {
	guild, err := cache.GetSession().State.Guild(guildID)
	if err != nil {
		return nil, err
	}
	for _, emoji := range guild.Emojis {
		if strings.ToLower(emoji.Name) == strings.ToLower(name) {
			return emoji, nil
		}
	}
	return nil, errors.New("no emoji with the given name found")
}
