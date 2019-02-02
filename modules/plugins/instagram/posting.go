package instagram

import (
	"fmt"
	"math"
	"strconv"

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

	mediaModifier := "Picture"
	if post.IsVideo {
		mediaModifier = "Video"
	}
	if len(post.MediaUrls) >= 2 {
		mediaModifier = fmt.Sprintf("Album (%d items)", len(post.MediaUrls))
	}

	content := []string{""}
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
		content[0] += "**" + helpers.GetTextF("plugins.instagram.post-embed-title", post.Author.FullName, post.Author.Username, instagramNameModifier, mediaModifier) + "** _" + helpers.GetText("plugins.instagram.embed-footer") + "_\n"
		if post.Caption != "" {
			content[0] += post.Caption + "\n"
		}
	}

	channelEmbed.Image = &discordgo.MessageEmbedImage{URL: post.MediaUrls[0]}

	mediaUrls := make([]string, 0)
	for _, mediaUrl := range post.MediaUrls {
		mediaUrls = append(mediaUrls, mediaUrl)
	}

	content[0] += fmt.Sprintf("<%s>", fmt.Sprintf(instagramFriendlyPost, post.Shortcode))

	if len(mediaUrls) > 0 {
		channelEmbed.Description += "\n\n`Links:` "
		for i, mediaUrl := range mediaUrls {
			if postType == models.InstagramSendPostTypeDirectLinks {
				index := int(math.Floor((float64(i+1) / 5.0) - 0.01))

				if index >= len(content) {
					content = append(content, "")
				}

				fmt.Println("adding " + mediaUrl + " to content at " + strconv.Itoa(index))
				content[index] += "\n" + mediaUrl
			}
			channelEmbed.Description += fmt.Sprintf("[%s](%s) ", emojis.From(strconv.Itoa(i+1)), mediaUrl)
		}
	}

	messageSend := &discordgo.MessageSend{
		Content: content[0],
	}
	if postType != models.InstagramSendPostTypeDirectLinks {
		messageSend.Embed = channelEmbed
	}

	_, err := helpers.SendComplex(channelID, messageSend)
	if err != nil {
		return
	}

	if len(content) > 1 {
		for _, text := range content[1:] {
			_, err = helpers.SendMessage(channelID, text)
			if err != nil {
				return
			}
		}
	}
}

/*
func (m *Handler) postLiveToChannel(channelID string, instagramUser Instagram_User) {
	instagramNameModifier := ""
	if instagramUser.IsVerified {
		instagramNameModifier += " ☑"
	}
	if instagramUser.IsPrivate {
		instagramNameModifier += " 🔒"
	}
	if instagramUser.IsBusiness {
		instagramNameModifier += " 🏢"
	}
	if instagramUser.IsFavorite {
		instagramNameModifier += " ⭐"
	}

	channelEmbed := &discordgo.MessageEmbed{
		Title:     helpers.GetTextF("plugins.instagram.live-embed-title", instagramUser.FullName, instagramUser.Username, instagramNameModifier),
		URL:       fmt.Sprintf(instagramFriendlyUser, instagramUser.Username),
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: instagramUser.ProfilePic.URL},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.instagram.embed-footer"),
			IconURL: helpers.GetText("plugins.instagram.embed-footer-imageurl"),
		},
		Image: &discordgo.MessageEmbedImage{URL: instagramUser.Broadcast.CoverFrameURL},
		Color: helpers.GetDiscordColorFromHex(hexColor),
	}

	mediaUrl := channelEmbed.URL

	_, err := helpers.SendComplex(channelID, &discordgo.MessageSend{
		Content: fmt.Sprintf("<%s>", mediaUrl),
		Embed:   channelEmbed,
	})
	if err != nil {
		cache.GetLogger().WithField("module", "instagram").Warnf("posting broadcast: #%d to channel: #%s failed: %s", instagramUser.Broadcast.ID, channelID, err.Error())
	}
}
*/

/*
func (m *Handler) postReelMediaToChannel(channelID string, story goinstaResponse.StoryResponse, number int, postMode models.InstagramSendPostType) {
	instagramNameModifier := ""
	if story.Reel.User.IsVerified {
		instagramNameModifier += " ☑"
	}
	if story.Reel.User.IsPrivate {
		instagramNameModifier += " 🔒"
	}
	/*
		if story.Reel.User.IsBusiness {
			instagramNameModifier += " 🏢"
		}
		if story.Reel.User.IsFavorite {
			instagramNameModifier += " ⭐"
		}
*/ /*

	reelMedia := story.Reel.Items[number]

	mediaModifier := "Picture"
	if reelMedia.MediaType == 2 {
		mediaModifier = "Video"
	}

	caption := ""
	if captionData, ok := reelMedia.Caption.(map[string]interface{}); ok {
		caption, _ = captionData["text"].(string)
	}

	var content string
	channelEmbed := &discordgo.MessageEmbed{
		Title:     helpers.GetTextF("plugins.instagram.reelmedia-embed-title", story.Reel.User.FullName, story.Reel.User.Username, instagramNameModifier, mediaModifier),
		URL:       fmt.Sprintf(instagramFriendlyUser, story.Reel.User.Username),
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: story.Reel.User.ProfilePicURL},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.instagram.embed-footer"),
			IconURL: helpers.GetText("plugins.instagram.embed-footer-imageurl"),
		},
		Description: caption,
		Color:       helpers.GetDiscordColorFromHex(hexColor),
	}
	if postMode == models.InstagramSendPostTypeDirectLinks {
		content += "**" + helpers.GetTextF("plugins.instagram.reelmedia-embed-title", story.Reel.User.FullName, story.Reel.User.Username, instagramNameModifier, mediaModifier) + "** _" + helpers.GetText("plugins.instagram.embed-footer") + "_\n"
		if caption != "" {
			content += caption + "\n"
		}
	}

	mediaUrl := ""
	thumbnailUrl := ""

	if len(reelMedia.ImageVersions2.Candidates) > 0 {
		channelEmbed.Image = &discordgo.MessageEmbedImage{URL: getBestCandidateURL(reelMedia.ImageVersions2.Candidates)}
		mediaUrl = getBestCandidateURL(reelMedia.ImageVersions2.Candidates)
	}
	if len(reelMedia.VideoVersions) > 0 {
		channelEmbed.Video = &discordgo.MessageEmbedVideo{
			URL: getBestStoryVideoVersionURL(story, number),
		}
		if mediaUrl != "" {
			thumbnailUrl = mediaUrl
		}
		mediaUrl = getBestStoryVideoVersionURL(story, number)
	}

	if mediaUrl != "" {
		channelEmbed.URL = mediaUrl
	} else {
		mediaUrl = channelEmbed.URL
	}

	content += stripInstagramDirectLink(mediaUrl) + "\n"
	if thumbnailUrl != "" {
		content += stripInstagramDirectLink(thumbnailUrl) + "\n"
	}

	messageSend := &discordgo.MessageSend{
		Content: content,
	}
	if postMode != models.InstagramSendPostTypeDirectLinks {
		messageSend.Content = fmt.Sprintf("<%s>", stripInstagramDirectLink(mediaUrl))
		messageSend.Embed = channelEmbed
	}

	_, err := helpers.SendComplex(channelID, messageSend)
	if err != nil {
		cache.GetLogger().WithField("module", "instagram").Warnf("posting reel media: #%s to channel: #%s failed: %s", reelMedia.ID, channelID, err.Error())
	}
}
*/
