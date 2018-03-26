package instagram

import (
	"fmt"
	"strconv"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/emojis"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
)

func (m *Handler) postPostToChannel(channelID string, post InstagramPostInformation, postType models.InstagramSendPostType) {
	instagramNameModifier := ""
	if post.Author.IsVerified {
		instagramNameModifier += " ☑"
	}
	if post.Author.IsPrivate {
		instagramNameModifier += " 🔒"
	}
	/*
		if post.User.IsBusiness {
			instagramNameModifier += " 🏢"
		}
	*/
	/*
		if post.User.IsFavorite {
			instagramNameModifier += " ⭐"
		}
	*/

	mediaModifier := "Picture"
	if post.IsVideo {
		mediaModifier = "Video"
	}
	if len(post.MediaUrls) >= 2 {
		mediaModifier = fmt.Sprintf("Album (%d items)", len(post.MediaUrls))
	}

	var content string
	channelEmbed := &discordgo.MessageEmbed{
		Title:     helpers.GetTextF("plugins.instagram.post-embed-title", post.Author.FullName, post.Author.Username, instagramNameModifier, mediaModifier),
		URL:       fmt.Sprintf(instagramFriendlyPost, post.Shortcode),
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: post.Author.ProfilePicUrl},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.instagram.embed-footer"),
			IconURL: helpers.GetText("plugins.instagram.embed-footer-imageurl"),
		},
		Description: post.Caption,
		Color:       helpers.GetDiscordColorFromHex(hexColor),
	}
	if postType == models.InstagramSendPostTypeDirectLinks {
		content += "**" + helpers.GetTextF("plugins.instagram.post-embed-title", post.Author.FullName, post.Author.Username, instagramNameModifier, mediaModifier) + "** _" + helpers.GetText("plugins.instagram.embed-footer") + "_\n"
		if post.Caption != "" {
			content += post.Caption + "\n"
		}
	}

	channelEmbed.Image = &discordgo.MessageEmbedImage{URL: post.MediaUrls[0]}

	mediaUrls := make([]string, 0)
	for _, mediaUrl := range post.MediaUrls {
		mediaUrls = append(mediaUrls, mediaUrl)
	}

	content += fmt.Sprintf("<%s>", fmt.Sprintf(instagramFriendlyPost, post.Shortcode))

	if len(mediaUrls) > 0 {
		channelEmbed.Description += "\n\n`Links:` "
		for i, mediaUrl := range mediaUrls {
			if postType == models.InstagramSendPostTypeDirectLinks {
				content += "\n" + mediaUrl
			}
			channelEmbed.Description += fmt.Sprintf("[%s](%s) ", emojis.From(strconv.Itoa(i+1)), mediaUrl)
		}
	}

	messageSend := &discordgo.MessageSend{
		Content: content,
	}
	if postType != models.InstagramSendPostTypeDirectLinks {
		messageSend.Embed = channelEmbed
	}

	_, err := helpers.SendComplex(channelID, messageSend)
	if err != nil {
		cache.GetLogger().WithField("module", "instagram").Warnf("posting post: #%s to channel: #%s failed: %s", post.ID, channelID, err)
	}
}
