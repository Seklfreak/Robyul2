package helpers

import (
	"github.com/Seklfreak/Robyul2/cache"
	"github.com/bwmarrin/discordgo"
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
