package plugins

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/bwmarrin/discordgo"
	humanize "github.com/dustin/go-humanize"
	"github.com/globalsign/mgo/bson"
	fb "github.com/huandu/facebook"
	"github.com/pkg/errors"
)

type Facebook struct{}

type Facebook_Page struct {
	ID                string
	Name              string
	About             string
	Likes             int64
	Username          string
	Verified          bool
	ProfilePictureUrl string
	Website           string
	Posts             []Facebook_Post
}

type Facebook_Post struct {
	ID         string
	Message    string
	CreatedAt  string
	PictureUrl string
	Url        string
}

type Facebook_Safe_Entries struct {
	entries []models.FacebookEntry
	mux     sync.Mutex
}

const (
	facebookHexColor     = "#3b5998"
	FacebookFriendlyPage = "https://facebook.com/%s/"
)

func (m *Facebook) Commands() []string {
	return []string{
		"facebook",
	}
}

func (m *Facebook) Init(session *discordgo.Session) {
	go m.checkFacebookFeedsLoop()
	cache.GetLogger().WithField("module", "facebook").Info("Started Facebook loop (10m)")
}
func (m *Facebook) checkFacebookFeedsLoop() {
	log := cache.GetLogger()

	defer helpers.Recover()
	defer func() {
		go func() {
			log.WithField("module", "facebook").Error("The checkFacebookFeedsLoop died. Please investigate! Will be restarted in 60 seconds")
			time.Sleep(60 * time.Second)
			m.checkFacebookFeedsLoop()
		}()
	}()

	var entries []models.FacebookEntry
	var bundledEntries map[string][]models.FacebookEntry

	for {
		err := helpers.MDbIter(helpers.MdbCollection(models.FacebookTable).Find(nil)).All(&entries)
		helpers.Relax(err)

		bundledEntries = make(map[string][]models.FacebookEntry, 0)

		for _, entry := range entries {
			channel, err := helpers.GetChannelWithoutApi(entry.ChannelID)
			if err != nil || channel == nil || channel.ID == "" {
				//cache.GetLogger().WithField("module", "facebook").Warn(fmt.Sprintf("skipped facebook @%s for Channel #%s on Guild #%s: channel not found!",
				//	entry.Username, entry.ChannelID, entry.ServerID))
				continue
			}

			if _, ok := bundledEntries[entry.Username]; ok {
				bundledEntries[entry.Username] = append(bundledEntries[entry.Username], entry)
			} else {
				bundledEntries[entry.Username] = []models.FacebookEntry{entry}
			}
		}

		cache.GetLogger().WithField("module", "facebook").Infof("checking %d pages for %d feeds", len(bundledEntries), len(entries))

		for facebookUsername, entries := range bundledEntries {
			//log.WithField("module", "facebook").Debug(fmt.Sprintf("checking Facebook Page %s", facebookUsername))

			facebookPage, err := m.lookupFacebookPage(facebookUsername)
			if err != nil {
				if strings.Contains(err.Error(), "Application request limit reached") {
					log.WithField("module", "facebook").Infoln("facebook api limit reached, retrying in one minute")
					time.Sleep(time.Minute)
					continue
				}
				log.WithField("module", "facebook").Warnf("updating facebook account %s failed: %s", facebookUsername, err.Error())
				continue
			}

			// https://github.com/golang/go/wiki/SliceTricks#reversing
			for i := len(facebookPage.Posts)/2 - 1; i >= 0; i-- {
				opp := len(facebookPage.Posts) - 1 - i
				facebookPage.Posts[i], facebookPage.Posts[opp] = facebookPage.Posts[opp], facebookPage.Posts[i]
			}

			for _, entry := range entries {
				changes := false

				for _, post := range facebookPage.Posts {
					postAlreadyPosted := false
					for _, postedPost := range entry.PostedPosts {
						if postedPost.ID == post.ID {
							postAlreadyPosted = true
						}
					}
					if postAlreadyPosted == false {
						log.WithField("module", "facebook").Info(fmt.Sprintf("Posting Post: #%s", post.ID))
						entry.PostedPosts = append(entry.PostedPosts, models.FacebookPostEntry{ID: post.ID, CreatedAt: post.CreatedAt})
						changes = true
						go m.postPostToChannel(entry.ChannelID, post, facebookPage)
					}

				}
				if changes == true {
					err = helpers.MDbUpsertID(
						models.FacebookTable,
						entry.ID,
						entry,
					)
					helpers.Relax(err)
				}
			}
			time.Sleep(10 * time.Second)
		}

		if len(entries) <= 10 {
			time.Sleep(1 * time.Minute)
		}
	}
}

