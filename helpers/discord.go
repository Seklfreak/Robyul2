package helpers

import (
	"errors"
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/bwmarrin/discordgo"
	"math/big"
	"regexp"
	"strings"
	"time"
)

var botAdmins = []string{
	"116620585638821891", // Sekl
}

// IsBotAdmin checks if $id is in $botAdmins
func IsBotAdmin(id string) bool {
	for _, s := range botAdmins {
		if s == id {
			return true
		}
	}

	return false
}

func IsAdmin(msg *discordgo.Message) bool {
	channel, e := cache.GetSession().Channel(msg.ChannelID)
	if e != nil {
		return false
	}

	guild, e := cache.GetSession().Guild(channel.GuildID)
	if e != nil {
		return false
	}

	if msg.Author.ID == guild.OwnerID || IsBotAdmin(msg.Author.ID) {
		return true
	}

	guildMember, e := cache.GetSession().GuildMember(guild.ID, msg.Author.ID)
	// Check if role may manage server
	for _, role := range guild.Roles {
		for _, userRole := range guildMember.Roles {
			if userRole == role.ID && role.Permissions&8 == 8 {
				return true
			}
		}
	}

	return false
}

// RequireAdmin only calls $cb if the author is an admin or has MANAGE_SERVER permission
func RequireAdmin(msg *discordgo.Message, cb Callback) {
	if !IsAdmin(msg) {
		cache.GetSession().ChannelMessageSend(msg.ChannelID, GetText("admin.no_permission"))
		return
	}

	cb()
}

func ConfirmEmbed(channelID string, author *discordgo.User, confirmMessageText string, confirmEmojiID string, abortEmojiID string) bool {
	// send embed asking the user to confirm
	confirmMessage, err := cache.GetSession().ChannelMessageSendEmbed(channelID, &discordgo.MessageEmbed{
		Title:       GetTextF("bot.embeds.please-confirm-title", author.Username),
		Description: confirmMessageText,
	})
	if err != nil {
		cache.GetSession().ChannelMessageSend(channelID, GetTextF("bot.errors.general", err.Error()))
	}

	// delete embed after everything is done
	defer cache.GetSession().ChannelMessageDelete(confirmMessage.ChannelID, confirmMessage.ID)

	// add default reactions to embed
	cache.GetSession().MessageReactionAdd(confirmMessage.ChannelID, confirmMessage.ID, confirmEmojiID)
	cache.GetSession().MessageReactionAdd(confirmMessage.ChannelID, confirmMessage.ID, abortEmojiID)

	// check every second if a reaction has been clicked
	for {
		confirmes, _ := cache.GetSession().MessageReactions(confirmMessage.ChannelID, confirmMessage.ID, confirmEmojiID, 100)
		for _, confirm := range confirmes {
			if confirm.ID == author.ID {
				// user has confirmed the call
				return true
			}
		}
		aborts, _ := cache.GetSession().MessageReactions(confirmMessage.ChannelID, confirmMessage.ID, abortEmojiID, 100)
		for _, abort := range aborts {
			if abort.ID == author.ID {
				// User has aborted the call
				return false
			}
		}

		time.Sleep(1 * time.Second)
	}
}

func GetMuteRole(guildID string) (*discordgo.Role, error) {
	guild, err := cache.GetSession().Guild(guildID)
	Relax(err)
	var muteRole *discordgo.Role
	settings, err := GuildSettingsGet(guildID)
	for _, role := range guild.Roles {
		Relax(err)
		if role.Name == settings.MutedRoleName {
			muteRole = role
		}
	}
	if muteRole == nil {
		muteRole, err = cache.GetSession().GuildRoleCreate(guildID)
		if err != nil {
			return muteRole, err
		}
		muteRole, err = cache.GetSession().GuildRoleEdit(guildID, muteRole.ID, settings.MutedRoleName, muteRole.Color, muteRole.Hoist, 0, muteRole.Mentionable)
		if err != nil {
			return muteRole, err
		}
		for _, channel := range guild.Channels {
			err = cache.GetSession().ChannelPermissionSet(channel.ID, muteRole.ID, "role", 0, discordgo.PermissionSendMessages)
			if err != nil {
				return muteRole, err
			}
		}
	}
	return muteRole, nil
}

func GetChannelFromMention(mention string) (*discordgo.Channel, error) {
	var targetChannel *discordgo.Channel
	re := regexp.MustCompile("(<#)?(\\d+)(>)?")
	result := re.FindStringSubmatch(mention)
	if len(result) == 4 {
		targetChannel, err := cache.GetSession().Channel(result[2])
		return targetChannel, err
	} else {
		return targetChannel, errors.New("Channel not found.")
	}
}

func GetUserFromMention(mention string) (*discordgo.User, error) {
	var targetUser *discordgo.User
	re := regexp.MustCompile("(<@)?(\\d+)(>)?")
	result := re.FindStringSubmatch(mention)
	if len(result) == 4 {
		targetUser, err := cache.GetSession().User(result[2])
		return targetUser, err
	} else {
		return targetUser, errors.New("User not found.")
	}
}

func GetDiscordColorFromHex(hex string) int {
	colorInt, ok := new(big.Int).SetString(strings.Replace(hex, "#", "", 1), 16)
	if ok == true {
		return int(colorInt.Int64())
	} else {
		return 0x0FADED
	}
}