func (m *Facebook) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if !helpers.ModuleIsAllowed(msg.ChannelID, msg.ID, msg.Author.ID, helpers.ModulePermFacebook) {
		return
	}

	args := strings.Fields(content)
	if len(args) >= 1 {
		switch args[0] {
		case "add": // [p]facebook add <facebook page name> <discord channel>
			helpers.RequireMod(msg, func() {
				session.ChannelTyping(msg.ChannelID)
				// get target channel
				var err error
				var targetChannel *discordgo.Channel
				var targetGuild *discordgo.Guild
				if len(args) >= 3 {
					targetChannel, err = helpers.GetChannelFromMention(msg, args[2])
					if err != nil {
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.invalid"))
						return
					}
				} else {
					helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
					return
				}
				targetGuild, err = helpers.GetGuild(targetChannel.GuildID)
				helpers.Relax(err)
				// get facebook account and tweets
				facebookPage, err := m.lookupFacebookPage(args[1])
				if err != nil {
					if e, ok := err.(*fb.Error); ok {
						if e.Code == 803 || e.Code == 100 || strings.Contains(err.Error(), "Unknown path components") {
							helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.facebook.page-not-found"))
							return
						}
					}
					helpers.Relax(err)
				}
				// Create DB Entries
				var dbPosts []models.FacebookPostEntry
				for _, facebookPost := range facebookPage.Posts {
					facebookPostEntry := models.FacebookPostEntry{ID: facebookPost.ID, CreatedAt: facebookPost.CreatedAt}
					dbPosts = append(dbPosts, facebookPostEntry)

				}
				// create new entry in db
				_, err = helpers.MDbInsert(
					models.FacebookTable,
					models.FacebookEntry{
						GuildID:     targetChannel.GuildID,
						ChannelID:   targetChannel.ID,
						Username:    facebookPage.Username,
						PostedPosts: dbPosts,
					},
				)
				helpers.Relax(err)

				_, err = helpers.EventlogLog(time.Now(), targetChannel.GuildID, targetChannel.GuildID,
					models.EventlogTargetTypeRobyulFacebookFeed, msg.Author.ID,
					models.EventlogTypeRobyulFacebookFeedAdd, "",
					nil,
					[]models.ElasticEventlogOption{
						{
							Key:   "facebook_channelid",
							Value: targetChannel.ID,
							Type:  models.EventlogTargetTypeChannel,
						},
						{
							Key:   "facebook_facebookusername",
							Value: facebookPage.Username,
						},
					}, false)
				helpers.RelaxLog(err)

				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.facebook.account-added-success", facebookPage.Username, targetChannel.ID))
				cache.GetLogger().WithField("module", "facebook").Info(fmt.Sprintf("Added Facebook Account %s to Channel %s (#%s) on Guild %s (#%s)", facebookPage.Username, targetChannel.Name, targetChannel.ID, targetGuild.Name, targetGuild.ID))
			})
		case "delete", "del", "remove": // [p]facebook delete <id>
			helpers.RequireMod(msg, func() {
				if len(args) >= 2 {
					session.ChannelTyping(msg.ChannelID)

					channel, err := helpers.GetChannel(msg.ChannelID)
					helpers.Relax(err)

					entryId := args[1]
					var entryBucket models.FacebookEntry
					err = helpers.MdbOne(
						helpers.MdbCollection(models.FacebookTable).Find(bson.M{"_id": helpers.HumanToMdbId(entryId), "guildid": channel.GuildID}),
						&entryBucket,
					)
					if err == nil {
						err = helpers.MDbDelete(models.FacebookTable, entryBucket.ID)

						_, err := helpers.EventlogLog(time.Now(), channel.GuildID, helpers.MdbIdToHuman(entryBucket.ID),
							models.EventlogTargetTypeRobyulFacebookFeed, msg.Author.ID,
							models.EventlogTypeRobyulFacebookFeedRemove, "",
							nil,
							[]models.ElasticEventlogOption{
								{
									Key:   "facebook_channelid",
									Value: entryBucket.ChannelID,
								},
								{
									Key:   "facebook_facebookusername",
									Value: entryBucket.Username,
								},
							}, false)
						helpers.RelaxLog(err)

						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.facebook.account-delete-success", entryBucket.Username))
						cache.GetLogger().WithField("module", "facebook").Info(fmt.Sprintf("Deleted Facebook Page `%s`", entryBucket.Username))
					} else {
						helpers.SendMessage(msg.ChannelID, helpers.GetText("plugins.facebook.account-delete-not-found-error"))
						return
					}
				} else {
					helpers.SendMessage(msg.ChannelID, helpers.GetText("bot.arguments.too-few"))
					return
				}
			})
		case "list": // [p]facebook list
			currentChannel, err := helpers.GetChannel(msg.ChannelID)
			helpers.Relax(err)
			var entryBucket []models.FacebookEntry
			err = helpers.MDbIter(helpers.MdbCollection(models.FacebookTable).Find(bson.M{"guildid": currentChannel.GuildID})).All(&entryBucket)
			helpers.Relax(err)

			if len(entryBucket) <= 0 {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.facebook.account-list-no-accounts-error"))
				return
			} else if err != nil {
				helpers.Relax(err)
			}

			resultMessage := ""
			for _, entry := range entryBucket {
				resultMessage += fmt.Sprintf("`%s`: Facebook Page `%s` posting to <#%s>\n", helpers.MdbIdToHuman(entry.ID), entry.Username, entry.ChannelID)
			}
			resultMessage += fmt.Sprintf("Found **%d** Facebook Pages in total.", len(entryBucket))
			for _, resultPage := range helpers.Pagify(resultMessage, "\n") {
				_, err = helpers.SendMessage(msg.ChannelID, resultPage)
				helpers.Relax(err)
			}
		default:
			session.ChannelTyping(msg.ChannelID)

			if args[0] == "" {
				helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.facebook.page-not-found"))
				return
			}

			facebookPage, err := m.lookupFacebookPage(args[0])
			if err != nil {
				if e, ok := err.(*fb.Error); ok {
					if e.Code == 803 || e.Code == 100 {
						helpers.SendMessage(msg.ChannelID, helpers.GetTextF("plugins.facebook.page-not-found"))
						return
					}
				}
				helpers.Relax(err)
			}

			facebookNameModifier := ""
			if facebookPage.Verified {
				facebookNameModifier += " ☑"
			}

			accountEmbed := &discordgo.MessageEmbed{
				Title:     helpers.GetTextF("plugins.facebook.page-embed-title", facebookPage.Name, facebookPage.Username, facebookNameModifier),
				URL:       fmt.Sprintf(FacebookFriendlyPage, facebookPage.Username),
				Thumbnail: &discordgo.MessageEmbedThumbnail{URL: facebookPage.ProfilePictureUrl},
				Footer: &discordgo.MessageEmbedFooter{
					Text:    helpers.GetText("plugins.facebook.embed-footer"),
					IconURL: helpers.GetText("plugins.facebook.embed-footer-imageurl"),
				},
				Description: facebookPage.About,
				Fields: []*discordgo.MessageEmbedField{
					{Name: "Likes", Value: humanize.Comma(facebookPage.Likes), Inline: true}},
				Color: helpers.GetDiscordColorFromHex(facebookHexColor),
			}
			if facebookPage.Website != "" {
				accountEmbed.Fields = append(accountEmbed.Fields, &discordgo.MessageEmbedField{
					Name:   "Website",
					Value:  facebookPage.Website,
					Inline: true,
				})
			}
			_, err = helpers.SendComplex(
				msg.ChannelID, &discordgo.MessageSend{
					Content: fmt.Sprintf("<%s>", fmt.Sprintf(FacebookFriendlyPage, facebookPage.Username)),
					Embed:   accountEmbed,
				})
			helpers.Relax(err)
			return
		}
	} else {
		helpers.SendMessage(msg.ChannelID, helpers.GetTextF("bot.arguments.too-few"))
	}
}

func (m *Facebook) lookupFacebookPage(siteName string) (Facebook_Page, error) {
	var facebookPage Facebook_Page
	facebookPageResult, err := fb.Get(fmt.Sprintf("/%s", siteName), fb.Params{
		"fields":       "id,name,about,fan_count,username,is_verified,picture,website",
		"access_token": helpers.GetConfig().Path("facebook.access_token").Data().(string),
	})
	if err != nil {
		return facebookPage, err
	}

	if _, ok := facebookPageResult["id"]; ok && facebookPageResult["id"] != "" {
		facebookPage.ID = facebookPageResult["id"].(string)
	} else {
		return facebookPage, errors.New("Unable to find facebook page ID")
	}
	if _, ok := facebookPageResult["name"]; ok && facebookPageResult["name"] != "" {
		facebookPage.Name = facebookPageResult["name"].(string)
	} else {
		return facebookPage, errors.New("Unable to find facebook page Name")
	}
	if _, ok := facebookPageResult["username"]; ok && facebookPageResult["username"] != "" {
		facebookPage.Username = facebookPageResult["username"].(string)
	} else {
		facebookPage.Username = facebookPage.ID
	}
	if _, ok := facebookPageResult["about"]; ok && facebookPageResult["about"] != "" {
		facebookPage.About = facebookPageResult["about"].(string)
	}
	if _, ok := facebookPageResult["website"]; ok && facebookPageResult["website"] != "" {
		facebookPage.Website = facebookPageResult["website"].(string)
	}

	err = facebookPageResult.DecodeField("picture.data.url", &facebookPage.ProfilePictureUrl)
	if err != nil {
		return facebookPage, err
	}
	err = facebookPageResult.DecodeField("fan_count", &facebookPage.Likes)
	if err != nil {
		return facebookPage, err
	}
	err = facebookPageResult.DecodeField("is_verified", &facebookPage.Verified)
	if err != nil {
		return facebookPage, err
	}

	var facebookPosts []Facebook_Post
	facebookPostsResultData, err := fb.Get(fmt.Sprintf("/%s/posts", siteName), fb.Params{
		"fields":       "id,message,created_time,picture,permalink_url", // TODO: ,child_attachments (better image quality(?))
		"limit":        10,
		"access_token": helpers.GetConfig().Path("facebook.access_token").Data().(string),
	})
	if err != nil {
		return facebookPage, err
	}
	var facebookPostsResult []fb.Result
	err = facebookPostsResultData.DecodeField("data", &facebookPostsResult)
	if err != nil {
		return facebookPage, err
	}

	for _, facebookPostResult := range facebookPostsResult {
		if _, ok := facebookPostResult["permalink_url"]; !ok {
			continue
		}

		var facebookPost Facebook_Post
		facebookPost.ID = facebookPostResult["id"].(string)
		if _, ok := facebookPostResult["message"]; ok {
			facebookPost.Message = facebookPostResult["message"].(string)
		}
		facebookPost.CreatedAt = facebookPostResult["created_time"].(string)
		facebookPost.Url = facebookPostResult["permalink_url"].(string)
		facebookPost.PictureUrl = ""
		if _, ok := facebookPostResult["picture"]; ok {
			facebookPost.PictureUrl = facebookPostResult["picture"].(string)
		}
		facebookPosts = append(facebookPosts, facebookPost)
	}
	facebookPage.Posts = facebookPosts

	return facebookPage, nil
}

func (m *Facebook) postPostToChannel(channelID string, post Facebook_Post, facebookPage Facebook_Page) {
	facebookNameModifier := ""
	if facebookPage.Verified {
		facebookNameModifier += " ☑"
	}

	channelEmbed := &discordgo.MessageEmbed{
		Title:     helpers.GetTextF("plugins.facebook.post-embed-title", facebookPage.Name, facebookPage.Username, facebookNameModifier),
		URL:       post.Url,
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: facebookPage.ProfilePictureUrl},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    helpers.GetText("plugins.facebook.embed-footer"),
			IconURL: helpers.GetText("plugins.facebook.embed-footer-imageurl"),
		},
		Description: post.Message,
		Color:       helpers.GetDiscordColorFromHex(facebookHexColor),
	}

	if post.PictureUrl != "" {
		channelEmbed.Image = &discordgo.MessageEmbedImage{URL: post.PictureUrl}
	}

	_, err := helpers.SendComplex(channelID, &discordgo.MessageSend{
		Content: fmt.Sprintf("<%s>", post.Url),
		Embed:   channelEmbed,
	})
	if err != nil {
		cache.GetLogger().WithField("module", "facebook").Warnf("posting post: #%s to channel: #%s failed: %s", post.ID, channelID, err)
	}
}
